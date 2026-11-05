package me

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
)

type UpdateRoute struct {
	route.Route
}

type profileDependencies interface {
	Database() *sql.DB
	ProfileWithAvatar(string, bool) (model.User, error)
	UserID(any) (string, bool)
}

func HandleUpdate(context *route.Context) {
	server, ok := context.Server.(profileDependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	userID, ok := server.UserID(context.User)
	if !ok {
		writeError(context.Writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return
	}
	var input struct {
		Bio string `json:"bio"`
	}
	if !decodeJSON(context.Writer, context.Request, &input) {
		return
	}
	if len(input.Bio) > 180 {
		writeError(context.Writer, http.StatusBadRequest, "invalid_bio", "Bio must contain at most 180 characters")
		return
	}
	if _, err := server.Database().Exec(`UPDATE users SET bio=? WHERE id=? AND deleted_at IS NULL`, strings.TrimSpace(input.Bio), userID); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	profile, err := server.ProfileWithAvatar(userID, true)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	writeJSON(context.Writer, http.StatusOK, profile)
}

func (UpdateRoute) Handler() route.Handler {
	return HandleUpdate
}

var UpdateDefinition = UpdateRoute{
	Route: route.NewRoute(http.MethodPut, "/api/v1/users/me", route.AccessUser),
}

type DeleteRoute struct {
	route.Route
}

func HandleDelete(context *route.Context) {
	server, ok := context.Server.(profileDependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	userID, ok := server.UserID(context.User)
	if !ok {
		writeError(context.Writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return
	}
	result, err := server.Database().Exec(`DELETE FROM users WHERE id=? AND deleted_at IS NULL`, userID)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(context.Writer, http.StatusNotFound, "user_not_found", "User was not found")
		return
	}
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (DeleteRoute) Handler() route.Handler {
	return HandleDelete
}

var DeleteDefinition = DeleteRoute{
	Route: route.NewRoute(http.MethodDelete, "/api/v1/users/me", route.AccessUser),
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
