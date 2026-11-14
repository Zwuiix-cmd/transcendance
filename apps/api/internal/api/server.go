package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	playeranalytics "card-arena/api/internal/analytics"
	"card-arena/api/internal/cache"
	"card-arena/api/internal/config"
	"card-arena/api/internal/db"
	"card-arena/api/internal/matchmaking"
	"card-arena/api/internal/model"
	"card-arena/api/internal/realtime"
	unogame "card-arena/api/internal/uno"
	"github.com/google/uuid"
)

type Server struct {
	cfg             config.Config
	store           *db.Store
	cache           *cache.Cache
	queue           *matchmaking.Queue
	client          *http.Client
	limiter         *rateLimiter
	audit           *auditWriter
	uno             *unogame.Engine
	router          *Router
	hub             *realtime.Hub
	ctx             context.Context
	cancel          context.CancelFunc
	maintenanceStop chan struct{}
	maintenanceDone chan struct{}
}

type authUser struct {
	ID       string
	Username string
	Role     string
	Elo      int
}

type authContextKey struct{}

type cachedAuthUser struct {
	user *authUser
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (writer *responseRecorder) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *responseRecorder) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	return writer.ResponseWriter.Write(body)
}

func (writer *responseRecorder) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

type rateLimiter struct {
	mu      sync.Mutex
	windows map[string]limitWindow
}
type limitWindow struct {
	started time.Time
	count   int
}

func newRateLimiter() *rateLimiter { return &rateLimiter{windows: map[string]limitWindow{}} }

func (limiter *rateLimiter) allow(key string, limit int) (bool, int) {
	if limit < 1 {
		limit = 1
	}
	now := time.Now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	window := limiter.windows[key]
	if window.started.IsZero() || now.Sub(window.started) >= time.Minute {
		window = limitWindow{started: now}
	}
	window.count++
	limiter.windows[key] = window
	return window.count <= limit, int(60 - now.Sub(window.started).Seconds())
}

func NewServer(cfg config.Config, store *db.Store) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{cfg: cfg, store: store, cache: cache.New(cfg.RedisURL), client: &http.Client{Timeout: 10 * time.Second}, limiter: newRateLimiter(), uno: unogame.NewEngine(store.DB), hub: realtime.NewHub(), ctx: ctx, cancel: cancel, maintenanceStop: make(chan struct{}), maintenanceDone: make(chan struct{})}
	server.queue = matchmaking.New(server.publishMatchmaking)
	server.uno.OnChange(server.publishUnoUpdate)
	server.audit = newAuditWriter(store)
	server.router = NewRouter(server)
	server.queue.Start(time.Second)
	go server.runMaintenance()
	return server
}

func (server *Server) UnoGame() *unogame.Engine { return server.uno }

// Hub exposes the realtime hub to tests and diagnostics.
func (server *Server) Hub() *realtime.Hub { return server.hub }

func (server *Server) Close() error {
	server.uno.Stop()
	server.queue.Stop()
	close(server.maintenanceStop)
	<-server.maintenanceDone
	server.hub.Close()
	server.cancel()
	if server.audit != nil {
		server.audit.close()
	}
	return server.cache.Close()
}

// Handler exposes health probes and the versioned REST API.
func (server *Server) Handler() http.Handler { return http.HandlerFunc(server.serveHTTP) }

