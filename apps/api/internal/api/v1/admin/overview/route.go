package overview

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
)

type Route struct {
	route.Route
}

type dependencies interface {
	Database() *sql.DB
}

func Handle(context *route.Context) {
	server, ok := context.Server.(dependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	var overview model.AdminOverview
	err := server.Database().QueryRow(`SELECT (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL),(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND last_login>=strftime('%Y-%m-%dT%H:%M:%fZ','now','-30 days')),(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND role='admin'),(SELECT COALESCE(ROUND(AVG(elo)),0) FROM users WHERE deleted_at IS NULL),(SELECT COALESCE(MAX(elo),0) FROM users WHERE deleted_at IS NULL),(SELECT COUNT(*) FROM games g WHERE EXISTS(SELECT 1 FROM game_participants p WHERE p.game_id=g.id)),(SELECT COUNT(*) FROM games WHERE status='waiting' AND EXISTS(SELECT 1 FROM game_participants p WHERE p.game_id=games.id)),(SELECT COUNT(*) FROM games WHERE status='in_progress' AND EXISTS(SELECT 1 FROM game_participants p WHERE p.game_id=games.id)),(SELECT COUNT(*) FROM games WHERE status='finished' AND EXISTS(SELECT 1 FROM game_participants p WHERE p.game_id=games.id)),(SELECT COALESCE(SUM(games_played),0) FROM user_stats)`).Scan(&overview.TotalUsers, &overview.ActiveUsers, &overview.AdminUsers, &overview.AverageElo, &overview.HighestElo, &overview.TotalGames, &overview.WaitingGames, &overview.ActiveGames, &overview.FinishedGames, &overview.RecordedGames)
	if err != nil {
		writeDatabaseError(context.Writer, err)
		return
	}
	writeJSON(context.Writer, http.StatusOK, overview)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1/admin/overview", route.AccessAdmin),
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{
		"code":      code,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func writeDatabaseError(writer http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "not_found", "Resource was not found")
		return
	}
	slog.Error("database request failed", "error", err)
	writeError(writer, http.StatusInternalServerError, "database_error", "The request could not be completed")
}
