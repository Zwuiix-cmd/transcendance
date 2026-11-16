package api

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"card-arena/api/internal/matchmaking"
	"card-arena/api/internal/model"
	"fmt"
	"github.com/google/uuid"
)

// Start a ranked / Unranked Game
func (server *Server) createMatchGame(
	mode string,
	players []matchmaking.MatchPlayer,
) (string, error) {

	if mode != "ranked" && mode != "unranked" {
		return "", fmt.Errorf("invalid matchmaking mode: %s", mode)
	}

	if len(players) < 2 || len(players) > 4 {
		return "", fmt.Errorf("invalid player count: %d", len(players))
	}

	gameID := uuid.NewString()
	ownerID := players[0].UserID
	created := time.Now().UTC().Format(time.RFC3339Nano)

	settings := model.DefaultGameSettings()

	tx, err := server.store.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
        INSERT INTO games (
            id,
            status,
            created_by,
            turn_time_limit,
            mode,
            is_private,
            join_code,
            created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `,
		gameID,
		"waiting",
		ownerID,
		15,
		mode,
		0,
		nil,
		created,
	)

	if err != nil {
		return "", err
	}

	_, err = tx.Exec(`
        INSERT INTO game_settings (
            game_id,
            max_players,
            starting_hand_size,
            draw_penalty,
            stack_draw_cards,
            jump_in,
            seven_zero,
            draw_until_playable,
            force_play,
            challenge_draw_four,
            allow_multiple_decks,
            bots_enabled
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `,
		gameID,
		settings.MaxPlayers,
		settings.StartingHandSize,
		settings.DrawPenalty,
		boolToInt(settings.StackDrawCards),
		boolToInt(settings.JumpIn),
		boolToInt(settings.SevenZero),
		boolToInt(settings.DrawUntilPlayable),
		boolToInt(settings.ForcePlay),
		boolToInt(settings.ChallengeDrawFour),
		boolToInt(settings.AllowMultipleDecks),
		boolToInt(settings.BotsEnabled),
	)

	if err != nil {
		return "", err
	}

	for _, player := range players {
		_, err = tx.Exec(`
            INSERT INTO game_participants (
                game_id,
                user_id,
                last_seen_at
            ) VALUES (?, ?, ?)
        `,
			gameID,
			player.UserID,
			created,
		)

		if err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	_, err = server.uno.Start(gameID, ownerID)
	if err != nil {
		_, _ = server.store.DB.Exec(
			`DELETE FROM games WHERE id = ?`,
			gameID,
		)
		return "", err
	}

	server.PublishGame(gameID)
	server.PublishLobbies()

	return gameID, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// Database exposes the API database to route-owned handlers.
func (server *Server) Database() *sql.DB {
	return server.store.DB
}

// Profile loads a user profile for a route-owned handler.
func (server *Server) Profile(id string, includeEmail bool) (model.User, error) {
	return server.profile(id, includeEmail)
}

// IssueSession creates a session for a route-owned handler.
func (server *Server) IssueSession(user model.User, request *http.Request) (model.AuthResponse, error) {
	return server.issueSession(user, request)
}

// Analytics exposes the domain computation to route-owned analytics endpoints.
func (server *Server) Analytics(id string) (model.UserAnalytics, error) {
	return server.analytics(id)
}

// AvatarURL resolves the public avatar URL used by route-owned listings.
func (server *Server) AvatarURL(id string, current *string) *string {
	return server.avatarURL(id, current)
}

// ProfileWithAvatar loads a profile and resolves its public avatar URL.
func (server *Server) ProfileWithAvatar(id string, includeEmail bool) (model.User, error) {
	return server.profileWithAvatar(id, includeEmail)
}

// UserID resolves the opaque authenticated user value exposed to route packages.
func (server *Server) UserID(value any) (string, bool) {
	user, ok := value.(*authUser)
	if !ok || user == nil {
		return "", false
	}
	return user.ID, true
}

// UserElo resolves the rating needed to create a matchmaking ticket.
func (server *Server) UserElo(value any) (int, bool) {
	user, ok := value.(*authUser)
	if !ok || user == nil {
		return 0, false
	}
	return user.Elo, true
}

// RouteQueue exposes the matchmaking service to the queue route.
func (server *Server) RouteQueue() *matchmaking.Queue {
	return server.queue
}

// CDNURL returns the internal CDN endpoint used by the avatar route.
func (server *Server) CDNURL() string {
	return server.cfg.CDNURL
}

// InternalCDNAPIKey returns the credential used for API-to-CDN avatar uploads.
func (server *Server) InternalCDNAPIKey() string {
	return server.cfg.InternalCDNAPIKey
}

// HTTPClient returns the server's configured outbound HTTP client.
func (server *Server) HTTPClient() *http.Client {
	return server.client
}

// AdminActor returns the identity established by the admin middleware.
func (server *Server) AdminActor(value any) (id, username string, ok bool) {
	user, ok := value.(*authUser)
	if !ok || user == nil {
		return "", "", false
	}
	return user.ID, user.Username, true
}

// RecordAudit lets route-owned mutation handlers preserve the server audit trail.
func (server *Server) RecordAudit(actorID, actorUsername, action, targetType, targetID, details, severity, ipAddress, userAgent string) {
	server.recordAudit(actorID, actorUsername, action, targetType, targetID, details, severity, ipAddress, userAgent)
}

// ClientKey resolves the request origin used in audit records.
func (server *Server) ClientKey(request *http.Request) string {
	return server.clientKey(request)
}

// CacheGetJSON and CacheSetJSON reuse the server's configured cache connection.
func (server *Server) CacheGetJSON(ctx context.Context, key string, target any) bool {
	return server.cache.GetJSON(ctx, key, target)
}

func (server *Server) CacheSetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	server.cache.SetJSON(ctx, key, value, ttl)
}
