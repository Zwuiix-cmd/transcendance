package v1

import (
	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/api/v1/admin"
	"card-arena/api/internal/api/v1/auth"
	"card-arena/api/internal/api/v1/games"
	"card-arena/api/internal/api/v1/leaderboard"
	"card-arena/api/internal/api/v1/matchmaking"
	"card-arena/api/internal/api/v1/telemetry"
	"card-arena/api/internal/api/v1/users"
)

func Definitions() []route.RegisteredRoute {
	definitions := []route.RegisteredRoute{Definition}
	definitions = append(definitions, auth.Definitions()...)
	definitions = append(definitions, users.Definitions()...)
	definitions = append(definitions, leaderboard.Definition)
	definitions = append(definitions, games.Definitions()...)
	definitions = append(definitions, matchmaking.Definitions()...)
	definitions = append(definitions, telemetry.Definition)
	definitions = append(definitions, admin.Definitions()...)
	return definitions
}
