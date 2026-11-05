package stats

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
)

type Route struct {
	route.Route
}

type dependencies interface {
	Database() *sql.DB
}

type filter struct {
	days                                      int
	severity, search, actor, category, status string
}

func Handle(context *route.Context) {
	server, ok := context.Server.(dependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	requestFilter := parseFilter(context.Request)
	if !requestFilter.valid() {
		writeError(context.Writer, http.StatusBadRequest, "invalid_log_filter", "Unknown audit log filter")
		return
	}
	where, args := requestFilter.where()
	var stats model.AdminLogStats
	err := server.Database().QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN action='http.request' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN action='http.request' AND (details LIKE '% -> 2%' OR details LIKE '% -> 3%') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN action='http.request' AND details LIKE '% -> 4%' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN action='http.request' AND details LIKE '% -> 5%' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN severity IN ('warning','critical') OR action LIKE 'security.%' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN action LIKE 'admin.%' THEN 1 ELSE 0 END),0),COUNT(DISTINCT NULLIF(trim(actor_username),'')) FROM audit_logs`+where, args...).Scan(&stats.TotalEvents, &stats.TotalRequests, &stats.SuccessfulRequests, &stats.ClientErrors, &stats.ServerErrors, &stats.SecurityEvents, &stats.AdminActions, &stats.ActiveActors)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	actorsQuery := `SELECT COALESCE(NULLIF(trim(actor_username),''),'System'),COUNT(*),SUM(CASE WHEN action='http.request' THEN 1 ELSE 0 END),SUM(CASE WHEN action='http.request' AND (details LIKE '% -> 2%' OR details LIKE '% -> 3%') THEN 1 ELSE 0 END),SUM(CASE WHEN action='http.request' AND (details LIKE '% -> 4%' OR details LIKE '% -> 5%') THEN 1 ELSE 0 END),SUM(CASE WHEN severity IN ('warning','critical') OR action LIKE 'security.%' THEN 1 ELSE 0 END),MAX(created_at) FROM audit_logs` + where + ` GROUP BY COALESCE(NULLIF(trim(actor_username),''),'System') ORDER BY 3 DESC,2 DESC,7 DESC LIMIT 100`
	actorRows, err := server.Database().Query(actorsQuery, args...)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	defer actorRows.Close()
	stats.Actors = []model.AdminActorStats{}
	for actorRows.Next() {
		var actor model.AdminActorStats
		if err := actorRows.Scan(&actor.Username, &actor.TotalEvents, &actor.Requests, &actor.SuccessfulRequests, &actor.FailedRequests, &actor.SecurityEvents, &actor.LastSeen); err != nil {
			writeDatabaseError(context.Writer, err)
			return
		}
		stats.Actors = append(stats.Actors, actor)
	}
	if err := actorRows.Err(); err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	writeJSON(context.Writer, http.StatusOK, stats)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1/admin/logs/stats", route.AccessAdmin),
}

func parseFilter(request *http.Request) filter {
	query := request.URL.Query()
	return filter{
		days:     queryInt(request, "days", 14),
		severity: strings.ToLower(strings.TrimSpace(query.Get("severity"))),
		search:   strings.TrimSpace(query.Get("search")),
		actor:    strings.TrimSpace(query.Get("actor")),
		category: strings.ToLower(strings.TrimSpace(query.Get("category"))),
		status:   strings.ToLower(strings.TrimSpace(query.Get("status"))),
	}
}

func (requestFilter filter) valid() bool {
	if requestFilter.days < 1 || requestFilter.days > 90 {
		return false
	}
	if requestFilter.category != "" && requestFilter.category != "all" && requestFilter.category != "requests" && requestFilter.category != "security" && requestFilter.category != "admin" && requestFilter.category != "auth" && requestFilter.category != "games" {
		return false
	}
	if requestFilter.status != "" && requestFilter.status != "all" && requestFilter.status != "success" && requestFilter.status != "client_error" && requestFilter.status != "server_error" {
		return false
	}
	return requestFilter.severity == "" || requestFilter.severity == "info" || requestFilter.severity == "warning" || requestFilter.severity == "critical"
}

func (requestFilter filter) where() (string, []any) {
	where := []string{"1=1"}
	args := []any{}
	if requestFilter.days > 0 {
		where = append(where, "created_at>=?")
		args = append(args, time.Now().UTC().Add(-time.Duration(requestFilter.days)*24*time.Hour).Format(time.RFC3339Nano))
	}
	if requestFilter.severity != "" {
		where = append(where, "severity=?")
		args = append(args, requestFilter.severity)
	}
	if requestFilter.search != "" {
		where = append(where, "(lower(COALESCE(actor_username,'')) LIKE lower(?) OR lower(action) LIKE lower(?) OR lower(COALESCE(target_id,'')) LIKE lower(?) OR lower(details) LIKE lower(?))")
		pattern := "%" + requestFilter.search + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if requestFilter.actor != "" {
		where = append(where, "lower(COALESCE(actor_username,'')) LIKE lower(?)")
		args = append(args, "%"+requestFilter.actor+"%")
	}
	switch requestFilter.category {
	case "requests":
		where = append(where, "action='http.request'")
	case "security":
		where = append(where, "(action LIKE 'security.%' OR severity IN ('warning','critical'))")
	case "admin":
		where = append(where, "action LIKE 'admin.%'")
	case "auth":
		where = append(where, "details LIKE '%/api/v1/auth/%'")
	case "games":
		where = append(where, "(details LIKE '%/api/v1/games%' OR details LIKE '%/api/v1/matchmaking%')")
	}
	switch requestFilter.status {
	case "success":
		where = append(where, "action='http.request' AND (details LIKE '% -> 2%' OR details LIKE '% -> 3%')")
	case "client_error":
		where = append(where, "action='http.request' AND details LIKE '% -> 4%'")
	case "server_error":
		where = append(where, "action='http.request' AND details LIKE '% -> 5%'")
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func queryInt(request *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(request.URL.Query().Get(key))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
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
