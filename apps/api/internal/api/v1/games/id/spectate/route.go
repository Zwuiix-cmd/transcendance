package spectate

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
	var private int
	if err := server.Database().QueryRow(`SELECT status,is_private FROM games WHERE id=?`, id).Scan(&status, &private); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Game was not found")
		} else {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
		}
		return
	}
	if status != "in_progress" {
		gamesdependencies.WriteError(context.Writer, http.StatusConflict, "game_not_live", "Only a game in progress can be spectated")
		return
	}
	if private != 0 {
		gamesdependencies.WriteError(context.Writer, http.StatusForbidden, "private_game", "Private games can only be viewed by their players")
		return
	}
	var player int
	if err := server.Database().QueryRow(`SELECT EXISTS(SELECT 1 FROM game_participants WHERE game_id=? AND user_id=?)`, id, userID).Scan(&player); err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	if player != 0 {
		gamesdependencies.WriteError(context.Writer, http.StatusConflict, "already_player", "Players reconnect through the player session")
		return
	}
	now := gamesdependencies.NowString()
	if _, err := server.Database().Exec(`INSERT INTO game_spectators(game_id,user_id,joined_at,last_seen_at) VALUES(?,?,?,?) ON CONFLICT(game_id,user_id) DO UPDATE SET last_seen_at=excluded.last_seen_at`, id, userID, now, now); err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
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
	Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/spectate", route.AccessUser),
}

type LeaveRoute struct {
	route.Route
}

func HandleLeave(context *route.Context) {
	server, userID, _, ok := gamesdependencies.RequireUser(context)
	if !ok {
		return
	}
	result, err := server.Database().Exec(`DELETE FROM game_spectators WHERE game_id=? AND user_id=?`, context.Param("id"), userID)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "not_spectating", "You are not spectating this game")
		return
	}
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (LeaveRoute) Handler() route.Handler {
	return HandleLeave
}

var LeaveDefinition = LeaveRoute{
	Route: route.NewRoute(http.MethodDelete, "/api/v1/games/{id}/spectate", route.AccessUser),
}
