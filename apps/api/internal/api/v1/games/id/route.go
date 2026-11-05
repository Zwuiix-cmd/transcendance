package gameid

import (
	"database/sql"
	"errors"
	"net/http"

	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
	"card-arena/api/internal/model"
)

type GetRoute struct {
	route.Route
}

func HandleGet(context *route.Context) {
	server, userID, _, ok := gamesdependencies.RequireUser(context)
	if !ok {
		return
	}
	id := context.Param("id")
	game, err := gamesdependencies.FetchGame(server.Database(), id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Lobby was not found")
		} else {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
		}
		return
	}
	participantsRows, err := server.Database().Query(`SELECT p.user_id,u.username,CASE WHEN g.created_by=p.user_id THEN 1 ELSE 0 END FROM game_participants p JOIN users u ON u.id=p.user_id JOIN games g ON g.id=p.game_id WHERE p.game_id=? ORDER BY CASE WHEN g.created_by=p.user_id THEN 0 ELSE 1 END ASC,u.username ASC`, id)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	defer participantsRows.Close()
	participants := []model.LobbyParticipant{}
	for participantsRows.Next() {
		var participant model.LobbyParticipant
		var host int
		if err := participantsRows.Scan(&participant.UserID, &participant.Username, &host); err != nil {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
			return
		}
		participant.Host = host != 0
		participants = append(participants, participant)
	}
	if err := participantsRows.Err(); err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	spectatorRows, err := server.Database().Query(`SELECT s.user_id,u.username FROM game_spectators s JOIN users u ON u.id=s.user_id WHERE s.game_id=? ORDER BY u.username ASC`, id)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	defer spectatorRows.Close()
	spectators := []model.LobbySpectator{}
	for spectatorRows.Next() {
		var spectator model.LobbySpectator
		if err := spectatorRows.Scan(&spectator.UserID, &spectator.Username); err != nil {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
			return
		}
		spectators = append(spectators, spectator)
	}
	if err := spectatorRows.Err(); err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	whitelist := []model.WhitelistEntry{}
	if game.CreatedBy.Valid && game.CreatedBy.String == userID {
		rows, queryErr := server.Database().Query(`SELECT w.user_id,u.username FROM game_whitelist w JOIN users u ON u.id=w.user_id WHERE w.game_id=? ORDER BY u.username ASC`, id)
		if queryErr != nil {
			gamesdependencies.WriteDatabaseError(context.Writer, queryErr)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var item model.WhitelistEntry
			if err := rows.Scan(&item.UserID, &item.Username); err != nil {
				gamesdependencies.WriteDatabaseError(context.Writer, err)
				return
			}
			whitelist = append(whitelist, item)
		}
		if err := rows.Err(); err != nil {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
			return
		}
	}
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, model.GameDetail{Game: gamesdependencies.GameResponse(game, userID), Participants: participants, Spectators: spectators, Whitelist: whitelist})
}

func (GetRoute) Handler() route.Handler {
	return HandleGet
}

var GetDefinition = GetRoute{
	Route: route.NewRoute(http.MethodGet, "/api/v1/games/{id}", route.AccessUser),
}

type DeleteRoute struct {
	route.Route
}

func HandleDelete(context *route.Context) {
	server, userID, _, ok := gamesdependencies.RequireUser(context)
	if !ok {
		return
	}
	id := context.Param("id")
	var status string
	if err := server.Database().QueryRow(`SELECT status FROM games WHERE id=? AND created_by=?`, id, userID).Scan(&status); err != nil {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Game was not found or is not owned by you")
		return
	}
	if status == "in_progress" {
		gamesdependencies.WriteError(context.Writer, http.StatusConflict, "game_in_progress", "An in-progress game cannot be deleted")
		return
	}
	result, err := server.Database().Exec(`DELETE FROM games WHERE id=? AND created_by=?`, id, userID)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Game was not found or is not owned by you")
		return
	}
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (DeleteRoute) Handler() route.Handler {
	return HandleDelete
}

var DeleteDefinition = DeleteRoute{
	Route: route.NewRoute(http.MethodDelete, "/api/v1/games/{id}", route.AccessUser),
}
