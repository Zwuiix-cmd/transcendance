package challenge

import (
	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
	"net/http"
)

type Route struct{ route.Route }

func Handle(context *route.Context) {
	server, userID, ok := gamesdependencies.RequireUnoUser(context)
	if !ok {
		return
	}
	snapshot, err := server.UnoGame().ChallengeDrawFour(context.Param("id"), userID)
	if err != nil {
		gamesdependencies.WriteUnoError(context.Writer, err)
		return
	}
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, snapshot)
}

func (Route) Handler() route.Handler { return Handle }

var Definition = Route{Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/uno/challenge", route.AccessUser)}