func (server *Server) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	server.applyCORS(writer, request)
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if request.URL.Path == "/healthz" || request.URL.Path == "/actuator/health/liveness" {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "UP"})
		return
	}
	if request.URL.Path == "/readyz" || request.URL.Path == "/actuator/health/readiness" {
		server.readiness(writer, request)
		return
	}
	if !strings.HasPrefix(request.URL.Path, "/api/v1") {
		writeError(writer, http.StatusNotFound, "not_found", "Route was not found")
		return
	}

	clientKey := server.clientKey(request)
	if !server.validAPIKey(request.Header.Get("X-API-Key")) {
		if server.cfg.Environment != "development" {

			allowed, retry := server.allowRate(request, "invalid:"+clientKey, server.cfg.InvalidRequestsPerMinute)
			if !allowed {
				writer.Header().Set("Retry-After", strconv.Itoa(retry))
				writeError(writer, http.StatusTooManyRequests, "rate_limited", "Too many invalid API requests")
				return
			}
		}
		writeError(writer, http.StatusUnauthorized, "invalid_api_key", "A valid API key is required")
		server.recordAudit("", "", "security.invalid_api_key", "security", request.URL.Path, request.Method+" "+request.URL.Path+" -> 401", "warning", clientKey, request.UserAgent())
		return
	}

	limit := server.cfg.ValidRequestsPerMinute
	if strings.HasPrefix(request.URL.Path, "/api/v1/admin/") {
		limit = server.cfg.AdminRequestsPerMinute
	}
	allowed, retry := server.allowRate(request, "valid:"+clientKey, limit)

	if server.cfg.Environment != "development" {
		if !allowed {
			writer.Header().Set("Retry-After", strconv.Itoa(retry))
			writeError(writer, http.StatusTooManyRequests, "rate_limited", "Too many requests")
			return
		}
	}

	request, actor := server.cacheAuthentication(request)
	recorder := &responseRecorder{ResponseWriter: writer}
	server.router.ServeHTTP(recorder, request)
	status := recorder.status
	if status == 0 {
		status = http.StatusOK
	}
	actorID, actorUsername := "", ""
	if actor != nil {
		actorID, actorUsername = actor.ID, actor.Username
	}
	severity := "info"
	if status >= 500 {
		severity = "critical"
	} else if status >= 400 {
		severity = "warning"
	}
	server.recordAudit(actorID, actorUsername, "http.request", "api", request.URL.Path, request.Method+" "+request.URL.Path+" -> "+strconv.Itoa(status), severity, clientKey, request.UserAgent())
}

func (server *Server) applyCORS(writer http.ResponseWriter, request *http.Request) {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return
	}
	for _, allowed := range server.cfg.CORSAllowedOrigins {
		if allowed == "*" || allowed == origin {
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Credentials", "true")
			writer.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key, X-Game-Service-Key")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			writer.Header().Add("Vary", "Origin")
			return
		}
	}
}

func (server *Server) validAPIKey(value string) bool {
	if server.cfg.APIKey == "" {
		return true
	}
	if len(value) != len(server.cfg.APIKey) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(server.cfg.APIKey)) == 1
}

func (server *Server) clientKey(request *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	if request.RemoteAddr != "" {
		return request.RemoteAddr
	}
	return "unknown"
}

func (server *Server) authenticated(request *http.Request) *authUser {
	if cached, ok := request.Context().Value(authContextKey{}).(cachedAuthUser); ok {
		return cached.user
	}
	return server.lookupAuthenticated(request)
}

func (server *Server) cacheAuthentication(request *http.Request) (*http.Request, *authUser) {
	header := strings.TrimSpace(request.Header.Get("Authorization"))
	if header == "" {
		return request, nil
	}
	user := server.lookupAuthenticated(request)
	return request.WithContext(context.WithValue(request.Context(), authContextKey{}, cachedAuthUser{user: user})), user
}

func (server *Server) lookupAuthenticated(request *http.Request) *authUser {
	header := strings.TrimSpace(request.Header.Get("Authorization"))
	if server.cfg.DebugAuthEnabled && header == "Bearer debug" {
		return server.findAuthUserByUsername(server.cfg.DebugAuthUsername)
	}
	if !strings.HasPrefix(header, "Bearer ") {
		return nil
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return nil
	}
	hash := tokenHash(token)
	var user authUser
	err := server.store.DB.QueryRow(`
		SELECT u.id, u.username, u.role, u.elo
		FROM auth_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND u.deleted_at IS NULL`, hash, nowString()).Scan(&user.ID, &user.Username, &user.Role, &user.Elo)
	if err != nil {
		return nil
	}
	return &user
}

