package gameid

import (
	"database/sql"
	"errors"
	"net/http"

	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
)

type GetRoute struct {
	route.Route
}

func HandleGet(context *route.Context) {
	server, userID, _, ok := gamesdependencies.RequireUser(context)
	if !ok {
		return
	}
	detail, err := gamesdependencies.FetchGameDetail(server.Database(), context.Param("id"), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Lobby was not found")
		} else {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
		}
		return
	}
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, detail)
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
	notifier := gamesdependencies.Notify(context)
	notifier.PublishGameDeleted(id)
	notifier.PublishLobbies()
	context.Writer.WriteHeader(http.StatusNoContent)
}

func (DeleteRoute) Handler() route.Handler {
	return HandleDelete
}

var DeleteDefinition = DeleteRoute{
	Route: route.NewRoute(http.MethodDelete, "/api/v1/games/{id}", route.AccessUser),
}
