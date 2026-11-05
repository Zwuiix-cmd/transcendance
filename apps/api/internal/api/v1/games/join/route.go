package join

import (
	"net/http"
	"regexp"
	"strings"

	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
	idjoin "card-arena/api/internal/api/v1/games/id/join"
)

type Route struct {
	route.Route
}

func Handle(context *route.Context) {
	server, userID, _, ok := gamesdependencies.RequireUser(context)
	if !ok {
		return
	}
	var input joinGameRequest
	if !gamesdependencies.DecodeJSON(context.Writer, context.Request, &input) {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	if code == "" || len(code) > 16 || !joinCodePattern.MatchString(code) {
		gamesdependencies.WriteError(context.Writer, http.StatusForbidden, "invalid_code", "The lobby code is invalid")
		return
	}
	var gameID string
	if err := server.Database().QueryRow(`SELECT id FROM games WHERE join_code=? AND status='waiting'`, code).Scan(&gameID); err != nil {
		gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Lobby was not found")
		return
	}
	idjoin.JoinGame(context, server.Database(), gameID, userID, code)
}

type joinGameRequest struct {
	Code string `json:"code"`
}

var joinCodePattern = regexp.MustCompile(`^[A-Za-z0-9]*$`)

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/games/join", route.AccessUser),
}
