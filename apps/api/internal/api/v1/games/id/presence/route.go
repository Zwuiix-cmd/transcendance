package presence

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
)

type Route struct {
	route.Route
}

type presenceRequest struct {
	Role string `json:"role"`
}

func Handle(context *route.Context) {
	server, userID, _, ok := gamesdependencies.RequireUser(context)
	if !ok {
		return
	}
	var input presenceRequest
	if err := json.NewDecoder(http.MaxBytesReader(context.Writer, context.Request.Body, 4096)).Decode(&input); err != nil {
		gamesdependencies.WriteError(context.Writer, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return
	}
	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role != "player" && role != "spectator" {
		gamesdependencies.WriteError(context.Writer, http.StatusBadRequest, "invalid_presence_role", "Presence role must be player or spectator")
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
		context.Writer.WriteHeader(http.StatusNoContent)
		return
	}
	var result sql.Result
	var err error
	if role == "player" {
		result, err = server.Database().Exec(`UPDATE game_participants SET last_seen_at=? WHERE game_id=? AND user_id=?`, gamesdependencies.NowString(), id, userID)
	} else {
		result, err = server.Database().Exec(`UPDATE game_spectators SET last_seen_at=? WHERE game_id=? AND user_id=?`, gamesdependencies.NowString(), id, userID)
	}
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		gamesdependencies.WriteError(context.Writer, http.StatusForbidden, "presence_forbidden", "You do not have an active role in this game")
		return
	}
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/presence", route.AccessUser),
}
