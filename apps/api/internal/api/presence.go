package api

import (
	"strings"
	"time"
)

const presenceTouchInterval = 30 * time.Second

// userConnected refreshes the last-seen marker of every lobby the user sits
// in, so that a lobby is never garbage-collected while its host is online.
func (server *Server) userConnected(userID string) {
	if userID == "" {
		return
	}
	_, _ = server.store.DB.Exec(`UPDATE game_participants SET last_seen_at=? WHERE user_id=?`, nowString(), userID)
}

func (server *Server) userDisconnected(userID string) {
	server.userConnected(userID)
}

func (server *Server) gameSubscribed(gameID, userID string) {
	if userID == "" {
		return
	}
	now := nowString()
	_, _ = server.store.DB.Exec(`UPDATE game_participants SET last_seen_at=? WHERE game_id=? AND user_id=?`, now, gameID, userID)
	_, _ = server.store.DB.Exec(`UPDATE game_spectators SET last_seen_at=? WHERE game_id=? AND user_id=?`, now, gameID, userID)
}

// gameUnsubscribed runs when the user's last connection stops following a
// game. A spectator who stopped watching leaves the spectator list at once.
func (server *Server) gameUnsubscribed(gameID, userID string) {
	if userID == "" || server.hub.SubscriptionCount("game:"+gameID, userID) > 0 {
		return
	}
	_, _ = server.store.DB.Exec(`UPDATE game_participants SET last_seen_at=? WHERE game_id=? AND user_id=?`, nowString(), gameID, userID)
	result, err := server.store.DB.Exec(`DELETE FROM game_spectators WHERE game_id=? AND user_id=?`, gameID, userID)
	if err != nil {
		return
	}
	if affected, _ := result.RowsAffected(); affected > 0 {
		server.PublishGame(gameID)
		server.PublishLobbies()
	}
}

// touchPresence keeps the rows of every connected user fresh while their
// socket stays open.
func (server *Server) touchPresence() {
	users := server.hub.ConnectedUsers()
	if len(users) == 0 {
		return
	}
	now := nowString()
	for start := 0; start < len(users); start += 200 {
		end := start + 200
		if end > len(users) {
			end = len(users)
		}
		chunk := users[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		args := make([]any, 0, len(chunk)+1)
		args = append(args, now)
		for _, id := range chunk {
			args = append(args, id)
		}
		_, _ = server.store.DB.Exec(`UPDATE game_participants SET last_seen_at=? WHERE user_id IN (`+placeholders+`)`, args...)
	}
	for topic, viewers := range server.hub.Subscriptions("game:") {
		gameID := strings.TrimPrefix(topic, "game:")
		for _, viewer := range viewers {
			_, _ = server.store.DB.Exec(`UPDATE game_spectators SET last_seen_at=? WHERE game_id=? AND user_id=?`, now, gameID, viewer)
		}
	}
}

// runMaintenance removes abandoned lobbies and stale spectators, and keeps
// presence markers fresh for connected users.
func (server *Server) runMaintenance() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	defer close(server.maintenanceDone)
	lastTouch := time.Now()
	for {
		select {
		case <-ticker.C:
			server.cleanupLobbies()
			server.cleanupSpectators()
			if time.Since(lastTouch) >= presenceTouchInterval {
				server.touchPresence()
				lastTouch = time.Now()
			}
		case <-server.maintenanceStop:
			return
		}
	}
}

func (server *Server) cleanupLobbies() {
	cutoff := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	rows, err := server.store.DB.Query(`SELECT id FROM games WHERE status='waiting' AND (created_by IS NULL OR NOT EXISTS (SELECT 1 FROM game_participants p WHERE p.game_id=games.id AND p.user_id=games.created_by AND COALESCE(p.last_seen_at,games.created_at)>?))`, cutoff)
	if err != nil {
		return
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if len(ids) == 0 {
		return
	}
	for _, id := range ids {
		if _, err := server.store.DB.Exec(`DELETE FROM games WHERE id=? AND status='waiting'`, id); err == nil {
			server.PublishGameDeleted(id)
		}
	}
	server.PublishLobbies()
}

func (server *Server) cleanupSpectators() {
	cutoff := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	rows, err := server.store.DB.Query(`SELECT DISTINCT game_id FROM game_spectators WHERE last_seen_at<?`, cutoff)
	if err != nil {
		return
	}
	games := make([]string, 0)
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			games = append(games, id)
		}
	}
	rows.Close()
	if len(games) == 0 {
		return
	}
	if _, err := server.store.DB.Exec(`DELETE FROM game_spectators WHERE last_seen_at<?`, cutoff); err != nil {
		return
	}
	for _, id := range games {
		server.PublishGame(id)
	}
	server.PublishLobbies()
}
