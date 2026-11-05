package games

import (
	"net/http"
	"regexp"
	"strings"

	route "card-arena/api/internal/api/route"
	gamesdependencies "card-arena/api/internal/api/v1/games/dependencies"
	"card-arena/api/internal/model"
	"github.com/google/uuid"
)

type ListRoute struct {
	route.Route
}

func HandleList(context *route.Context) {
	server, viewerID, ok := gamesdependencies.OptionalUser(context)
	if !ok {
		return
	}

	status := strings.TrimSpace(context.Request.URL.Query().Get("status"))
	if status != "" && !gameStatusPattern.MatchString(status) {
		gamesdependencies.WriteError(context.Writer, http.StatusBadRequest, "invalid_status", "Unknown game status")
		return
	}
	limit, offset, page := gamesdependencies.Pagination(context.Request, 20, 100)
	query := `SELECT g.id,g.status,g.created_by,u.username,g.created_at,g.started_at,g.ended_at,g.turn_time_limit,g.mode,g.is_private,g.join_code,COALESCE(gs.max_players,4),COALESCE(gs.starting_hand_size,7),COALESCE(gs.draw_penalty,2),COALESCE(gs.stack_draw_cards,0),COALESCE(gs.jump_in,0),COALESCE(gs.seven_zero,0),COALESCE(gs.draw_until_playable,0),COALESCE(gs.force_play,0),COALESCE(gs.challenge_draw_four,1),COALESCE(gs.allow_multiple_decks,0),COALESCE(gs.bots_enabled,0),(SELECT COUNT(*) FROM game_participants p WHERE p.game_id=g.id),EXISTS(SELECT 1 FROM game_participants p2 WHERE p2.game_id=g.id AND p2.user_id=?),(SELECT EXISTS(SELECT 1 FROM game_spectators s2 WHERE s2.game_id=g.id AND s2.user_id=?)),(SELECT COUNT(*) FROM game_spectators s WHERE s.game_id=g.id),(SELECT COUNT(*) FROM game_whitelist w WHERE w.game_id=g.id) FROM games g LEFT JOIN users u ON u.id=g.created_by LEFT JOIN game_settings gs ON gs.game_id=g.id WHERE g.mode='custom'`
	if status != "" {
		query += ` AND g.status=?`
	}
	query += ` ORDER BY g.created_at DESC LIMIT ? OFFSET ?`
	args := []any{viewerID, viewerID}
	if status != "" {
		args = append(args, status)
	}
	args = append(args, limit+1, offset)
	rows, err := server.Database().Query(query, args...)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	defer rows.Close()
	games := []model.Game{}
	for rows.Next() {
		game, scanErr := gamesdependencies.ScanGame(rows)
		if scanErr != nil {
			gamesdependencies.WriteDatabaseError(context.Writer, scanErr)
			return
		}
		games = append(games, gamesdependencies.GameResponse(game, viewerID))
	}
	if err = rows.Err(); err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	hasMore := len(games) > limit
	if hasMore {
		games = games[:limit]
	}
	gamesdependencies.SetPaginationHeaders(context.Writer, page, limit, hasMore)
	gamesdependencies.WriteJSON(context.Writer, http.StatusOK, games)
}

func (ListRoute) Handler() route.Handler {
	return HandleList
}

var ListDefinition = ListRoute{
	Route: route.NewRoute(http.MethodGet, "/api/v1/games", route.AccessOptionalUser),
}

type CreateRoute struct {
	route.Route
}

