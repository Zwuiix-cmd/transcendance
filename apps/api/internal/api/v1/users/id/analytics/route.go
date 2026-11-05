package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	playeranalytics "card-arena/api/internal/analytics"
	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
)

type Route struct {
	route.Route
}

type analyticsDependencies interface {
	Database() *sql.DB
	Profile(string, bool) (model.User, error)
	CacheGetJSON(context.Context, string, any) bool
	CacheSetJSON(context.Context, string, any, time.Duration)
}

func Handle(context *route.Context) {
	server, ok := context.Server.(analyticsDependencies)
	if !ok {
		route.NotImplemented(context)
		return
	}
	cacheKey := "analytics:user:" + context.Param("id")
	var cached model.UserAnalytics
	if server.CacheGetJSON(context.Request.Context(), cacheKey, &cached) {
		writeJSON(context.Writer, http.StatusOK, cached)
		return
	}
	value, err := analyticsForUser(server, context.Param("id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(context.Writer, http.StatusNotFound, "user_not_found", "User was not found")
		} else {
			writeDatabaseError(context.Writer, err)
		}
		return
	}
	server.CacheSetJSON(context.Request.Context(), cacheKey, value, 5*time.Second)
	writeJSON(context.Writer, http.StatusOK, value)
}

func (Route) Handler() route.Handler {
	return Handle
}

var Definition = Route{
	Route: route.NewRoute(http.MethodGet, "/api/v1/users/{id}/analytics", route.AccessPublic),
}

