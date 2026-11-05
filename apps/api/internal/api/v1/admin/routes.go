package admin

import (
	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/api/v1/admin/dashboard"
	"card-arena/api/internal/api/v1/admin/logs"
	"card-arena/api/internal/api/v1/admin/logs/stats"
	"card-arena/api/internal/api/v1/admin/overview"
	"card-arena/api/internal/api/v1/admin/users"
	"card-arena/api/internal/api/v1/admin/users/id"
	"card-arena/api/internal/api/v1/admin/users/id/analytics"
	"card-arena/api/internal/api/v1/admin/users/id/elo"
	"card-arena/api/internal/api/v1/admin/users/id/role"
)

func Definitions() []route.RegisteredRoute {
	return []route.RegisteredRoute{
		overview.Definition,
		dashboard.Definition,
		logs.Definition,
		stats.Definition,
		users.Definition,
		analytics.Definition,
		userid.Definition,
		elo.Definition,
		role.Definition,
	}
}
