package whitelistuser

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
	id, target := context.Param("id"), context.Param("userId")
	var owner string
	if err := server.Database().QueryRow(`SELECT created_by FROM games WHERE id=?`, id).Scan(&owner); err != nil {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Lobby was not found")
		return
	}
	if owner != userID {
		gamesdependencies.WriteError(context.Writer, http.StatusForbidden, "not_host", "Only the host can manage this lobby")
		return
	}
	result, err := server.Database().Exec(`DELETE FROM game_whitelist WHERE game_id=? AND user_id=?`, id, target)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "not_whitelisted", "User is not on the lobby whitelist")
		return
	}
	gamesdependencies.Notify(context).PublishGame(id)
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodDelete, "/api/v1/games/{id}/whitelist/{userId}", route.AccessUser),
}
