package gamesdependencies

import (
	"database/sql"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
)

// Notifier pushes lobby changes to connected clients. The API server
// implements it; tests and tools that lack a realtime hub get a no-op.
type Notifier interface {
	PublishGame(gameID string)
	PublishGameDeleted(gameID string)
	PublishLobbies()
}

type noopNotifier struct{}

func (noopNotifier) PublishGame(string)        {}
func (noopNotifier) PublishGameDeleted(string) {}
func (noopNotifier) PublishLobbies()           {}

func Notify(context *route.Context) Notifier {
	if notifier, ok := context.Server.(Notifier); ok {
		return notifier
	}
	return noopNotifier{}
}

// FetchGameDetail loads the lobby view of a game as seen by viewerID. The
// whitelist is only exposed to the host.
func FetchGameDetail(database *sql.DB, id, viewerID string) (model.GameDetail, error) {
	game, err := FetchGame(database, id, viewerID)
	if err != nil {
		return model.GameDetail{}, err
	}
	participants := []model.LobbyParticipant{}
	rows, err := database.Query(`SELECT p.user_id,u.username,CASE WHEN g.created_by=p.user_id THEN 1 ELSE 0 END FROM game_participants p JOIN users u ON u.id=p.user_id JOIN games g ON g.id=p.game_id WHERE p.game_id=? ORDER BY CASE WHEN g.created_by=p.user_id THEN 0 ELSE 1 END ASC,u.username ASC`, id)
	if err != nil {
		return model.GameDetail{}, err
	}
	for rows.Next() {
		var participant model.LobbyParticipant
		var host int
		if err := rows.Scan(&participant.UserID, &participant.Username, &host); err != nil {
			rows.Close()
			return model.GameDetail{}, err
		}
		participant.Host = host != 0
		participants = append(participants, participant)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return model.GameDetail{}, err
	}
	rows.Close()
	spectators := []model.LobbySpectator{}
	rows, err = database.Query(`SELECT s.user_id,u.username FROM game_spectators s JOIN users u ON u.id=s.user_id WHERE s.game_id=? ORDER BY u.username ASC`, id)
	if err != nil {
		return model.GameDetail{}, err
	}
	for rows.Next() {
		var spectator model.LobbySpectator
		if err := rows.Scan(&spectator.UserID, &spectator.Username); err != nil {
			rows.Close()
			return model.GameDetail{}, err
		}
		spectators = append(spectators, spectator)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return model.GameDetail{}, err
	}
	rows.Close()
	whitelist := []model.WhitelistEntry{}
	if game.CreatedBy.Valid && game.CreatedBy.String == viewerID {
		rows, err = database.Query(`SELECT w.user_id,u.username FROM game_whitelist w JOIN users u ON u.id=w.user_id WHERE w.game_id=? ORDER BY u.username ASC`, id)
		if err != nil {
			return model.GameDetail{}, err
		}
		for rows.Next() {
			var item model.WhitelistEntry
			if err := rows.Scan(&item.UserID, &item.Username); err != nil {
				rows.Close()
				return model.GameDetail{}, err
			}
			whitelist = append(whitelist, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return model.GameDetail{}, err
		}
		rows.Close()
	}
	return model.GameDetail{Game: GameResponse(game, viewerID), Participants: participants, Spectators: spectators, Whitelist: whitelist}, nil
}

// HasGameAccess reports whether viewerID is a player or a spectator of the game.
func HasGameAccess(database *sql.DB, gameID, viewerID string) bool {
	if viewerID == "" {
		return false
	}
	var allowed int
	if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM game_participants WHERE game_id=? AND user_id=?) OR EXISTS(SELECT 1 FROM game_spectators WHERE game_id=? AND user_id=?)`, gameID, viewerID, gameID, viewerID).Scan(&allowed); err != nil {
		return false
	}
	return allowed != 0
}
