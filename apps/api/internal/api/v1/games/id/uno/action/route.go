package action

import (
	"net/http"

	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
	unogame "card-arena/api/internal/uno"
)

type Route struct{ route.Route }

func Handle(context *route.Context) {
	server, userID, ok := gamesdependencies.RequireUnoUser(context)
	if !ok {
		return
	}
	var input unogame.Action
	if !gamesdependencies.DecodeJSON(context.Writer, context.Request, &input) {
		return
	}
	snapshot, err := server.UnoGame().Act(context.Param("id"), userID, input)
	if err != nil {
		gamesdependencies.WriteUnoError(context.Writer, err)
		return
	}
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, snapshot)
}

func (Route) Handler() route.Handler { return Handle }

var Definition = Route{Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/uno/action", route.AccessUser)}
