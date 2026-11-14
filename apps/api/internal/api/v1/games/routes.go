package games

import (
	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/api/v1/games/id"
	"card-arena/api/internal/api/v1/games/id/code/rotate"
	gamejoin "card-arena/api/internal/api/v1/games/id/join"
	"card-arena/api/internal/api/v1/games/id/leave"
	"card-arena/api/internal/api/v1/games/id/participants/user-id"
	"card-arena/api/internal/api/v1/games/id/reconnect"
	"card-arena/api/internal/api/v1/games/id/result"
	"card-arena/api/internal/api/v1/games/id/spectate"
	"card-arena/api/internal/api/v1/games/id/status"
	"card-arena/api/internal/api/v1/games/id/uno/action"
	"card-arena/api/internal/api/v1/games/id/uno/draw"
	"card-arena/api/internal/api/v1/games/id/uno/play"
	"card-arena/api/internal/api/v1/games/id/uno/start"
	"card-arena/api/internal/api/v1/games/id/uno/state"
	"card-arena/api/internal/api/v1/games/id/whitelist"
	"card-arena/api/internal/api/v1/games/id/whitelist/user-id"
	"card-arena/api/internal/api/v1/games/join"
)

func Definitions() []route.RegisteredRoute {
	return []route.RegisteredRoute{
		ListDefinition,
		CreateDefinition,
		join.Definition,
		result.Definition,
		gameid.GetDefinition,
		gameid.DeleteDefinition,
		gamejoin.Definition,
		leave.Definition,
		status.Definition,
		state.Definition,
		start.Definition,
		draw.Definition,
		play.Definition,
		action.Definition,
		rotate.Definition,
		whitelist.Definition,
		whitelistuser.Definition,
		participantuser.Definition,
		reconnect.Definition,
		spectate.Definition,
		spectate.LeaveDefinition,
	}
}
