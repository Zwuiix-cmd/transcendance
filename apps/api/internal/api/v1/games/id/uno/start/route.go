package start

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
	snapshot, err := server.UnoGame().Start(context.Param("id"), userID)
	if err != nil {
		gamesdependencies.WriteUnoError(context.Writer, err)
		return
	}
	notifier := gamesdependencies.Notify(context)
	notifier.PublishGame(context.Param("id"))
	notifier.PublishLobbies()
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, snapshot)
}

func (Route) Handler() route.Handler { return Handle }

var Definition = Route{Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/uno/start", route.AccessUser)}
