package join

import (
	"database/sql"
	"errors"
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
	var input joinGameRequest
	if context.Request.ContentLength != 0 && !gamesdependencies.DecodeJSON(context.Writer, context.Request, &input) {
		return
	}
	JoinGame(context, server.Database(), context.Param("id"), userID, strings.ToUpper(strings.TrimSpace(input.Code)))
}

type joinGameRequest struct {
	Code string `json:"code"`
}

func JoinGame(context *route.Context, database *sql.DB, id, userID, code string) {
	tx, err := database.Begin()
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	var status string
	var private int
	var storedCode sql.NullString
	var maxPlayers, count, whitelistCount int
	err = tx.QueryRow(`SELECT g.status,g.is_private,g.join_code,COALESCE(gs.max_players,4),(SELECT COUNT(*) FROM game_participants p WHERE p.game_id=g.id),(SELECT COUNT(*) FROM game_whitelist w WHERE w.game_id=g.id) FROM games g LEFT JOIN game_settings gs ON gs.game_id=g.id WHERE g.id=?`, id).Scan(&status, &private, &storedCode, &maxPlayers, &count, &whitelistCount)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "game_not_found", "Lobby was not found")
		} else {
			gamesdependencies.WriteDatabaseError(context.Writer, err)
		}
		return
	}
	if status != "waiting" {
		_ = tx.Rollback()
		gamesdependencies.WriteError(context.Writer, http.StatusConflict, "game_not_waiting", "This lobby is no longer accepting players")
		return
	}
	var already int
	_ = tx.QueryRow(`SELECT 1 FROM game_participants WHERE game_id=? AND user_id=?`, id, userID).Scan(&already)
	if already != 0 {
		_ = tx.Rollback()
		game, fetchErr := gamesdependencies.FetchGame(database, id, userID)
		if fetchErr != nil {
			gamesdependencies.WriteDatabaseError(context.Writer, fetchErr)
			return
		}
		gamesdependencies.WriteJSON(context.Writer, http.StatusOK, gamesdependencies.GameResponse(game, userID))
		return
	}
	if count >= maxPlayers {
		_ = tx.Rollback()
		gamesdependencies.WriteError(context.Writer, http.StatusConflict, "game_full", "This lobby has reached its player limit")
		return
	}
	if private != 0 {
		if !storedCode.Valid || storedCode.String != code {
			_ = tx.Rollback()
			gamesdependencies.WriteError(context.Writer, http.StatusForbidden, "invalid_code", "The lobby code is invalid")
			return
		}
		if whitelistCount > 0 {
			var allowed int
			_ = tx.QueryRow(`SELECT 1 FROM game_whitelist WHERE game_id=? AND user_id=?`, id, userID).Scan(&allowed)
			if allowed == 0 {
				_ = tx.Rollback()
				gamesdependencies.WriteError(context.Writer, http.StatusNotFound, "not_whitelisted", "User is not on the lobby whitelist")
				return
			}
		}
	}
	if _, err = tx.Exec(`INSERT INTO game_participants(game_id,user_id,last_seen_at) VALUES(?,?,?)`, id, userID, gamesdependencies.NowString()); err != nil {
		_ = tx.Rollback()
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	if err = tx.Commit(); err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	game, err := gamesdependencies.FetchGame(database, id, userID)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	notifier := gamesdependencies.Notify(context)
	notifier.PublishGame(id)
	notifier.PublishLobbies()
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, gamesdependencies.GameResponse(game, userID))
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodPost, "/api/v1/games/{id}/join", route.AccessUser),
}
