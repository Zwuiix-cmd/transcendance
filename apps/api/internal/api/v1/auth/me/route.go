package me

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
)

type Route struct {
	route.Route
}

type meDependencies interface {
	ProfileWithAvatar(string, bool) (model.User, error)
	UserID(any) (string, bool)
}

func Handle(context *route.Context) {
	server, ok := context.Server.(meDependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	value := context.Server.RequireUser(context.Writer, context.Request)
	if value == nil {
		return
	}
	userID, ok := server.UserID(value)
	if !ok {
		writeError(context.Writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return
	}
	profile, err := server.ProfileWithAvatar(userID, true)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	writeJSON(context.Writer, http.StatusOK, profile)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1/auth/me", route.AccessUser),
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
