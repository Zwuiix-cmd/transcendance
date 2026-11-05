package users

import (
	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/api/v1/users/id"
	"card-arena/api/internal/api/v1/users/id/analytics"
	"card-arena/api/internal/api/v1/users/me"
	"card-arena/api/internal/api/v1/users/me/avatar"
	"card-arena/api/internal/api/v1/users/me/password"
	"card-arena/api/internal/api/v1/users/search"
)

func Definitions() []route.RegisteredRoute {
	return []route.RegisteredRoute{
		search.Definition,
		me.UpdateDefinition,
		me.DeleteDefinition,
		password.Definition,
		avatar.Definition,
		analytics.Definition,
		userid.Definition,
	}
}
