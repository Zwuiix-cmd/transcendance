package api

import (
	"log/slog"

	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
	"card-arena/api/internal/model"
	"card-arena/api/internal/realtime"
	unogame "card-arena/api/internal/uno"
	"time"

	"card-arena/api/internal/settlement"
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
    // End of game
    if update.Status == "finished" && len(update.Results) >= 2 {
        participants := make([]settlement.Participant, 0, len(update.Results))

        for _, result := range update.Results {
            participants = append(participants, settlement.Participant{
                UserID:          result.PlayerID,
                Position:        result.Rank,
                Score:           result.PointsEarned,
                UnosCalled:      result.UnosCalled,
                UnoPenalties:    result.UnoPenalties,
                DrawFoursPlayed: result.DrawFourPlayed,
                CardsDrawn:      result.CardsDrawn,
            })
        }

        tx, err := server.store.DB.Begin()
        if err != nil {
            slog.Error("could not begin game settlement",
                "game", update.GameID,
                "error", err,
            )
        } else {
            outcome, err := settlement.Apply(
                tx,
                settlement.Result{
                    GameID:       update.GameID,
                    Participants: participants,
                },
                time.Now().UTC(),
            )

            if err != nil {
                _ = tx.Rollback()

                slog.Error("could not settle game",
                    "game", update.GameID,
                    "error", err,
                )
            } else if err := tx.Commit(); err != nil {
                slog.Error("could not commit game settlement",
                    "game", update.GameID,
                    "error", err,
                )
            } else if !outcome.AlreadySettled {
                server.PublishLeaderboard()

                for _, participant := range outcome.Participants {
                    server.PublishProfile(participant.UserID)
                }
            }
        }
    }

    // Broadcast the state to the players
    server.hub.Publish(realtime.Event{
        Topic: realtime.GameTopic(update.GameID),
        Name:  realtime.EventUnoState,
        Render: func(viewerID string) any {
            if !gamesdependencies.HasGameAccess(
                server.store.DB,
                update.GameID,
                viewerID,
            ) {
                return nil
            }

            return update.Snapshot(viewerID)
        },
    })

    if update.Status == "finished" {
        server.PublishGame(update.GameID)
        server.PublishLobbies()

        slog.Info("uno game finished",
            "game", update.GameID,
            "winner", update.WinnerID,
        )
    }
}