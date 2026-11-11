package api

import (
	"log/slog"

	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
	"card-arena/api/internal/model"
	"card-arena/api/internal/realtime"
	unogame "card-arena/api/internal/uno"
)

// PublishGame pushes the lobby view of a game to every subscriber. The view
// is rendered per viewer so that only the host receives the whitelist and the
// invite code.
func (server *Server) PublishGame(gameID string) {
	server.hub.Publish(realtime.Event{Topic: realtime.GameTopic(gameID), Name: realtime.EventGameUpdated, Render: func(viewerID string) any {
		if viewerID == "" {
			return nil
		}
		detail, err := gamesdependencies.FetchGameDetail(server.store.DB, gameID, viewerID)
		if err != nil {
			return nil
		}
		return detail
	}})
}

func (server *Server) PublishGameDeleted(gameID string) {
	server.hub.Publish(realtime.Event{Topic: realtime.GameTopic(gameID), Name: realtime.EventGameDeleted, Payload: map[string]string{"gameId": gameID}})
}

func (server *Server) PublishLobbies() {
	server.hub.Publish(realtime.Event{Topic: realtime.TopicLobbies, Name: realtime.EventLobbiesChanged})
}

func (server *Server) PublishLeaderboard() {
	server.hub.Publish(realtime.Event{Topic: realtime.TopicLeaderboard, Name: realtime.EventLeaderboardChanged})
}

func (server *Server) PublishProfile(userID string) {
	server.hub.Publish(realtime.Event{Topic: realtime.ProfileTopic(userID), Name: realtime.EventProfileChanged, Payload: map[string]string{"userId": userID}})
}

func (server *Server) publishMatchmaking(userID string, status model.MatchmakingStatus) {
	server.hub.Publish(realtime.Event{Topic: realtime.UserTopic(userID), Name: realtime.EventMatchmakingUpdated, Payload: status})
}

func (server *Server) publishUnoUpdate(update unogame.Update) {
	server.hub.Publish(realtime.Event{Topic: realtime.GameTopic(update.GameID), Name: realtime.EventUnoState, Render: func(viewerID string) any {
		if !gamesdependencies.HasGameAccess(server.store.DB, update.GameID, viewerID) {
			return nil
		}
		return update.Snapshot(viewerID)
	}})
	if update.Status == "finished" {
		server.PublishGame(update.GameID)
		server.PublishLobbies()
		slog.Info("uno game finished", "game", update.GameID, "winner", update.WinnerID)
	}
}
