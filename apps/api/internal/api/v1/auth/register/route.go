package register

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Route struct {
	route.Route
}

type registrationServer interface {
	Database() *sql.DB
	Profile(string, bool) (model.User, error)
	IssueSession(model.User, *http.Request) (model.AuthResponse, error)
}

func Handle(context *route.Context) {
	server, ok := context.Server.(registrationServer)
	if !ok {
		route.NotImplemented(context)
		return
	}

	var input registerRequest
	if !decodeJSON(context.Writer, context.Request, &input) {
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if len(input.Username) < 3 || len(input.Username) > 24 || !usernamePattern.MatchString(input.Username) {
		writeError(context.Writer, http.StatusBadRequest, "invalid_username", "Username must contain 3 to 24 letters, numbers or underscores")
		return
	}
	if !learnerEmailPattern.MatchString(input.Email) {
		writeError(context.Writer, http.StatusBadRequest, "invalid_email", "Email must use the @learner.42.tech domain")
		return
	}
	if len(input.Password) < 12 || len(input.Password) > 128 {
		writeError(context.Writer, http.StatusBadRequest, "invalid_password", "Password must contain between 12 and 128 characters")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		writeError(context.Writer, http.StatusInternalServerError, "password_error", "Could not secure the password")
		return
	}
	id := uuid.NewString()
	created := nowString()
	tx, err := server.Database().Begin()
	if err != nil {
		writeError(context.Writer, http.StatusInternalServerError, "database_error", "Could not create the account")
		return
	}
	_, err = tx.Exec(`INSERT INTO users (id, username, email, password_hash, elo, role, created_at) VALUES (?, ?, ?, ?, 1000, 'player', ?)`, id, input.Username, input.Email, string(hash), created)
	if err == nil {
		_, err = tx.Exec(`INSERT INTO user_stats (user_id) VALUES (?)`, id)
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO user_elo_history (id,user_id,previous_elo,new_elo,delta,reason,created_at) VALUES (?,?,?,?,?,?,?)`, uuid.NewString(), id, 1000, 1000, 0, "account_created", created)
	}
	if err != nil {
		_ = tx.Rollback()
		if isConstraint(err) {
			writeError(context.Writer, http.StatusConflict, "user_exists", "Username or email is already in use")
		} else {
			writeError(context.Writer, http.StatusInternalServerError, "database_error", "Could not create the account")
		}
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(context.Writer, http.StatusInternalServerError, "database_error", "Could not create the account")
		return
	}
	user, err := server.Profile(id, true)
	if err != nil {
		writeError(context.Writer, http.StatusInternalServerError, "database_error", "Could not load the account")
		return
	}
	response, err := server.IssueSession(user, context.Request)
	if err != nil {
		writeError(context.Writer, http.StatusInternalServerError, "session_error", "Could not create a session")
		return
	}
	writeJSON(context.Writer, http.StatusCreated, response)
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

var (
	learnerEmailPattern = regexp.MustCompile(`(?i)^[^\s@]+@learner\.42\.tech$`)
	usernamePattern     = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
)

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(target); err != nil {
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

func isConstraint(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "constraint") || strings.Contains(message, "unique")
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/auth/register", route.AccessPublic),
}
