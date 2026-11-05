package dashboard

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	route "card-arena/api/internal/api/route"
	adminlogs "card-arena/api/internal/api/v1/admin/logs"
	"card-arena/api/internal/model"
)

type Route struct {
	route.Route
}

type dependencies interface {
	Database() *sql.DB
	ReadAuditLogs(int, adminlogs.Filter) ([]model.AuditLog, error)
}

func Handle(context *route.Context) {
	server, ok := context.Server.(dependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	days := queryInt(context.Request, "days", 14)
	if days < 7 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	logLimit := queryInt(context.Request, "logs", 12)
	if logLimit < 1 {
		logLimit = 1
	}
	if logLimit > 50 {
		logLimit = 50
	}

	overview, err := readOverview(server.Database())
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour).Format(time.RFC3339Nano)
	activityRows, err := server.Database().Query(`SELECT substr(created_at,1,10),SUM(CASE WHEN action='http.request' THEN 1 ELSE 0 END),SUM(CASE WHEN action='http.request' AND details LIKE '% /api/v1/auth/register -> %' THEN 1 ELSE 0 END),SUM(CASE WHEN action='http.request' AND details LIKE '% /api/v1/auth/login -> %' THEN 1 ELSE 0 END),SUM(CASE WHEN action='http.request' AND (details LIKE '% /api/v1/games%' OR details LIKE '% /api/v1/matchmaking%') THEN 1 ELSE 0 END),SUM(CASE WHEN action LIKE 'admin.%' THEN 1 ELSE 0 END),SUM(CASE WHEN severity IN ('warning','critical') OR action LIKE 'security.%' THEN 1 ELSE 0 END) FROM audit_logs WHERE created_at>=? GROUP BY substr(created_at,1,10) ORDER BY 1 ASC`, since)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	activity := []model.AdminActivityPoint{}
	for activityRows.Next() {
		var point model.AdminActivityPoint
		if err := activityRows.Scan(&point.Date, &point.TotalRequests, &point.Registrations, &point.Logins, &point.GameActions, &point.AdminActions, &point.SecurityEvents); err != nil {
			activityRows.Close()
			writeDatabaseError(context.Writer, err)
			return
		}
		activity = append(activity, point)
	}
	if err := activityRows.Err(); err != nil {
		activityRows.Close()
		writeDatabaseError(context.Writer, err)
		return
	}
	activityRows.Close()
	eloDistribution, err := distribution(server.Database(), `SELECT CASE WHEN elo<1000 THEN 'Below 1000' WHEN elo<1200 THEN '1000–1199' WHEN elo<1500 THEN '1200–1499' WHEN elo<1800 THEN '1500–1799' ELSE '1800+' END,COUNT(*) FROM users WHERE deleted_at IS NULL GROUP BY 1 ORDER BY MIN(elo)`)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	modeDistribution, err := distribution(server.Database(), `SELECT mode,COUNT(*) FROM games g WHERE EXISTS(SELECT 1 FROM game_participants p WHERE p.game_id=g.id) GROUP BY mode ORDER BY COUNT(*) DESC`)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	recent, err := server.ReadAuditLogs(logLimit, adminlogs.Filter{})
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	var logsToday int
	_ = server.Database().QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE created_at>=?`, time.Now().UTC().Truncate(24*time.Hour).Format(time.RFC3339Nano)).Scan(&logsToday)
	writeJSON(context.Writer, http.StatusOK, model.AdminDashboard{Overview: overview, LogsToday: logsToday, Activity: activity, EloDistribution: eloDistribution, ModeDistribution: modeDistribution, RecentLogs: recent})
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1/admin/dashboard", route.AccessAdmin),
}

func readOverview(database *sql.DB) (model.AdminOverview, error) {
	var overview model.AdminOverview
	err := database.QueryRow(`SELECT (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL),(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND last_login>=strftime('%Y-%m-%dT%H:%M:%fZ','now','-30 days')),(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND role='admin'),(SELECT COALESCE(ROUND(AVG(elo)),0) FROM users WHERE deleted_at IS NULL),(SELECT COALESCE(MAX(elo),0) FROM users WHERE deleted_at IS NULL),(SELECT COUNT(*) FROM games g WHERE EXISTS(SELECT 1 FROM game_participants p WHERE p.game_id=g.id)),(SELECT COUNT(*) FROM games WHERE status='waiting' AND EXISTS(SELECT 1 FROM game_participants p WHERE p.game_id=games.id)),(SELECT COUNT(*) FROM games WHERE status='in_progress' AND EXISTS(SELECT 1 FROM game_participants p WHERE p.game_id=games.id)),(SELECT COUNT(*) FROM games WHERE status='finished' AND EXISTS(SELECT 1 FROM game_participants p WHERE p.game_id=games.id)),(SELECT COALESCE(SUM(games_played),0) FROM user_stats)`).Scan(&overview.TotalUsers, &overview.ActiveUsers, &overview.AdminUsers, &overview.AverageElo, &overview.HighestElo, &overview.TotalGames, &overview.WaitingGames, &overview.ActiveGames, &overview.FinishedGames, &overview.RecordedGames)
	return overview, err
}

func distribution(database *sql.DB, query string) ([]model.AdminDistribution, error) {
	rows, err := database.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.AdminDistribution{}
	for rows.Next() {
		var item model.AdminDistribution
		if err := rows.Scan(&item.Label, &item.Value); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
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
	writeJSON(writer, status, map[string]any{
		"code":      code,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func writeDatabaseError(writer http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "not_found", "Resource was not found")
		return
	}
	slog.Error("database request failed", "error", err)
	writeError(writer, http.StatusInternalServerError, "database_error", "The request could not be completed")
}
