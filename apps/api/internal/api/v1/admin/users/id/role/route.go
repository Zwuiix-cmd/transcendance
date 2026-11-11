package role

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	route "card-arena/api/internal/api/route"
)

type Route struct {
	route.Route
}

type dependencies interface {
	Database() *sql.DB
	AdminActor(any) (string, string, bool)
	RecordAudit(string, string, string, string, string, string, string, string, string)
	ClientKey(*http.Request) string
}

var rolePattern = regexp.MustCompile(`^(player|staff|developer|admin)$`)

func Handle(context *route.Context) {
	server, ok := context.Server.(dependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	adminID, adminUsername, ok := server.AdminActor(context.User)
	if !ok {
		writeError(context.Writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return
	}
	var input struct {
		Role string `json:"role"`
	}
	if !decodeJSON(context.Writer, context.Request, &input) {
		return
	}
	role := strings.ToLower(strings.TrimSpace(input.Role))
	if !rolePattern.MatchString(role) {
		writeError(context.Writer, http.StatusBadRequest, "invalid_role", "Role must be player, staff, developer or admin")
		return
	}
	id := context.Param("id")
	if id == adminID {
		writeError(context.Writer, http.StatusBadRequest, "self_role_change", "You cannot change your own role")
		return
	}
	result, err := server.Database().Exec(`UPDATE users SET role=? WHERE id=? AND deleted_at IS NULL`, role, id)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(context.Writer, http.StatusNotFound, "user_not_found", "User was not found")
		return
	}
	server.RecordAudit(adminID, adminUsername, "admin.user.role_changed", "user", id, "Role set to "+role, "warning", server.ClientKey(context.Request), context.Request.UserAgent())
	if notifier, ok := context.Server.(interface{ PublishProfile(string) }); ok {
		notifier.PublishProfile(id)
	}
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPut, "/api/v1/admin/users/{id}/role", route.AccessAdmin),
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return false
	}
	return true
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