func analyticsForUser(server analyticsDependencies, id string) (model.UserAnalytics, error) {
	profile, err := server.Profile(id, false)
	if err != nil {
		return model.UserAnalytics{}, err
	}
	rows, err := server.Database().Query(`SELECT id,game_id,previous_elo,new_elo,delta,reason,created_at FROM user_elo_history WHERE user_id=? ORDER BY created_at ASC,id ASC`, id)
	if err != nil {
		return model.UserAnalytics{}, err
	}
	history := []model.EloHistoryPoint{}
	for rows.Next() {
		var point model.EloHistoryPoint
		var gameID sql.NullString
		if err := rows.Scan(&point.ID, &gameID, &point.PreviousElo, &point.NewElo, &point.Delta, &point.Reason, &point.CreatedAt); err != nil {
			return model.UserAnalytics{}, err
		}
		point.GameID = nullString(gameID)
		history = append(history, point)
	}
	if err := rows.Err(); err != nil {
		return model.UserAnalytics{}, err
	}
	_ = rows.Close()
	activityRows, err := server.Database().Query(`SELECT substr(COALESCE(g.ended_at,g.created_at),1,10),COUNT(*),SUM(CASE WHEN p.position=1 THEN 1 ELSE 0 END),SUM(CASE WHEN p.position IS NOT NULL AND p.position<>1 THEN 1 ELSE 0 END),COALESCE(SUM(p.score),0),COALESCE(SUM(COALESCE(NULLIF(p.elo_delta,0),(SELECT SUM(h.delta) FROM user_elo_history h WHERE h.user_id=p.user_id AND h.game_id=p.game_id),0)),0) FROM game_participants p JOIN games g ON g.id=p.game_id WHERE p.user_id=? AND g.status='finished' GROUP BY substr(COALESCE(g.ended_at,g.created_at),1,10) ORDER BY 1 ASC LIMIT 365`, id)
	if err != nil {
		return model.UserAnalytics{}, err
	}
	activity := []model.UserActivityPoint{}
	for activityRows.Next() {
		var point model.UserActivityPoint
		if err := activityRows.Scan(&point.Date, &point.Games, &point.Wins, &point.Losses, &point.Score, &point.EloDelta); err != nil {
			return model.UserAnalytics{}, err
		}
		activity = append(activity, point)
	}
	if err := activityRows.Err(); err != nil {
		return model.UserAnalytics{}, err
	}
	_ = activityRows.Close()
	gameRows, err := server.Database().Query(`SELECT g.id,g.mode,g.status,p.position,p.score,COALESCE(NULLIF(p.elo_delta,0),(SELECT SUM(h.delta) FROM user_elo_history h WHERE h.user_id=p.user_id AND h.game_id=g.id),0),COALESCE(g.ended_at,g.created_at) FROM game_participants p JOIN games g ON g.id=p.game_id WHERE p.user_id=? AND g.status='finished' ORDER BY COALESCE(g.ended_at,g.created_at) DESC LIMIT 50`, id)
	if err != nil {
		return model.UserAnalytics{}, err
	}
	recent := []model.UserGamePoint{}
	for gameRows.Next() {
		var point model.UserGamePoint
		var position sql.NullInt64
		if err := gameRows.Scan(&point.GameID, &point.Mode, &point.Status, &position, &point.Score, &point.EloDelta, &point.PlayedAt); err != nil {
			return model.UserAnalytics{}, err
		}
		if position.Valid {
			value := int(position.Int64)
			point.Position = &value
		}
		recent = append(recent, point)
	}
	if err := gameRows.Err(); err != nil {
		return model.UserAnalytics{}, err
	}
	_ = gameRows.Close()
	var rank, total int
	if err := server.Database().QueryRow(`SELECT (SELECT COUNT(*)+1 FROM users higher WHERE higher.deleted_at IS NULL AND higher.elo>target.elo),(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL) FROM users target WHERE target.id=? AND target.deleted_at IS NULL`, id).Scan(&rank, &total); err != nil {
		return model.UserAnalytics{}, err
	}
	average := float64(profile.Elo)
	peak := profile.Elo
	lowest := profile.Elo
	net := 0
	positive := 0
	negative := 0
	if len(history) > 0 {
		sum := 0
		peak = history[0].NewElo
		lowest = history[0].NewElo
		for _, point := range history {
			sum += point.NewElo
			if point.NewElo > peak {
				peak = point.NewElo
			}
			if point.NewElo < lowest {
				lowest = point.NewElo
			}
			net += point.Delta
			if point.Delta > 0 {
				positive++
			}
			if point.Delta < 0 {
				negative++
			}
		}
		average = float64(sum) / float64(len(history))
	}
	averageChange := 0.0
	if len(history) > 0 {
		averageChange = float64(net) / float64(len(history))
	}
	variance := 0.0
	if len(history) > 0 {
		for _, point := range history {
			variance += math.Pow(float64(point.Delta)-averageChange, 2)
		}
		variance /= float64(len(history))
	}
	percentile := 0.0
	if total > 0 {
		percentile = float64(total-rank+1) / float64(total) * 100
	}
	score, totalEloDelta := 0, 0
	for _, point := range activity {
		score += point.Score
		totalEloDelta += point.EloDelta
	}
	winRate := 0.0
	if profile.Stats.GamesPlayed > 0 {
		winRate = float64(profile.Stats.GamesWon) / float64(profile.Stats.GamesPlayed)
	}
	summary := model.UserAnalyticsSummary{CurrentElo: profile.Elo, AverageElo: average, PeakElo: peak, LowestElo: lowest, NetEloDelta: net, Volatility: math.Sqrt(variance), AverageEloDelta: averageChange, PositiveEloDeltas: positive, NegativeEloDeltas: negative, EloEvents: len(history), Rank: rank, TotalPlayers: total, Percentile: percentile, GamesPlayed: profile.Stats.GamesPlayed, GamesWon: profile.Stats.GamesWon, WinRate: winRate, CurrentStreak: profile.Stats.CurrentStreak, MaxStreak: profile.Stats.MaxStreak, UnosCalled: profile.Stats.UnosCalled, UnoPenalties: profile.Stats.UnoPenalties, DrawFoursPlayed: profile.Stats.DrawFoursPlayed, CardsDrawn: profile.Stats.CardsDrawn, TotalScore: score, TotalEloDelta: totalEloDelta}
	visibleActivity := playeranalytics.RecentActivity(activity)
	return model.UserAnalytics{Summary: summary, EloHistory: history, Activity: activity, RecentGames: recent, ActivityMetrics: playeranalytics.SummarizeActivity(activity), EloChart: playeranalytics.PrepareEloChart(history), ActivityChart: playeranalytics.PreparePlayerActivityChart(visibleActivity)}, nil
}

func nullString(value sql.NullString) *string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	valueCopy := value.String
	return &valueCopy
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{"code": code, "message": message, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}

func writeDatabaseError(writer http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(writer, http.StatusNotFound, "not_found", "Resource was not found")
		return
	}
	slog.Error("database request failed", "error", err)
	writeError(writer, http.StatusInternalServerError, "database_error", "The request could not be completed")
}
