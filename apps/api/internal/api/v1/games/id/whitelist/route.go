package whitelist

import (
	"net/http"
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
		Username string `json:"username"`
	}
	if !gamesdependencies.DecodeJSON(context.Writer, context.Request, &input) {
		return
	}
	id := context.Param("id")
	var owner string
	if err := server.Database().QueryRow(`SELECT created_by FROM games WHERE id=?`, id).Scan(&owner); err != nil {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Lobby was not found")
		return
	}
	if owner != userID {
		gamesdependencies.WriteError(context.Writer, http.StatusForbidden, "not_host", "Only the host can manage this lobby")
		return
	}
	var target string
	if err := server.Database().QueryRow(`SELECT id FROM users WHERE username=? COLLATE NOCASE AND deleted_at IS NULL`, strings.TrimSpace(input.Username)).Scan(&target); err != nil {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "user_not_found", "User was not found")
		return
	}
	if target == userID {
		gamesdependencies.WriteError(context.Writer, http.StatusBadRequest, "owner_cannot_whitelist", "The host cannot perform this action on themselves")
		return
	}
	if _, err := server.Database().Exec(`INSERT INTO game_whitelist(game_id,user_id,created_at) VALUES(?,?,?)`, id, target, gamesdependencies.NowString()); err != nil {
		if gamesdependencies.IsConstraint(err) {
			gamesdependencies.WriteError(context.Writer, http.StatusConflict, "already_whitelisted", "User is already on the whitelist")
		} else {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
		}
		return
	}
	gamesdependencies.Notify(context).PublishGame(id)
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/whitelist", route.AccessUser),
}
