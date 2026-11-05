package search

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
)

type Route struct {
	route.Route
}

type searchDependencies interface {
	Database() *sql.DB
	AvatarURL(string, *string) *string
}

func Handle(context *route.Context) {
	server, ok := context.Server.(searchDependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	query := strings.TrimSpace(context.Request.URL.Query().Get("q"))
	limit, offset, page := pagination(context.Request, 12, 24)
	if len(query) < 2 || len(query) > 32 {
		writeError(context.Writer, http.StatusBadRequest, "invalid_query", "Search must contain between 2 and 32 characters")
		return
	}
	searchTerm := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
	var rows *sql.Rows
	var err error
	if utf8.RuneCountInString(query) >= 3 {
		rows, err = server.Database().Query(`SELECT u.id,u.username,u.bio,u.avatar_url,u.elo,s.games_played,s.games_won FROM users_fts f JOIN users u ON u.rowid=f.rowid JOIN user_stats s ON s.user_id=u.id WHERE users_fts MATCH ? AND u.deleted_at IS NULL ORDER BY CASE WHEN u.username = ? COLLATE NOCASE THEN 0 ELSE 1 END,u.elo DESC,u.username ASC LIMIT ? OFFSET ?`, `username : `+searchTerm, query, limit+1, offset)
	}
	if err != nil || rows == nil {
		// FTS5 is optional in SQLite builds used by some local distributions.
		// Keep the fallback indexable by using a prefix predicate rather than a
		// leading wildcard scan.
		rows, err = server.Database().Query(`SELECT u.id,u.username,u.bio,u.avatar_url,u.elo,s.games_played,s.games_won FROM users u JOIN user_stats s ON s.user_id=u.id WHERE u.deleted_at IS NULL AND u.username LIKE ? COLLATE NOCASE ORDER BY CASE WHEN u.username = ? COLLATE NOCASE THEN 0 ELSE 1 END,u.elo DESC,u.username ASC LIMIT ? OFFSET ?`, query+"%", query, limit+1, offset)
	}
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	defer rows.Close()
	results := []model.ProfileSearchResult{}
	for rows.Next() {
		var item model.ProfileSearchResult
		var avatar sql.NullString
		if err := rows.Scan(&item.ID, &item.Username, &item.Bio, &avatar, &item.Elo, &item.GamesPlayed, &item.GamesWon); err != nil {
			writeDatabaseError(context.Writer, err)
			return
		}
		item.AvatarURL = nullString(avatar)
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	for index := range results {
		results[index].AvatarURL = server.AvatarURL(results[index].ID, results[index].AvatarURL)
	}
	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	setPaginationHeaders(context.Writer, page, limit, hasMore)
	writeJSON(context.Writer, http.StatusOK, results)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1/users/search", route.AccessPublic),
}

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
	writeJSON(writer, status, map[string]any{"code": code, "message": message, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}

func writeDatabaseError(writer http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "not_found", "Resource was not found")
		return
	}
	slog.Error("database request failed", "error", err)
	writeError(writer, http.StatusInternalServerError, "database_error", "The request could not be completed")
}
