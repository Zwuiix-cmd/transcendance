package auth

import (
	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/api/v1/auth/login"
	"card-arena/api/internal/api/v1/auth/logout"
	"card-arena/api/internal/api/v1/auth/me"
	"card-arena/api/internal/api/v1/auth/register"
)

func Definitions() []route.RegisteredRoute {
	return []route.RegisteredRoute{
		register.Definition,
		login.Definition,
		logout.Definition,
		me.Definition,
	}
}
