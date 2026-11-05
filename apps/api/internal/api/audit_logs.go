package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"card-arena/api/internal/model"
)

type logFilter struct {
	days                                      int
	severity, search, actor, category, status string
	offset                                    int
}

func (filter logFilter) where() (string, []any) {
	where := []string{"1=1"}
	args := []any{}
	if filter.days > 0 {
		where = append(where, "created_at>=?")
		args = append(args, time.Now().UTC().Add(-time.Duration(filter.days)*24*time.Hour).Format(time.RFC3339Nano))
	}
	if filter.severity != "" {
		where = append(where, "severity=?")
		args = append(args, filter.severity)
	}
	if filter.search != "" {
		where = append(where, "(lower(COALESCE(actor_username,'')) LIKE lower(?) OR lower(action) LIKE lower(?) OR lower(COALESCE(target_id,'')) LIKE lower(?) OR lower(details) LIKE lower(?))")
		pattern := "%" + filter.search + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if filter.actor != "" {
		where = append(where, "lower(COALESCE(actor_username,'')) LIKE lower(?)")
		args = append(args, "%"+filter.actor+"%")
	}
	switch filter.category {
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
	switch filter.status {
	case "success":
		where = append(where, "action='http.request' AND (details LIKE '% -> 2%' OR details LIKE '% -> 3%')")
	case "client_error":
		where = append(where, "action='http.request' AND details LIKE '% -> 4%'")
	case "server_error":
		where = append(where, "action='http.request' AND details LIKE '% -> 5%'")
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func (server *Server) auditLogs(limit int, filter logFilter) ([]model.AuditLog, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}
	where, args := filter.where()
	query := `SELECT id,actor_username,action,target_type,target_id,details,severity,created_at,ip_address,user_agent FROM audit_logs` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, filter.offset)
	rows, err := server.store.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := []model.AuditLog{}
	for rows.Next() {
		var item model.AuditLog
		var actor, target, ip, agent sql.NullString
		if err := rows.Scan(&item.ID, &actor, &item.Action, &item.TargetType, &target, &item.Details, &item.Severity, &item.CreatedAt, &ip, &agent); err != nil {
			return nil, err
		}
		item.ActorUsername = nullString(actor)
		item.TargetID = nullString(target)
		item.IPAddress = nullString(ip)
		item.UserAgent = nullString(agent)
		if code := statusFromDetails(item.Details); code > 0 {
			item.StatusCode = &code
		}
		logs = append(logs, item)
	}
	return logs, rows.Err()
}

func statusFromDetails(details string) int {
	marker := strings.LastIndex(details, " -> ")
	if marker < 0 {
		return 0
	}
	code, err := strconv.Atoi(strings.TrimSpace(details[marker+4:]))
	if err != nil {
		return 0
	}
	return code
}

func parseLogFilter(request *http.Request) logFilter {
	return logFilter{
		days:     queryInt(request, "days", 14),
		severity: strings.ToLower(strings.TrimSpace(request.URL.Query().Get("severity"))),
		search:   strings.TrimSpace(request.URL.Query().Get("search")),
		actor:    strings.TrimSpace(request.URL.Query().Get("actor")),
		category: strings.ToLower(strings.TrimSpace(request.URL.Query().Get("category"))),
		status:   strings.ToLower(strings.TrimSpace(request.URL.Query().Get("status"))),
	}
}

func validLogFilter(filter logFilter) bool {
	if filter.days < 1 || filter.days > 90 {
		return false
	}
	if filter.category != "" && filter.category != "all" && filter.category != "requests" && filter.category != "security" && filter.category != "admin" && filter.category != "auth" && filter.category != "games" {
		return false
	}
	if filter.status != "" && filter.status != "all" && filter.status != "success" && filter.status != "client_error" && filter.status != "server_error" {
		return false
	}
	if filter.severity != "" && filter.severity != "info" && filter.severity != "warning" && filter.severity != "critical" {
		return false
	}
	return true
}
