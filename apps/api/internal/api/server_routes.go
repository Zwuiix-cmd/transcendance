package api

import (
	"net/http"

	routecontract "card-arena/api/internal/api/route"
	adminlogs "card-arena/api/internal/api/v1/admin/logs"
	"card-arena/api/internal/model"
)

var _ routecontract.Runtime = (*Server)(nil)

func (server *Server) Authenticated(request *http.Request) any {
	return server.authenticated(request)
}

func (server *Server) RequireUser(writer http.ResponseWriter, request *http.Request) any {
	return server.requireUser(writer, request)
}

func (server *Server) RequireAdmin(writer http.ResponseWriter, request *http.Request) any {
	return server.requireAdmin(writer, request)
}

func (server *Server) RequireGameService(writer http.ResponseWriter, request *http.Request) bool {
	return server.requireGameService(writer, request)
}

func (server *Server) ReadAuditLogs(limit int, filter adminlogs.Filter) ([]model.AuditLog, error) {
	return server.auditLogs(limit, logFilter{
		days:     filter.Days,
		severity: filter.Severity,
		search:   filter.Search,
		actor:    filter.Actor,
		category: filter.Category,
		status:   filter.Status,
		offset:   filter.Offset,
	})
}
