package login

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type Route struct {
	route.Route
}

type loginDependencies interface {
	Database() *sql.DB
	Profile(string, bool) (model.User, error)
	IssueSession(model.User, *http.Request) (model.AuthResponse, error)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Handle(context *route.Context) {
	server, ok := context.Server.(loginDependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}

	var input loginRequest
	if !decodeJSON(context.Writer, context.Request, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	var userID, passwordHash string
	if err := server.Database().QueryRow(`SELECT id, password_hash FROM users WHERE email = ? COLLATE NOCASE AND deleted_at IS NULL`, input.Email).Scan(&userID, &passwordHash); err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
		writeError(context.Writer, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials")
		return
	}
	_, _ = server.Database().Exec(`UPDATE users SET last_login = ? WHERE id = ?`, nowString(), userID)
	user, err := server.Profile(userID, true)
	if err != nil {
		writeError(context.Writer, http.StatusInternalServerError, "database_error", "Could not load the account")
		return
	}
	response, err := server.IssueSession(user, context.Request)
	if err != nil {
		writeError(context.Writer, http.StatusInternalServerError, "session_error", "Could not create a session")
		return
	}
	writeJSON(context.Writer, http.StatusOK, response)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/auth/login", route.AccessPublic),
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
	writeJSON(writer, status, map[string]any{"code": code, "message": message, "timestamp": nowString()})
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
