package rotate

import (
	"net/http"

	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
)

type Route struct {
	route.Route
}

func Handle(context *route.Context) {
	server, userID, _, ok := gamesdependencies.RequireUser(context)
	if !ok {
		return
	}
	id := context.Param("id")
	var private int
	var owner string
	if err := server.Database().QueryRow(`SELECT created_by,is_private FROM games WHERE id=?`, id).Scan(&owner, &private); err != nil || private == 0 || owner != userID {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Private lobby was not found or is not owned by you")
		return
	}
	if _, err := server.Database().Exec(`UPDATE games SET join_code=? WHERE id=?`, gamesdependencies.GenerateJoinCode(), id); err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	game, err := gamesdependencies.FetchGame(server.Database(), id, userID)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	gamesdependencies.Notify(context).PublishGame(id)
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, gamesdependencies.GameResponse(game, userID))
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/code/rotate", route.AccessUser),
}
