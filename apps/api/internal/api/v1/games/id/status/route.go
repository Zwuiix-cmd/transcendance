package status

import (
	"net/http"
	"regexp"
	"strings"

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
	var input struct {
		Status string `json:"status"`
	}
	if !gamesdependencies.DecodeJSON(context.Writer, context.Request, &input) {
		return
	}
	input.Status = strings.TrimSpace(input.Status)
	if !gameStatusPattern.MatchString(input.Status) {
		gamesdependencies.WriteError(context.Writer, http.StatusBadRequest, "invalid_status", "Status must be waiting, in_progress or finished")
		return
	}
	if input.Status == "finished" {
		gamesdependencies.WriteError(context.Writer, http.StatusConflict, "game_result_required", "Finished games must be settled through the game result endpoint")
		return
	}
	id := context.Param("id")
	var owner string
	if err := server.Database().QueryRow(`SELECT created_by FROM games WHERE id=?`, id).Scan(&owner); err != nil || owner != userID {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Game was not found or is not owned by you")
		return
	}
	var err error
	if input.Status == "waiting" {
		_, err = server.Database().Exec(`UPDATE games SET status=? WHERE id=?`, input.Status, id)
	} else {
		_, err = server.Database().Exec(`UPDATE games SET status=?,started_at=? WHERE id=?`, input.Status, gamesdependencies.NowString(), id)
	}
	if err != nil {
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

var gameStatusPattern = regexp.MustCompile(`^(waiting|in_progress|finished)$`)

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPut, "/api/v1/games/{id}/status", route.AccessUser),
}
