package elo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	route "card-arena/api/internal/api/route"
	"github.com/google/uuid"
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

type ratingNotifier interface {
	PublishLeaderboard()
	PublishProfile(string)
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
	var input struct {
		Elo *int `json:"elo"`
	}
	if !decodeJSON(context.Writer, context.Request, &input) {
		return
	}
	if input.Elo == nil || *input.Elo < 0 || *input.Elo > 5000 {
		writeError(context.Writer, http.StatusBadRequest, "invalid_elo", "ELO must be between 0 and 5000")
		return
	}
	id := context.Param("id")
	var previous int
	if err := server.Database().QueryRow(`SELECT elo FROM users WHERE id=? AND deleted_at IS NULL`, id).Scan(&previous); err != nil {
		writeError(context.Writer, http.StatusNotFound, "user_not_found", "User was not found")
		return
	}
	if _, err := server.Database().Exec(`UPDATE users SET elo=? WHERE id=? AND deleted_at IS NULL`, *input.Elo, id); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	if previous != *input.Elo {
		_, _ = server.Database().Exec(`INSERT INTO user_elo_history(id,user_id,previous_elo,new_elo,delta,reason,created_at) VALUES(?,?,?,?,?,?,?)`, uuid.NewString(), id, previous, *input.Elo, *input.Elo-previous, "admin_adjustment", time.Now().UTC().Format(time.RFC3339Nano))
	}
	server.RecordAudit(adminID, adminUsername, "admin.user.elo_updated", "user", id, "ELO set to "+strconv.Itoa(*input.Elo), "warning", server.ClientKey(context.Request), context.Request.UserAgent())
	if notifier, ok := context.Server.(ratingNotifier); ok && previous != *input.Elo {
		notifier.PublishLeaderboard()
		notifier.PublishProfile(id)
	}
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPut, "/api/v1/admin/users/{id}/elo", route.AccessAdmin),
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
