package reconnect

import (
	"database/sql"
	"errors"
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
	var status string
	if err := server.Database().QueryRow(`SELECT status FROM games WHERE id=?`, id).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Game was not found")
		} else {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
		}
		return
	}
	if status == "finished" {
		gamesdependencies.WriteError(context.Writer, http.StatusConflict, "game_finished", "This game has already finished")
		return
	}
	result, err := server.Database().Exec(`UPDATE game_participants SET last_seen_at=? WHERE game_id=? AND user_id=?`, gamesdependencies.NowString(), id, userID)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		gamesdependencies.WriteError(context.Writer, http.StatusForbidden, "player_only", "Only a player already assigned to this game can reconnect")
		return
	}
	_, _ = server.Database().Exec(`DELETE FROM game_spectators WHERE game_id=? AND user_id=?`, id, userID)
	game, err := gamesdependencies.FetchGame(server.Database(), id, userID)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, gamesdependencies.GameResponse(game, userID))
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/reconnect", route.AccessUser),
}
