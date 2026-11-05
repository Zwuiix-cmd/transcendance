package matchmaking

import (
	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/api/v1/matchmaking/queue"
)

func Definitions() []route.RegisteredRoute {
	return []route.RegisteredRoute{
		queue.JoinDefinition,
		queue.StatusDefinition,
		queue.LeaveDefinition,
	}
}