func HandleCreate(context *route.Context) {
	server, userID, _, ok := gamesdependencies.RequireUser(context)
	if !ok {
		return
	}
	var input createGameRequest
	if !gamesdependencies.DecodeJSON(context.Writer, context.Request, &input) {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	if mode == "" {
		mode = "custom"
	}
	if mode != "custom" {
		gamesdependencies.WriteError(context.Writer, http.StatusBadRequest, "matchmaking_required", "Ranked and unranked games must use matchmaking")
		return
	}
	turn := 15
	if input.TurnTimeLimit != nil {
		turn = *input.TurnTimeLimit
	}
	if turn < 5 || turn > 120 {
		gamesdependencies.WriteError(context.Writer, http.StatusBadRequest, "invalid_turn_time", "Turn time limit must be between 5 and 120 seconds")
		return
	}
	settings := model.DefaultGameSettings()
	if input.MaxPlayers != nil {
		settings.MaxPlayers = *input.MaxPlayers
	}
	if input.StartingHandSize != nil {
		settings.StartingHandSize = *input.StartingHandSize
	}
	if input.DrawPenalty != nil {
		settings.DrawPenalty = *input.DrawPenalty
	}
	settings.StackDrawCards = input.StackDrawCards
	settings.JumpIn = input.JumpIn
	settings.SevenZero = input.SevenZero
	settings.DrawUntilPlayable = input.DrawUntilPlayable
	settings.ForcePlay = input.ForcePlay
	if input.ChallengeDrawFour != nil {
		settings.ChallengeDrawFour = *input.ChallengeDrawFour
	}
	settings.AllowMultipleDecks = input.AllowMultipleDecks
	settings.BotsEnabled = input.BotsEnabled
	if settings.MaxPlayers < 2 || settings.MaxPlayers > 8 || settings.StartingHandSize < 3 || settings.StartingHandSize > 14 || settings.DrawPenalty < 1 || settings.DrawPenalty > 4 {
		gamesdependencies.WriteError(context.Writer, http.StatusBadRequest, "invalid_settings", "The game settings are outside their allowed range")
		return
	}
	id := uuid.NewString()
	created := gamesdependencies.NowString()
	code := ""
	if input.PrivateGame {
		code = gamesdependencies.GenerateJoinCode()
	}
	tx, err := server.Database().Begin()
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	var activeGames int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM games WHERE created_by=? AND status IN ('waiting','in_progress')`, userID).Scan(&activeGames); err != nil {
		_ = tx.Rollback()
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	if activeGames > 0 {
		_ = tx.Rollback()
		gamesdependencies.WriteError(context.Writer, http.StatusConflict, "active_game_exists", "You already have an active custom game")
		return
	}
	_, err = tx.Exec(`INSERT INTO games(id,status,created_by,turn_time_limit,mode,is_private,join_code,created_at) VALUES(?,?,?,?,?,?,?,?)`, id, "waiting", userID, turn, mode, boolInt(input.PrivateGame), blankValue(code), created)
	if err == nil {
		_, err = tx.Exec(`INSERT INTO game_settings(game_id,max_players,starting_hand_size,draw_penalty,stack_draw_cards,jump_in,seven_zero,draw_until_playable,force_play,challenge_draw_four,allow_multiple_decks,bots_enabled) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, id, settings.MaxPlayers, settings.StartingHandSize, settings.DrawPenalty, boolInt(settings.StackDrawCards), boolInt(settings.JumpIn), boolInt(settings.SevenZero), boolInt(settings.DrawUntilPlayable), boolInt(settings.ForcePlay), boolInt(settings.ChallengeDrawFour), boolInt(settings.AllowMultipleDecks), boolInt(settings.BotsEnabled))
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO game_participants(game_id,user_id,last_seen_at) VALUES(?,?,?)`, id, userID, created)
	}
	if err != nil {
		_ = tx.Rollback()
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	if err = tx.Commit(); err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	game, err := gamesdependencies.FetchGame(server.Database(), id, userID)
	if err != nil {
		gamesdependencies.WriteDatabaseError(context.Writer, err)
		return
	}
	gamesdependencies.WriteJSON(context.Writer, http.StatusCreated, gamesdependencies.GameResponse(game, userID))
}

type createGameRequest struct {
	TurnTimeLimit      *int   `json:"turnTimeLimit"`
	PrivateGame        bool   `json:"privateGame"`
	Mode               string `json:"mode"`
	MaxPlayers         *int   `json:"maxPlayers"`
	StartingHandSize   *int   `json:"startingHandSize"`
	DrawPenalty        *int   `json:"drawPenalty"`
	StackDrawCards     bool   `json:"stackDrawCards"`
	JumpIn             bool   `json:"jumpIn"`
	SevenZero          bool   `json:"sevenZero"`
	DrawUntilPlayable  bool   `json:"drawUntilPlayable"`
	ForcePlay          bool   `json:"forcePlay"`
	ChallengeDrawFour  *bool  `json:"challengeDrawFour"`
	AllowMultipleDecks bool   `json:"allowMultipleDecks"`
	BotsEnabled        bool   `json:"botsEnabled"`
}

var gameStatusPattern = regexp.MustCompile(`^(waiting|in_progress|finished)$`)

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func blankValue(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (CreateRoute) Handler() route.Handler {
	return HandleCreate
}

var CreateDefinition = CreateRoute{
	Route: route.NewRoute(http.MethodPost, "/api/v1/games", route.AccessUser),
}