func (server *Server) requireUser(writer http.ResponseWriter, request *http.Request) *authUser {
	user := server.authenticated(request)
	if user == nil {
		writeError(writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return nil
	}
	return user
}

func (server *Server) requireAdmin(writer http.ResponseWriter, request *http.Request) *authUser {
	user := server.requireUser(writer, request)
	if user == nil {
		return nil
	}
	if user.Role != "admin" {
		writeError(writer, http.StatusForbidden, "admin_required", "Admin access is required")
		return nil
	}
	return user
}

func (server *Server) requireGameService(writer http.ResponseWriter, request *http.Request) bool {
	expected := strings.TrimSpace(server.cfg.GameServiceKey)
	provided := strings.TrimSpace(request.Header.Get("X-Game-Service-Key"))
	if expected == "" {
		writeError(writer, http.StatusServiceUnavailable, "game_service_not_configured", "The game result service is not configured")
		return false
	}
	if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeError(writer, http.StatusUnauthorized, "invalid_game_service_key", "A valid game service key is required")
		return false
	}
	return true
}

func (server *Server) findAuthUserByUsername(username string) *authUser {
	var user authUser
	err := server.store.DB.QueryRow(`SELECT id, username, role, elo FROM users WHERE username = ? COLLATE NOCASE AND deleted_at IS NULL`, username).Scan(&user.ID, &user.Username, &user.Role, &user.Elo)
	if err != nil {
		return nil
	}
	return &user
}

func (server *Server) issueSession(user model.User, request *http.Request) (model.AuthResponse, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return model.AuthResponse{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	created := time.Now().UTC()
	expires := created.Add(server.cfg.SessionTTL)
	_, err := server.store.DB.Exec(`INSERT INTO auth_sessions (id,user_id,token_hash,created_at,expires_at,user_agent,ip_address) VALUES (?,?,?,?,?,?,?)`, uuid.NewString(), user.ID, tokenHash(token), created.Format(time.RFC3339Nano), expires.Format(time.RFC3339Nano), request.UserAgent(), server.clientKey(request))
	if err != nil {
		return model.AuthResponse{}, err
	}
	return model.AuthResponse{AccessToken: token, ExpiresAt: expires.Format(time.RFC3339Nano), User: user}, nil
}

func tokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func (server *Server) profile(id string, includeEmail bool) (model.User, error) {
	var user model.User
	var email, avatar, lastLogin sql.NullString
	var stats model.Stats
	err := server.store.DB.QueryRow(`SELECT u.id,u.username,u.bio,u.email,u.avatar_url,u.elo,u.role,u.created_at,u.last_login,COALESCE(s.games_played,0),COALESCE(s.games_won,0),COALESCE(s.current_streak,0),COALESCE(s.max_streak,0),COALESCE(s.unos_called,0),COALESCE(s.uno_penalties,0),COALESCE(s.draw_fours_played,0),COALESCE(s.cards_drawn,0) FROM users u LEFT JOIN user_stats s ON s.user_id=u.id WHERE u.id=? AND u.deleted_at IS NULL`, id).Scan(&user.ID, &user.Username, &user.Bio, &email, &avatar, &user.Elo, &user.Role, &user.CreatedAt, &lastLogin, &stats.GamesPlayed, &stats.GamesWon, &stats.CurrentStreak, &stats.MaxStreak, &stats.UnosCalled, &stats.UnoPenalties, &stats.DrawFoursPlayed, &stats.CardsDrawn)
	if err != nil {
		return model.User{}, err
	}
	user.Email = nullString(email)
	if !includeEmail {
		user.Email = nil
	}
	user.AvatarURL = nullString(avatar)
	user.LastLogin = nullString(lastLogin)
	user.Stats = stats
	return user, nil
}

func (server *Server) profileWithAvatar(id string, includeEmail bool) (model.User, error) {
	user, err := server.profile(id, includeEmail)
	if err != nil {
		return user, err
	}
	user.AvatarURL = server.avatarURL(id, user.AvatarURL)
	return user, nil
}

func (server *Server) avatarURL(id string, current *string) *string {
	if current != nil && strings.TrimSpace(*current) != "" {
		return current
	}
	value := strings.TrimRight(server.cfg.CDNPublicURL, "/") + "/avatars/" + url.PathEscape(id) + "/avatar"
	return &value
}

func nowString() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func nullString(value sql.NullString) *string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	valueCopy := value.String
	return &valueCopy
}

