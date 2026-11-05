package queue

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/matchmaking"
)

type queueServer interface {
	UserID(any) (string, bool)
	UserElo(any) (int, bool)
	RouteQueue() *matchmaking.Queue
}

type JoinRoute struct {
	route.Route
}

func HandleJoin(context *route.Context) {
	server, userID, elo, ok := requireUser(context)
	if !ok {
		return
	}
	var input struct {
		Mode string `json:"mode"`
	}
	if !decodeJSON(context.Writer, context.Request, &input) {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	if mode != "ranked" && mode != "unranked" {
		writeError(context.Writer, http.StatusBadRequest, "invalid_queue_mode", "Queue mode must be ranked or unranked")
		return
	}
	writeJSON(context.Writer, http.StatusOK, server.RouteQueue().Join(userID, mode, elo))
}

func (JoinRoute) Handler() route.Handler {
	return HandleJoin
}

var JoinDefinition = JoinRoute{
	Route: route.NewRoute(http.MethodPost, "/api/v1/matchmaking/queue", route.AccessUser),
}

type StatusRoute struct {
	route.Route
}

func HandleStatus(context *route.Context) {
	server, userID, _, ok := requireUser(context)
	if !ok {
		return
	}
	writeJSON(context.Writer, http.StatusOK, server.RouteQueue().Status(userID))
}

func (StatusRoute) Handler() route.Handler {
	return HandleStatus
}

var StatusDefinition = StatusRoute{
	Route: route.NewRoute(http.MethodGet, "/api/v1/matchmaking/queue", route.AccessUser),
}

type LeaveRoute struct {
	route.Route
}

func HandleLeave(context *route.Context) {
	server, userID, _, ok := requireUser(context)
	if !ok {
		return
	}
	server.RouteQueue().Leave(userID)
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (LeaveRoute) Handler() route.Handler {
	return HandleLeave
}

var LeaveDefinition = LeaveRoute{
	Route: route.NewRoute(http.MethodDelete, "/api/v1/matchmaking/queue", route.AccessUser),
}

func dependencies(context *route.Context) (queueServer, bool) {
	server, ok := context.Server.(queueServer)
	if !ok {
		route.NotImplemented(context)
		return nil, false
	}
	return server, true
}

func requireUser(context *route.Context) (queueServer, string, int, bool) {
	server, ok := dependencies(context)
	if !ok {
		return nil, "", 0, false
	}
	id, validID := server.UserID(context.User)
	elo, validElo := server.UserElo(context.User)
	if !validID || !validElo {
		writeError(context.Writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return nil, "", 0, false
	}
	return server, id, elo, true
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
