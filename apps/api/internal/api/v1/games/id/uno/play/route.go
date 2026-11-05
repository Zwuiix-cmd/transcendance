package play

import (
	"net/http"

	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
)

type Route struct{ route.Route }

type request struct {
	CardIndex int    `json:"cardIndex"`
	Color     string `json:"color"`
}

func Handle(context *route.Context) {
	server, userID, ok := gamesdependencies.RequireUnoUser(context)
	if !ok {
		return
	}
	var input request
	if !gamesdependencies.DecodeJSON(context.Writer, context.Request, &input) {
		return
	}
	snapshot, err := server.UnoGame().Play(context.Param("id"), userID, input.CardIndex, input.Color)
	if err != nil {
		gamesdependencies.WriteUnoError(context.Writer, err)
		return
	}
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, snapshot)
}

func (Route) Handler() route.Handler { return Handle }

var Definition = Route{Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/uno/play", route.AccessUser)}
