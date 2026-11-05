package leave

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
	var owner string
	if err := server.Database().QueryRow(`SELECT created_by FROM games WHERE id=?`, context.Param("id")).Scan(&owner); err != nil {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Lobby was not found")
		return
	}
	if owner == userID {
		_, err := server.Database().Exec(`UPDATE game_participants SET last_seen_at=? WHERE game_id=? AND user_id=?`, gamesdependencies.NowString(), context.Param("id"), userID)
		if err != nil {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
			return
		}
		context.Writer.WriteHeader(http.StatusNoContent)
		return
	}
	result, err := server.Database().Exec(`DELETE FROM game_participants WHERE game_id=? AND user_id=?`, context.Param("id"), userID)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "not_member", "User is not in this lobby")
		return
	}
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/leave", route.AccessUser),
}
