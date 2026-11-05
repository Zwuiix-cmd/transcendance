package leaderboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
)

type Route struct {
	route.Route
}

type dependencies interface {
	Database() *sql.DB
	AvatarURL(string, *string) *string
	CacheGetJSON(context.Context, string, any) bool
	CacheSetJSON(context.Context, string, any, time.Duration)
}

func Handle(context *route.Context) {
	server, ok := context.Server.(dependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}

	limit, offset, page := pagination(context.Request, 100, 250)
	cacheKey := "leaderboard:" + strconv.Itoa(limit) + ":" + strconv.Itoa(offset)
	var cached struct {
		Entries []model.LeaderboardEntry `json:"entries"`
		HasMore bool                     `json:"hasMore"`
	}
	if server.CacheGetJSON(context.Request.Context(), cacheKey, &cached) {
		setPaginationHeaders(context.Writer, page, limit, cached.HasMore)
		writeJSON(context.Writer, http.StatusOK, cached.Entries)
		return
	}

	rows, err := server.Database().Query(`SELECT u.id,u.username,u.avatar_url,u.elo,s.games_played,s.games_won FROM users u JOIN user_stats s ON s.user_id=u.id WHERE u.deleted_at IS NULL ORDER BY u.elo DESC,s.games_won DESC,u.username ASC LIMIT ? OFFSET ?`, limit+1, offset)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	defer rows.Close()
	entries := []model.LeaderboardEntry{}
	rank := offset + 1
	for rows.Next() {
		var entry model.LeaderboardEntry
		var avatar sql.NullString
		if err := rows.Scan(&entry.UserID, &entry.Username, &avatar, &entry.Elo, &entry.GamesPlayed, &entry.GamesWon); err != nil {
			writeDatabaseError(context.Writer, err)
			return
		}
		entry.Rank = rank
		rank++
		if entry.GamesPlayed > 0 {
			entry.WinRate = float64(entry.GamesWon) / float64(entry.GamesPlayed)
		}
		entry.AvatarURL = nullableString(avatar)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	for index := range entries {
		entries[index].AvatarURL = server.AvatarURL(entries[index].UserID, entries[index].AvatarURL)
	}
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	server.CacheSetJSON(context.Request.Context(), cacheKey, struct {
		Entries []model.LeaderboardEntry `json:"entries"`
		HasMore bool                     `json:"hasMore"`
	}{Entries: entries, HasMore: hasMore}, 5*time.Second)
	setPaginationHeaders(context.Writer, page, limit, hasMore)
	writeJSON(context.Writer, http.StatusOK, entries)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1/leaderboard", route.AccessPublic),
}

func pagination(request *http.Request, defaultLimit, maxLimit int) (int, int, int) {
	limit := queryInt(request, "limit", defaultLimit)
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	page := queryInt(request, "page", 1)
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	return limit, (page - 1) * limit, page
}

func queryInt(request *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(request.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func nullableString(value sql.NullString) *string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	copy := value.String
	return &copy
}

func setPaginationHeaders(writer http.ResponseWriter, page, limit int, hasMore bool) {
	writer.Header().Set("X-Pagination-Page", strconv.Itoa(page))
	writer.Header().Set("X-Pagination-Limit", strconv.Itoa(limit))
	writer.Header().Set("X-Pagination-Has-More", strconv.FormatBool(hasMore))
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
