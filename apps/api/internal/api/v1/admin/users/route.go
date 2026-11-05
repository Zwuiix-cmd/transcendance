package users

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
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
}

func Handle(context *route.Context) {
	server, ok := context.Server.(dependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	limit, offset, page := pagination(context.Request, 100, 250)
	where := []string{"u.deleted_at IS NULL"}
	args := []any{}
	if query := strings.TrimSpace(context.Request.URL.Query().Get("q")); query != "" {
		pattern := "%" + query + "%"
		where = append(where, "(u.id LIKE ? OR u.username LIKE ? COLLATE NOCASE OR u.email LIKE ? COLLATE NOCASE)")
		args = append(args, pattern, pattern, pattern)
	}
	if role := strings.TrimSpace(context.Request.URL.Query().Get("role")); role != "" && role != "all" {
		if !rolePattern.MatchString(role) {
			writeError(context.Writer, http.StatusBadRequest, "invalid_role", "Unknown role")
			return
		}
		where = append(where, "u.role=?")
		args = append(args, role)
	}
	query := `SELECT u.id,u.username,u.email,u.avatar_url,u.elo,u.role,u.created_at,u.last_login,COALESCE(s.games_played,0),COALESCE(s.games_won,0) FROM users u LEFT JOIN user_stats s ON s.user_id=u.id WHERE ` + strings.Join(where, " AND ") + ` ORDER BY u.elo DESC,u.created_at ASC LIMIT ? OFFSET ?`
	args = append(args, limit+1, offset)
	rows, err := server.Database().Query(query, args...)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	defer rows.Close()
	users := []model.AdminUser{}
	for rows.Next() {
		var item model.AdminUser
		var avatar, last sql.NullString
		if err := rows.Scan(&item.ID, &item.Username, &item.Email, &avatar, &item.Elo, &item.Role, &item.CreatedAt, &last, &item.GamesPlayed, &item.GamesWon); err != nil {
			writeDatabaseError(context.Writer, err)
			return
		}
		item.AvatarURL = nullableString(avatar)
		item.LastLogin = nullableString(last)
		users = append(users, item)
	}
	if err := rows.Err(); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	for index := range users {
		users[index].AvatarURL = server.AvatarURL(users[index].ID, users[index].AvatarURL)
	}
	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}
	setPaginationHeaders(context.Writer, page, limit, hasMore)
	writeJSON(context.Writer, http.StatusOK, users)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1/admin/users", route.AccessAdmin),
}

var rolePattern = regexp.MustCompile(`^(player|staff|developer|admin)$`)

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