func queryInt(request *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(request.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{"code": code, "message": message, "timestamp": nowString()})
}

func (server *Server) recordAudit(actorID, actorUsername, action, targetType, targetID, details, severity, ipAddress, userAgent string) {
	if len(details) > 1000 {
		details = details[:1000]
	}
	if len(action) > 80 {
		action = action[:80]
	}
	if severity != "warning" && severity != "critical" {
		severity = "info"
	}
	if server.audit != nil {
		server.audit.enqueue(auditEvent{actorID: actorID, actorUsername: actorUsername, action: action, targetType: targetType, targetID: targetID, details: details, severity: severity, ipAddress: ipAddress, userAgent: userAgent})
	}
}
func nilIfBlank(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (server *Server) analytics(id string) (model.UserAnalytics, error) {
	profile, err := server.profile(id, false)
	if err != nil {
		return model.UserAnalytics{}, err
	}
	rows, err := server.store.DB.Query(`SELECT id,game_id,previous_elo,new_elo,delta,reason,created_at FROM user_elo_history WHERE user_id=? ORDER BY created_at ASC,id ASC`, id)
	if err != nil {
		return model.UserAnalytics{}, err
	}
	history := []model.EloHistoryPoint{}
	for rows.Next() {
		var point model.EloHistoryPoint
		var gameID sql.NullString
		if err := rows.Scan(&point.ID, &gameID, &point.PreviousElo, &point.NewElo, &point.Delta, &point.Reason, &point.CreatedAt); err != nil {
			return model.UserAnalytics{}, err
		}
		point.GameID = nullString(gameID)
		history = append(history, point)
	}
	if err := rows.Err(); err != nil {
		return model.UserAnalytics{}, err
	}
	_ = rows.Close()
	activityRows, err := server.store.DB.Query(`SELECT substr(COALESCE(g.ended_at,g.created_at),1,10),COUNT(*),SUM(CASE WHEN p.position=1 THEN 1 ELSE 0 END),SUM(CASE WHEN p.position IS NOT NULL AND p.position<>1 THEN 1 ELSE 0 END),COALESCE(SUM(p.score),0),COALESCE(SUM(COALESCE(NULLIF(p.elo_delta,0),(SELECT SUM(h.delta) FROM user_elo_history h WHERE h.user_id=p.user_id AND h.game_id=p.game_id),0)),0) FROM game_participants p JOIN games g ON g.id=p.game_id WHERE p.user_id=? AND g.status='finished' GROUP BY substr(COALESCE(g.ended_at,g.created_at),1,10) ORDER BY 1 ASC LIMIT 365`, id)
	if err != nil {
		return model.UserAnalytics{}, err
	}
	activity := []model.UserActivityPoint{}
	for activityRows.Next() {
		var point model.UserActivityPoint
		if err := activityRows.Scan(&point.Date, &point.Games, &point.Wins, &point.Losses, &point.Score, &point.EloDelta); err != nil {
			return model.UserAnalytics{}, err
		}
		activity = append(activity, point)
	}
	if err := activityRows.Err(); err != nil {
		return model.UserAnalytics{}, err
	}
	_ = activityRows.Close()
	gameRows, err := server.store.DB.Query(`SELECT g.id,g.mode,g.status,p.position,p.score,COALESCE(NULLIF(p.elo_delta,0),(SELECT SUM(h.delta) FROM user_elo_history h WHERE h.user_id=p.user_id AND h.game_id=g.id),0),COALESCE(g.ended_at,g.created_at) FROM game_participants p JOIN games g ON g.id=p.game_id WHERE p.user_id=? AND g.status='finished' ORDER BY COALESCE(g.ended_at,g.created_at) DESC LIMIT 50`, id)
	if err != nil {
		return model.UserAnalytics{}, err
	}
	recent := []model.UserGamePoint{}
	for gameRows.Next() {
		var point model.UserGamePoint
		var position sql.NullInt64
		if err := gameRows.Scan(&point.GameID, &point.Mode, &point.Status, &position, &point.Score, &point.EloDelta, &point.PlayedAt); err != nil {
			return model.UserAnalytics{}, err
		}
		if position.Valid {
			value := int(position.Int64)
			point.Position = &value
		}
		recent = append(recent, point)
	}
	if err := gameRows.Err(); err != nil {
		return model.UserAnalytics{}, err
	}
	_ = gameRows.Close()
	var rank, total int
	if err := server.store.DB.QueryRow(`SELECT (SELECT COUNT(*)+1 FROM users higher WHERE higher.deleted_at IS NULL AND higher.elo>target.elo),(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL) FROM users target WHERE target.id=? AND target.deleted_at IS NULL`, id).Scan(&rank, &total); err != nil {
		return model.UserAnalytics{}, err
	}
	average := float64(profile.Elo)
	peak := profile.Elo
	lowest := profile.Elo
	net := 0
	positive := 0
	negative := 0
	if len(history) > 0 {
		sum := 0
		peak = history[0].NewElo
		lowest = history[0].NewElo
		for _, point := range history {
			sum += point.NewElo
			if point.NewElo > peak {
				peak = point.NewElo
			}
			if point.NewElo < lowest {
				lowest = point.NewElo
			}
			net += point.Delta
			if point.Delta > 0 {
				positive++
			}
			if point.Delta < 0 {
				negative++
			}
		}
		average = float64(sum) / float64(len(history))
	}
	averageChange := 0.0
	if len(history) > 0 {
		averageChange = float64(net) / float64(len(history))
	}
	variance := 0.0
	if len(history) > 0 {
		for _, point := range history {
			variance += math.Pow(float64(point.Delta)-averageChange, 2)
		}
		variance /= float64(len(history))
	}
	percentile := 0.0
	if total > 0 {
		percentile = float64(total-rank+1) / float64(total) * 100
	}
	score, totalEloDelta := 0, 0
	for _, point := range activity {
		score += point.Score
		totalEloDelta += point.EloDelta
	}
	winRate := 0.0
	if profile.Stats.GamesPlayed > 0 {
		winRate = float64(profile.Stats.GamesWon) / float64(profile.Stats.GamesPlayed)
	}
	summary := model.UserAnalyticsSummary{CurrentElo: profile.Elo, AverageElo: average, PeakElo: peak, LowestElo: lowest, NetEloDelta: net, Volatility: math.Sqrt(variance), AverageEloDelta: averageChange, PositiveEloDeltas: positive, NegativeEloDeltas: negative, EloEvents: len(history), Rank: rank, TotalPlayers: total, Percentile: percentile, GamesPlayed: profile.Stats.GamesPlayed, GamesWon: profile.Stats.GamesWon, WinRate: winRate, CurrentStreak: profile.Stats.CurrentStreak, MaxStreak: profile.Stats.MaxStreak, UnosCalled: profile.Stats.UnosCalled, UnoPenalties: profile.Stats.UnoPenalties, DrawFoursPlayed: profile.Stats.DrawFoursPlayed, CardsDrawn: profile.Stats.CardsDrawn, TotalScore: score, TotalEloDelta: totalEloDelta}
	visibleActivity := playeranalytics.RecentActivity(activity)
	return model.UserAnalytics{
		Summary:         summary,
		EloHistory:      history,
		Activity:        activity,
		RecentGames:     recent,
		ActivityMetrics: playeranalytics.SummarizeActivity(activity),
		EloChart:        playeranalytics.PrepareEloChart(history),
		ActivityChart:   playeranalytics.PreparePlayerActivityChart(visibleActivity),
	}, nil
}
