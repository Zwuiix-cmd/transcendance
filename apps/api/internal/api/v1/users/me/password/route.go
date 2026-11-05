package password

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	route "card-arena/api/internal/api/route"
	"golang.org/x/crypto/bcrypt"
)

type Route struct {
	route.Route
}

type passwordDependencies interface {
	Database() *sql.DB
	UserID(any) (string, bool)
}

func Handle(context *route.Context) {
	server, ok := context.Server.(passwordDependencies)
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
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(context.Writer, context.Request, &input) {
		return
	}
	if len(input.NewPassword) < 12 || len(input.NewPassword) > 128 {
		writeError(context.Writer, http.StatusBadRequest, "invalid_password", "Password must contain between 12 and 128 characters")
		return
	}
	var oldHash string
	if err := server.Database().QueryRow(`SELECT password_hash FROM users WHERE id=? AND deleted_at IS NULL`, userID).Scan(&oldHash); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(input.CurrentPassword)) != nil {
		writeError(context.Writer, http.StatusBadRequest, "invalid_current_password", "Current password is incorrect")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(input.NewPassword)) == nil {
		writeError(context.Writer, http.StatusBadRequest, "same_password", "New password must be different")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 12)
	if err != nil {
		writeError(context.Writer, http.StatusInternalServerError, "password_error", "Could not secure the password")
		return
	}
	if _, err = server.Database().Exec(`UPDATE users SET password_hash=? WHERE id=?`, string(hash), userID); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	_, _ = server.Database().Exec(`UPDATE auth_sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, time.Now().UTC().Format(time.RFC3339Nano), userID)
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPut, "/api/v1/users/me/password", route.AccessUser),
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
