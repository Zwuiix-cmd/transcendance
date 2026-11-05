package userid

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
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
}

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
	id := context.Param("id")
	if id == adminID {
		writeError(context.Writer, http.StatusBadRequest, "self_delete", "You cannot delete your own account from admin")
		return
	}
	result, err := server.Database().Exec(`DELETE FROM users WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(context.Writer, http.StatusNotFound, "user_not_found", "User was not found")
		return
	}
	server.RecordAudit(adminID, adminUsername, "admin.user.deleted", "user", id, "Account and linked data deleted", "critical", "", "admin-panel")
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodDelete, "/api/v1/admin/users/{id}", route.AccessAdmin),
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
