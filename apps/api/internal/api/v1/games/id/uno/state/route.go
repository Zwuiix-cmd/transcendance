package state

import (
	"net/http"

	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
)

type Route struct{ route.Route }

func Handle(context *route.Context) {
	server, userID, ok := gamesdependencies.RequireUnoUser(context)
	if !ok {
		return
	}
	if !gamesdependencies.HasGameAccess(server.Database(), context.Param("id"), userID) {
		gamesdependencies.WriteError(context.Writer, http.StatusForbidden, "game_access_required", "Join or spectate this game before reading its state")
		return
	}
	snapshot, err := server.UnoGame().State(context.Param("id"), userID)
	if err != nil {
		gamesdependencies.WriteUnoError(context.Writer, err)
		return
	}
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, snapshot)
}

func (Route) Handler() route.Handler { return Handle }

var Definition = Route{Route: route.NewRoute(http.MethodGet, "/api/v1/games/{id}/uno/state", route.AccessUser)}
