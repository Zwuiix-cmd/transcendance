package logout

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	route "card-arena/api/internal/api/route"
)

type Route struct {
	route.Route
}

type logoutDependencies interface {
	Database() *sql.DB
}

func Handle(context *route.Context) {
	server, ok := context.Server.(logoutDependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	if context.Server.RequireUser(context.Writer, context.Request) == nil {
		return
	}
	header := strings.TrimSpace(context.Request.Header.Get("Authorization"))
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" || header == token {
		writeError(context.Writer, http.StatusUnauthorized, "token_required", "A bearer token is required")
		return
	}
	_, _ = server.Database().Exec(`UPDATE auth_sessions SET revoked_at = ? WHERE token_hash = ?`, nowString(), tokenHash(token))
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/auth/logout", route.AccessUser),
}

func tokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
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
