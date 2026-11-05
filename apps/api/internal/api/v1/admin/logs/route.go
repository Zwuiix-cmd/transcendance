package logs

import (
	"encoding/json"
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

// Filter contains only the HTTP-level filters accepted by the audit log route.
// The API server translates it into its database query representation.
type Filter struct {
	Days     int
	Severity string
	Search   string
	Actor    string
	Category string
	Status   string
	Offset   int
}

type auditLogReader interface {
	ReadAuditLogs(limit int, filter Filter) ([]model.AuditLog, error)
}

func Handle(context *route.Context) {
	filter := parseFilter(context.Request)
	if !filter.valid() {
		writeError(context.Writer, http.StatusBadRequest, "invalid_log_filter", "Unknown audit log filter")
		return
	}

	reader, ok := context.Server.(auditLogReader)
	if !ok {
		writeError(context.Writer, http.StatusInternalServerError, "logs_unavailable", "Audit logs are unavailable")
		return
	}

	limit := queryInt(context.Request, "limit", 100)
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	page := queryInt(context.Request, "page", 1)
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	filter.Offset = (page - 1) * limit
	logs, err := reader.ReadAuditLogs(limit+1, filter)
	if err != nil {
		slog.Error("could not read audit logs", "error", err)
		writeError(context.Writer, http.StatusInternalServerError, "database_error", "The request could not be completed")
		return
	}
	hasMore := len(logs) > limit
	if hasMore {
		logs = logs[:limit]
	}
	context.Writer.Header().Set("X-Pagination-Page", strconv.Itoa(page))
	context.Writer.Header().Set("X-Pagination-Limit", strconv.Itoa(limit))
	context.Writer.Header().Set("X-Pagination-Has-More", strconv.FormatBool(hasMore))
	writeJSON(context.Writer, http.StatusOK, logs)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1/admin/logs", route.AccessAdmin),
}

func parseFilter(request *http.Request) Filter {
	query := request.URL.Query()
	return Filter{
		Days:     queryInt(request, "days", 14),
		Severity: strings.ToLower(strings.TrimSpace(query.Get("severity"))),
		Search:   strings.TrimSpace(query.Get("search")),
		Actor:    strings.TrimSpace(query.Get("actor")),
		Category: strings.ToLower(strings.TrimSpace(query.Get("category"))),
		Status:   strings.ToLower(strings.TrimSpace(query.Get("status"))),
	}
}

func (filter Filter) valid() bool {
	if filter.Days < 1 || filter.Days > 90 {
		return false
	}
	if filter.Category != "" && filter.Category != "all" && filter.Category != "requests" && filter.Category != "security" && filter.Category != "admin" && filter.Category != "auth" && filter.Category != "games" {
		return false
	}
	if filter.Status != "" && filter.Status != "all" && filter.Status != "success" && filter.Status != "client_error" && filter.Status != "server_error" {
		return false
	}
	return filter.Severity == "" || filter.Severity == "info" || filter.Severity == "warning" || filter.Severity == "critical"
}

func queryInt(request *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(request.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{
		"code":      code,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
}
