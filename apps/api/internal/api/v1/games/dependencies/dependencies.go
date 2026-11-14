package gamesdependencies

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	route "card-arena/api/internal/api/route"
	"card-arena/api/internal/model"
	unogame "card-arena/api/internal/uno"
	"github.com/google/uuid"
)

type Server interface {
	Database() *sql.DB
	UserID(any) (string, bool)
}

type UnoServer interface {
	Server
	UnoGame() *unogame.Engine
}

func GetUnoServer(context *route.Context) (UnoServer, bool) {
	server, ok := context.Server.(UnoServer)
	if !ok || server.UnoGame() == nil {
		route.NotImplemented(context)
		return nil, false
	}
	return server, true
}

func RequireUnoUser(context *route.Context) (UnoServer, string, bool) {
	server, ok := GetUnoServer(context)
	if !ok {
		return nil, "", false
	}
	id, valid := server.UserID(context.User)
	if !valid {
		WriteError(context.Writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return nil, "", false
	}
	return server, id, true
}

func GetServer(context *route.Context) (Server, bool) {
	server, ok := context.Server.(Server)
	if !ok {
		route.NotImplemented(context)
		return nil, false
	}
	return server, true
}

func RequireUser(context *route.Context) (Server, string, int, bool) {
	server, ok := GetServer(context)
	if !ok {
		return nil, "", 0, false
	}
	id, valid := server.UserID(context.User)
	if !valid {
		WriteError(context.Writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return nil, "", 0, false
	}
	return server, id, 0, true
}

func OptionalUser(context *route.Context) (Server, string, bool) {
	server, ok := GetServer(context)
	if !ok {
		return nil, "", false
	}
	if context.User == nil {
		return server, "", true
	}
	id, valid := server.UserID(context.User)
	if !valid {
		WriteError(context.Writer, http.StatusUnauthorized, "authentication_required", "Authentication is required")
		return nil, "", false
	}
	return server, id, true
}

type SQLScanner interface {
	Scan(...any) error
}

type ScannedGame struct {
	ID               string
	Status           string
	CreatedBy        sql.NullString
	CreatorUsername  sql.NullString
	CreatedAt        string
	StartedAt        sql.NullString
	EndedAt          sql.NullString
	TurnTimeLimit    int
	Mode             string
	PrivateGame      int
	JoinCode         sql.NullString
	Settings         model.GameSettings
	ParticipantCount int
	Joined           int
	Spectating       int
	SpectatorCount   int
	WhitelistCount   int
}

func ScanGame(scanner SQLScanner) (ScannedGame, error) {
	var game ScannedGame
	var stack, jump, seven, draw, force, challenge, multiple, bots int
	err := scanner.Scan(&game.ID, &game.Status, &game.CreatedBy, &game.CreatorUsername, &game.CreatedAt, &game.StartedAt, &game.EndedAt, &game.TurnTimeLimit, &game.Mode, &game.PrivateGame, &game.JoinCode, &game.Settings.MaxPlayers, &game.Settings.StartingHandSize, &game.Settings.DrawPenalty, &stack, &jump, &seven, &draw, &force, &challenge, &multiple, &bots, &game.Settings.BotDifficulty, &game.ParticipantCount, &game.Joined, &game.Spectating, &game.SpectatorCount, &game.WhitelistCount)
	game.Settings.StackDrawCards = stack != 0
	game.Settings.JumpIn = jump != 0
	game.Settings.SevenZero = seven != 0
	game.Settings.DrawUntilPlayable = draw != 0
	game.Settings.ForcePlay = force != 0
	game.Settings.ChallengeDrawFour = challenge != 0
	game.Settings.AllowMultipleDecks = multiple != 0
	game.Settings.BotsEnabled = bots != 0
	return game, err
}

func FetchGame(database *sql.DB, id, viewerID string) (ScannedGame, error) {
	query := `SELECT g.id,g.status,g.created_by,u.username,g.created_at,g.started_at,g.ended_at,g.turn_time_limit,g.mode,g.is_private,g.join_code,COALESCE(gs.max_players,4),COALESCE(gs.starting_hand_size,7),COALESCE(gs.draw_penalty,2),COALESCE(gs.stack_draw_cards,0),COALESCE(gs.jump_in,0),COALESCE(gs.seven_zero,0),COALESCE(gs.draw_until_playable,0),COALESCE(gs.force_play,0),COALESCE(gs.challenge_draw_four,1),COALESCE(gs.allow_multiple_decks,0),COALESCE(gs.bots_enabled,0),COALESCE(gs.bot_difficulty,'intermediate'),(SELECT COUNT(*) FROM game_participants p WHERE p.game_id=g.id),EXISTS(SELECT 1 FROM game_participants p2 WHERE p2.game_id=g.id AND p2.user_id=?),(SELECT EXISTS(SELECT 1 FROM game_spectators s2 WHERE s2.game_id=g.id AND s2.user_id=?)),(SELECT COUNT(*) FROM game_spectators s WHERE s.game_id=g.id),(SELECT COUNT(*) FROM game_whitelist w WHERE w.game_id=g.id) FROM games g LEFT JOIN users u ON u.id=g.created_by LEFT JOIN game_settings gs ON gs.game_id=g.id WHERE g.id=?`
	return ScanGame(database.QueryRow(query, viewerID, viewerID, id))
}

func GameResponse(game ScannedGame, viewerID string) model.Game {
	host := viewerID != "" && game.CreatedBy.Valid && game.CreatedBy.String == viewerID
	viewerRole := "observer"
	if host || game.Joined != 0 {
		viewerRole = "player"
	} else if game.Spectating != 0 {
		viewerRole = "spectator"
	}
	var joinCode *string
	if host && game.JoinCode.Valid {
		value := game.JoinCode.String
		joinCode = &value
	}
	return model.Game{ID: game.ID, Status: game.Status, CreatedBy: NullString(game.CreatedBy), CreatorUsername: NullString(game.CreatorUsername), CreatedAt: game.CreatedAt, StartedAt: NullString(game.StartedAt), EndedAt: NullString(game.EndedAt), TurnTimeLimit: game.TurnTimeLimit, Mode: game.Mode, ParticipantCount: game.ParticipantCount, PrivateGame: game.PrivateGame != 0, JoinCode: joinCode, Host: host, Joined: game.Joined != 0, Spectating: game.Spectating != 0, ViewerRole: viewerRole, SpectatorCount: game.SpectatorCount, WhitelistCount: game.WhitelistCount, Settings: game.Settings}
}

func NullString(value sql.NullString) *string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	copy := value.String
	return &copy
}

func DecodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		WriteError(writer, http.StatusBadRequest, "invalid_json", "Request body is invalid")
		return false
	}
	return true
}

func WriteJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func WriteError(writer http.ResponseWriter, status int, code, message string) {
	WriteJSON(writer, status, map[string]any{"code": code, "message": message, "timestamp": NowString()})
}

func WriteDatabaseError(writer http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		WriteError(writer, http.StatusNotFound, "not_found", "Resource was not found")
		return
	}
	WriteError(writer, http.StatusInternalServerError, "database_error", "The request could not be completed")
}

func WriteUnoError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		WriteError(writer, http.StatusNotFound, "game_not_found", "Game was not found")
	case errors.Is(err, unogame.ErrNotOwner):
		WriteError(writer, http.StatusForbidden, "not_game_owner", "Only the game owner can start this game")
	case errors.Is(err, unogame.ErrNotEnoughPlayers):
		WriteError(writer, http.StatusConflict, "not_enough_players", "At least two players are required when bots are disabled")
	case errors.Is(err, unogame.ErrNotYourTurn):
		WriteError(writer, http.StatusConflict, "not_your_turn", "It is not your turn")
	case errors.Is(err, unogame.ErrNotAPlayer):
		WriteError(writer, http.StatusForbidden, "not_a_player", "You are not seated at this table")
	case errors.Is(err, unogame.ErrInvalidCard):
		WriteError(writer, http.StatusConflict, "invalid_card", unoMessage(err, "This card cannot be played"))
	case errors.Is(err, unogame.ErrInvalidAction):
		WriteError(writer, http.StatusConflict, "invalid_action", unoMessage(err, "This UNO action is not allowed"))
	case errors.Is(err, unogame.ErrGameNotActive):
		WriteError(writer, http.StatusConflict, "game_not_active", "This game is not active")
	default:
		WriteDatabaseError(writer, err)
	}
}

func unoMessage(err error, fallback string) string {
	message := err.Error()
	if index := strings.Index(message, ": "); index >= 0 && index+2 < len(message) {
		detail := message[index+2:]
		return strings.ToUpper(detail[:1]) + detail[1:]
	}
	return fallback
}

func IsConstraint(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "constraint") || strings.Contains(message, "unique")
}

func NowString() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func GenerateJoinCode() string {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:6]
	}
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for index := range raw {
		raw[index] = alphabet[int(raw[index])%len(alphabet)]
	}
	return string(raw)
}

func Pagination(request *http.Request, defaultLimit, maxLimit int) (int, int, int) {
	limit := QueryInt(request, "limit", defaultLimit)
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	page := QueryInt(request, "page", 1)
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	return limit, (page - 1) * limit, page
}

func QueryInt(request *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(request.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func SetPaginationHeaders(writer http.ResponseWriter, page, limit int, hasMore bool) {
	writer.Header().Set("X-Pagination-Page", strconv.Itoa(page))
	writer.Header().Set("X-Pagination-Limit", strconv.Itoa(limit))
	writer.Header().Set("X-Pagination-Has-More", strconv.FormatBool(hasMore))
}
