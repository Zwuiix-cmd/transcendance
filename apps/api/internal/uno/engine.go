package uno

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrGameNotActive    = errors.New("uno game is not active")
	ErrNotYourTurn      = errors.New("it is not the player's turn")
	ErrInvalidCard      = errors.New("card cannot be played")
	ErrInvalidAction    = errors.New("uno action is invalid")
	ErrNotOwner         = errors.New("user is not the game owner")
	ErrNotEnoughPlayers = errors.New("the game needs more players")
)

type Engine struct {
	database *sql.DB
	mu       sync.Mutex
	onChange func(Update)
}

type Update struct {
    GameID   string
    Status   string
    WinnerID string
    Results  []PlayerResult
    Snapshot func(viewerID string) Snapshot
}

type card struct {
	Face string `json:"face"`
}

type player struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Bot      bool   `json:"bot"`
	Finished bool   `json:"finished"`
	Hand     []card `json:"hand"`
}

type gameState struct {
	GameID            string         `json:"gameId"`
	Status            string         `json:"status"`
	Version           int64          `json:"version"`
	TurnPlayerID      string         `json:"turnPlayerId"`
	TurnNumber        int            `json:"turnNumber"`
	Direction         int            `json:"direction"`
	CurrentColor      string         `json:"currentColor"`
	DrawPile          []card         `json:"drawPile"`
	DiscardPile       []card         `json:"discardPile"`
	Players           []player       `json:"players"`
	WinnerID          string         `json:"winnerId,omitempty"`
	WinnerScore       int            `json:"winnerScore"`
	Results           []PlayerResult `json:"results,omitempty"`
	LastAction        string         `json:"lastAction"`
	UpdatedAt         string         `json:"updatedAt"`
	Rules             gameRules      `json:"rules"`
	PendingDraw       int            `json:"pendingDraw,omitempty"`
	PendingW4By       string         `json:"pendingW4By,omitempty"`
	PendingW4HadColor bool           `json:"pendingW4HadColor,omitempty"`
	LastDrawnBy       string         `json:"lastDrawnBy,omitempty"`
	LastDrawnCard     string         `json:"lastDrawnCard,omitempty"`
}

type gameRules struct {
	DrawPenalty       int  `json:"drawPenalty"`
	StackDrawCards    bool `json:"stackDrawCards"`
	JumpIn            bool `json:"jumpIn"`
	SevenZero         bool `json:"sevenZero"`
	DrawUntilPlayable bool `json:"drawUntilPlayable"`
	ForcePlay         bool `json:"forcePlay"`
	ChallengeDrawFour bool `json:"challengeDrawFour"`
}

type PlayerResult struct {
	Rank           int    `json:"rank"`
	PlayerID       string `json:"playerId"`
	Name           string `json:"name"`
	Bot            bool   `json:"bot"`
	Winner         bool   `json:"winner"`
	CardsRemaining int    `json:"cardsRemaining"`
	HandPoints     int    `json:"handPoints"`
	PointsEarned   int    `json:"pointsEarned"`
	UnosCalled	   int	  `json:"unosCalled"`
	UnoPenalties   int	  `json:"unoPenalties"`
	DrawFourPlayed int	  `json:"drawFourPlayed"`
	CardsDrawn	   int	  `json:"cardsDrawn"`
}

type PublicPlayer struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Bot       bool     `json:"bot"`
	Finished  bool     `json:"finished"`
	CardCount int      `json:"cardCount"`
	Hand      []string `json:"hand,omitempty"`
}

type Snapshot struct {
	GameID       string         `json:"gameId"`
	Status       string         `json:"status"`
	Version      int64          `json:"version"`
	TurnPlayerID string         `json:"turnPlayerId"`
	TurnNumber   int            `json:"turnNumber"`
	CurrentColor string         `json:"currentColor"`
	TopCard      string         `json:"topCard"`
	DrawCount    int            `json:"drawCount"`
	Players      []PublicPlayer `json:"players"`
	WinnerID     string         `json:"winnerId,omitempty"`
	WinnerScore  int            `json:"winnerScore"`
	Results      []PlayerResult `json:"results,omitempty"`
	LastAction   string         `json:"lastAction"`
	PendingDraw  int            `json:"pendingDraw,omitempty"`
	CanChallenge bool           `json:"canChallenge"`
}

func NewEngine(database *sql.DB) *Engine { return &Engine{database: database} }

func (engine *Engine) OnChange(callback func(Update)) { engine.onChange = callback }

func (engine *Engine) emit(state *gameState) {
    if engine.onChange == nil {
        return
    }

    copied := *state
    copied.Results = append([]PlayerResult(nil), state.Results...)

    engine.onChange(Update{
        GameID:   copied.GameID,
        Status:   copied.Status,
        WinnerID: copied.WinnerID,
        Results:  copied.Results,
        Snapshot: func(viewerID string) Snapshot {
            return publicSnapshot(&copied, viewerID)
        },
    })
}

func (engine *Engine) Start(gameID, userID string) (Snapshot, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	tx, err := engine.database.Begin()
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback()
	var owner, status string
	var botsEnabled, participantCount int
	if err = tx.QueryRow(`SELECT g.created_by,g.status,COALESCE(gs.bots_enabled,0),(SELECT COUNT(*) FROM game_participants p WHERE p.game_id=g.id) FROM games g LEFT JOIN game_settings gs ON gs.game_id=g.id WHERE g.id=?`, gameID).Scan(&owner, &status, &botsEnabled, &participantCount); err != nil {
		return Snapshot{}, err
	}
	if owner != userID {
		return Snapshot{}, ErrNotOwner
	}
	if status != "waiting" && status != "in_progress" {
		return Snapshot{}, ErrGameNotActive
	}
	if botsEnabled == 0 && participantCount < 2 {
		return Snapshot{}, ErrNotEnoughPlayers
	}
	if status == "waiting" {
		if _, err = tx.Exec(`UPDATE games SET status='in_progress',started_at=? WHERE id=?`, now(), gameID); err != nil {
			return Snapshot{}, err
		}
	}
	state, err := engine.loadOrCreate(tx, gameID)
	if err != nil {
		return Snapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return Snapshot{}, err
	}
	engine.emit(state)
	return publicSnapshot(state, userID), nil
}

func (engine *Engine) State(gameID, viewerID string) (Snapshot, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	tx, err := engine.database.Begin()
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback()
	state, err := engine.loadOrCreate(tx, gameID)
	if err != nil {
		return Snapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return Snapshot{}, err
	}
	return publicSnapshot(state, viewerID), nil
}

func (engine *Engine) Draw(gameID, userID string) (Snapshot, error) {
	return engine.mutate(gameID, userID, func(state *gameState, playerIndex int) error {
		if state.PendingDraw > 0 {
			if err := engine.drawCards(state, playerIndex, state.PendingDraw); err != nil {
				return err
			}
			state.PendingDraw = 0
			state.PendingW4By = ""
			state.PendingW4HadColor = false
			state.LastAction = "draw-penalty"
			engine.advance(state, playerIndex, 1)
			return nil
		}
		var drawn card
		for {
			if len(state.DrawPile) == 0 {
				engine.recycle(state)
			}
			if len(state.DrawPile) == 0 {
				engine.advance(state, playerIndex, 1)
				state.LastAction = "draw-pile-empty-pass"
				return nil
			}
			drawn = state.DrawPile[len(state.DrawPile)-1]
			state.DrawPile = state.DrawPile[:len(state.DrawPile)-1]
			state.Players[playerIndex].Hand = append(state.Players[playerIndex].Hand, drawn)
			if !state.Rules.DrawUntilPlayable || canPlay(drawn.Face, state.CurrentColor, state.DiscardPile[len(state.DiscardPile)-1].Face) {
				break
			}
		}
		state.LastDrawnBy, state.LastDrawnCard = userID, drawn.Face
		state.LastAction = "draw"
		if state.Rules.ForcePlay && canPlay(drawn.Face, state.CurrentColor, state.DiscardPile[len(state.DiscardPile)-1].Face) {
			chosenColor := ""
			if isWild(drawn.Face) {
				chosenColor = mostCommonColor(state.Players[playerIndex].Hand[:len(state.Players[playerIndex].Hand)-1])
			}
			return engine.playFromHand(state, playerIndex, len(state.Players[playerIndex].Hand)-1, chosenColor)
		}
		if state.Rules.DrawUntilPlayable && canPlay(drawn.Face, state.CurrentColor, state.DiscardPile[len(state.DiscardPile)-1].Face) {
			return nil
		}
		engine.advance(state, playerIndex, 1)
		return nil
	})
}

func (engine *Engine) Play(gameID, userID string, cardIndex int, color string) (Snapshot, error) {
	return engine.mutate(gameID, userID, func(state *gameState, playerIndex int) error {
		if state.TurnPlayerID != userID {
			if !state.Rules.JumpIn || len(state.DiscardPile) == 0 || cardIndex < 0 || cardIndex >= len(state.Players[playerIndex].Hand) || state.Players[playerIndex].Hand[cardIndex].Face != state.DiscardPile[len(state.DiscardPile)-1].Face {
				return ErrNotYourTurn
			}
			state.TurnPlayerID = userID
		}
		return engine.playFromHand(state, playerIndex, cardIndex, color)
	}, true)
}

func (engine *Engine) playFromHand(state *gameState, playerIndex, cardIndex int, color string) error {
	hand := state.Players[playerIndex].Hand
	if cardIndex < 0 || cardIndex >= len(hand) {
		return fmt.Errorf("%w: card index is out of range", ErrInvalidCard)
	}
	played := hand[cardIndex]
	if state.PendingDraw > 0 {
		if !state.Rules.StackDrawCards || (symbolOf(played.Face) != "A2" && played.Face != "W4") {
			return fmt.Errorf("%w: a draw penalty must be accepted or stacked", ErrInvalidCard)
		}
	} else if len(state.DiscardPile) == 0 || !canPlay(played.Face, state.CurrentColor, state.DiscardPile[len(state.DiscardPile)-1].Face) {
		return fmt.Errorf("%w: card does not match the active color or symbol", ErrInvalidCard)
	}
	previousColor := state.CurrentColor
	hadActiveColor := false
	if played.Face == "W4" {
		for i, c := range hand {
			if i != cardIndex && colorOf(c.Face) == previousColor {
				hadActiveColor = true
			}
		}
	}
	if isWild(played.Face) {
		color = strings.ToUpper(strings.TrimSpace(color))
		if color != "R" && color != "G" && color != "B" && color != "Y" {
			return fmt.Errorf("%w: a color is required for a wild card", ErrInvalidAction)
		}
		state.CurrentColor = color
	} else {
		state.CurrentColor = colorOf(played.Face)
	}

	state.Players[playerIndex].Hand = append(hand[:cardIndex], hand[cardIndex+1:]...)
	state.DiscardPile = append(state.DiscardPile, played)
	state.LastAction = "play"
	state.LastDrawnBy, state.LastDrawnCard = "", ""
	if len(state.Players[playerIndex].Hand) == 0 {
		state.Status = "finished"
		state.WinnerID = state.Players[playerIndex].ID
		state.Players[playerIndex].Finished = true
		state.Results = calculateResults(state, playerIndex)
		state.WinnerScore = state.Results[0].PointsEarned
		return nil
	}
	if state.PendingDraw > 0 && state.Rules.StackDrawCards {
		state.PendingDraw += penaltyFor(played.Face, state.Rules.DrawPenalty)
	} else if played.Face == "W4" || symbolOf(played.Face) == "A2" {
		state.PendingDraw = penaltyFor(played.Face, state.Rules.DrawPenalty)
	}
	if played.Face == "W4" {
		state.PendingW4By = state.Players[playerIndex].ID
		state.PendingW4HadColor = hadActiveColor
	}
	if state.Rules.SevenZero && symbolOf(played.Face) == "7" {
		next, ok := nextIndexRaw(state, playerIndex)
		if ok {
			state.Players[playerIndex].Hand, state.Players[next].Hand = state.Players[next].Hand, state.Players[playerIndex].Hand
		}
	}
	if state.Rules.SevenZero && symbolOf(played.Face) == "0" {
		rotateHands(state)
	}
	steps, err := engine.applyCardEffect(state, playerIndex, played)
	if err != nil {
		return err
	}
	if state.PendingDraw > 0 && state.Rules.StackDrawCards {
		steps = 1
	}
	engine.advance(state, playerIndex, steps)
	return nil
}

func penaltyFor(face string, configured int) int {
	if face == "W4" {
		return 4
	}
	if configured < 1 {
		return 2
	}
	return configured
}
func mostCommonColor(hand []card) string {
	counts := map[string]int{"R": 0, "G": 0, "B": 0, "Y": 0}
	for _, c := range hand {
		if color := colorOf(c.Face); color != "" {
			counts[color]++
		}
	}
	best := "R"
	for _, color := range []string{"G", "B", "Y"} {
		if counts[color] > counts[best] {
			best = color
		}
	}
	return best
}
func nextIndexRaw(state *gameState, current int) (int, bool) {
	n := len(state.Players)
	if n < 2 {
		return 0, false
	}
	i := (current + state.Direction + n) % n
	return i, !state.Players[i].Finished
}
func rotateHands(state *gameState) {
	active := activePlayers(state)
	if len(active) < 2 {
		return
	}
	hands := make([][]card, len(active))
	for i, p := range active {
		hands[i] = state.Players[p].Hand
	}
	for i, p := range active {
		state.Players[p].Hand = hands[(i-1+len(active))%len(active)]
	}
}

func (engine *Engine) ChallengeDrawFour(gameID, userID string) (Snapshot, error) {
	return engine.mutate(gameID, userID, func(state *gameState, playerIndex int) error {
		if !state.Rules.ChallengeDrawFour || state.PendingW4By == "" || state.PendingDraw != 4 {
			return fmt.Errorf("%w: there is no challengeable +4", ErrInvalidAction)
		}
		if state.PendingW4HadColor {
			for i := range state.Players {
				if state.Players[i].ID == state.PendingW4By {
					if err := engine.drawCards(state, i, 4); err != nil {
						return err
					}
					break
				}
			}
			state.PendingDraw = 0
			state.PendingW4By = ""
			state.PendingW4HadColor = false
			state.LastAction = "challenge-success"
			return nil
		}
		if err := engine.drawCards(state, playerIndex, 6); err != nil {
			return err
		}
		state.PendingDraw = 0
		state.PendingW4By = ""
		state.PendingW4HadColor = false
		state.LastAction = "challenge-failed"
		engine.advance(state, playerIndex, 1)
		return nil
	})
}

func calculateResults(state *gameState, winnerIndex int) []PlayerResult {
	results := make([]PlayerResult, 0, len(state.Players))
	totalWinnerPoints := 0

	for index, current := range state.Players {
		handPoints := calculateHandPoints(current.Hand)
		if index != winnerIndex {
			totalWinnerPoints += handPoints
		}
		results = append(results, PlayerResult{
			PlayerID:       current.ID,
			Name:           current.Name,
			Bot:            current.Bot,
			Winner:         index == winnerIndex,
			CardsRemaining: len(current.Hand),
			HandPoints:     handPoints,
			UnosCalled:	    0,
			UnoPenalties:   0,
			DrawFourPlayed: 0,
			CardsDrawn:     0,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Winner != results[j].Winner {
			return results[i].Winner
		}
		return results[i].HandPoints < results[j].HandPoints
	})

	for index := range results {
		results[index].Rank = index + 1
		if results[index].Winner {
			results[index].PointsEarned = totalWinnerPoints
		}
	}
	return results
}

func calculateHandPoints(hand []card) int {
	total := 0
	for _, remainingCard := range hand {
		face := remainingCard.Face
		switch {
		case isWild(face):
			total += 50
		case symbolOf(face) == "A2" || symbolOf(face) == "R" || symbolOf(face) == "S":
			total += 20
		default:
			value, err := strconv.Atoi(symbolOf(face))
			if err == nil {
				total += value
			}
		}
	}
	return total
}

func (engine *Engine) applyCardEffect(state *gameState, playerIndex int, played card) (int, error) {
	switch played.Face {
	case "WC":
		return 1, nil

	case "W4":
		return 1, nil

	default:
		switch symbolOf(played.Face) {
		case "A2":
			return 1, nil

		case "S":
			return 2, nil

		case "R":
			state.Direction *= -1
			if len(activePlayers(state)) == 2 {
				return 2, nil
			}
			return 1, nil
		}
	}

	return 1, nil
}

func (engine *Engine) drawCards(state *gameState, targetIndex, count int) error {
	for i := 0; i < count; i++ {
		if len(state.DrawPile) == 0 {
			engine.recycle(state)
		}
		if len(state.DrawPile) == 0 {
			break
		}

		last := len(state.DrawPile) - 1
		state.Players[targetIndex].Hand = append(state.Players[targetIndex].Hand, state.DrawPile[last])
		state.DrawPile = state.DrawPile[:last]
	}
	return nil
}

func (engine *Engine) nextPlayerIndex(state *gameState, current int) (int, bool) {
	count := len(state.Players)
	if count < 2 {
		return 0, false
	}

	for step := 1; step < count; step++ {
		next := (current + state.Direction*step) % count
		if next < 0 {
			next += count
		}
		if !state.Players[next].Finished {
			return next, true
		}
	}
	return 0, false
}

func activePlayers(state *gameState) []int {
	var active []int
	for index := range state.Players {
		if !state.Players[index].Finished {
			active = append(active, index)
		}
	}
	return active
}

func (engine *Engine) mutate(gameID, userID string, action func(*gameState, int) error, allowJump ...bool) (Snapshot, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	tx, err := engine.database.Begin()
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback()
	state, err := engine.loadOrCreate(tx, gameID)
	if err != nil {
		return Snapshot{}, err
	}
	playerIndex := -1
	for index := range state.Players {
		if state.Players[index].ID == userID {
			playerIndex = index
			break
		}
	}
	if playerIndex < 0 || state.Players[playerIndex].Bot {
		return Snapshot{}, fmt.Errorf("%w: only a real player can act", ErrInvalidAction)
	}
	if state.Status != "active" {
		return Snapshot{}, ErrGameNotActive
	}
	if state.TurnPlayerID != userID {
		if len(allowJump) == 0 || !allowJump[0] {
			return Snapshot{}, ErrNotYourTurn
		}
	}
	if err = action(state, playerIndex); err != nil {
		return Snapshot{}, err
	}
	state.Version++
	state.TurnNumber++
	state.UpdatedAt = now()
	/*if state.Status == "finished" {
		if _, err = tx.Exec(`UPDATE games SET status='finished',ended_at=? WHERE id=?`, state.UpdatedAt, gameID); err != nil {
			return Snapshot{}, err
		}
	}*/
	if err = save(tx, state); err != nil {
		return Snapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return Snapshot{}, err
	}
	engine.emit(state)
	return publicSnapshot(state, userID), nil
}

func (engine *Engine) loadOrCreate(tx *sql.Tx, gameID string) (*gameState, error) {
	var encoded string
	err := tx.QueryRow(`SELECT state_json FROM uno_game_states WHERE game_id=?`, gameID).Scan(&encoded)
	if err == nil {
		var state gameState
		if err = json.Unmarshal([]byte(encoded), &state); err != nil {
			return nil, fmt.Errorf("decode uno state: %w", err)
		}
		var status string
		if err = tx.QueryRow(`SELECT status FROM games WHERE id=?`, gameID).Scan(&status); err != nil {
			return nil, err
		}
		if status == "finished" {
			state.Status = "finished"
		} else if status != "in_progress" {
			return nil, ErrGameNotActive
		}
		return &state, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var status string
	if err = tx.QueryRow(`SELECT status FROM games WHERE id=?`, gameID).Scan(&status); err != nil {
		return nil, err
	}
	if status != "in_progress" {
		return nil, ErrGameNotActive
	}
	return initialize(tx, gameID)
}

func initialize(tx *sql.Tx, gameID string) (*gameState, error) {
	var handSize, maxPlayers, botsEnabled, multipleDecks, drawPenalty, stack, jump, seven, drawUntil, force, challenge int
	if err := tx.QueryRow(`SELECT COALESCE(max_players,4),COALESCE(starting_hand_size,7),COALESCE(bots_enabled,0),COALESCE(allow_multiple_decks,0),COALESCE(draw_penalty,2),COALESCE(stack_draw_cards,0),COALESCE(jump_in,0),COALESCE(seven_zero,0),COALESCE(draw_until_playable,0),COALESCE(force_play,0),COALESCE(challenge_draw_four,1) FROM game_settings WHERE game_id=?`, gameID).Scan(&maxPlayers, &handSize, &botsEnabled, &multipleDecks, &drawPenalty, &stack, &jump, &seven, &drawUntil, &force, &challenge); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT p.user_id,u.username FROM game_participants p JOIN users u ON u.id=p.user_id JOIN games g ON g.id=p.game_id WHERE p.game_id=? ORDER BY CASE WHEN p.user_id=g.created_by THEN 0 ELSE 1 END,p.user_id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	state := &gameState{GameID: gameID, Status: "active", Version: 1, Direction: 1, TurnNumber: 1, UpdatedAt: now(), Rules: gameRules{DrawPenalty: drawPenalty, StackDrawCards: stack != 0, JumpIn: jump != 0, SevenZero: seven != 0, DrawUntilPlayable: drawUntil != 0, ForcePlay: force != 0, ChallengeDrawFour: challenge != 0}}
	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		state.Players = append(state.Players, player{ID: id, Name: name})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(state.Players) == 0 {
		return nil, fmt.Errorf("%w: game has no players", ErrInvalidAction)
	}
	if botsEnabled != 0 {
		botNames := []string{"Nova", "Pixel", "Orbit", "Mochi", "Rook", "Echo", "Ziggy", "Comet"}
		for index := len(state.Players); index < maxPlayers && index < 8; index++ {
			state.Players = append(state.Players, player{ID: fmt.Sprintf("bot-%s-%d", gameID, index), Name: botNames[index%len(botNames)], Bot: true})
		}
	}
	deck := buildDeck(multipleDecks != 0)
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	random.Shuffle(len(deck), func(left, right int) { deck[left], deck[right] = deck[right], deck[left] })
	for round := 0; round < handSize; round++ {
		for index := range state.Players {
			if len(deck) == 0 {
				break
			}
			state.Players[index].Hand = append(state.Players[index].Hand, deck[len(deck)-1])
			deck = deck[:len(deck)-1]
		}
	}
	for len(deck) > 0 {
		candidate := deck[len(deck)-1]
		deck = deck[:len(deck)-1]
		if !isWild(candidate.Face) {
			state.DiscardPile = append(state.DiscardPile, candidate)
			state.CurrentColor = colorOf(candidate.Face)
			break
		}
		deck = append([]card{candidate}, deck...)
	}
	state.DrawPile = deck
	state.TurnPlayerID = state.Players[0].ID
	if err = save(tx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func save(tx *sql.Tx, state *gameState) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO uno_game_states(game_id,version,state_json,updated_at) VALUES(?,?,?,?) ON CONFLICT(game_id) DO UPDATE SET version=excluded.version,state_json=excluded.state_json,updated_at=excluded.updated_at`, state.GameID, state.Version, string(encoded), state.UpdatedAt)
	return err
}

func (engine *Engine) recycle(state *gameState) {
	if len(state.DiscardPile) <= 1 {
		return
	}
	top := state.DiscardPile[len(state.DiscardPile)-1]
	state.DrawPile = append(state.DrawPile, state.DiscardPile[:len(state.DiscardPile)-1]...)
	state.DiscardPile = []card{top}
	rand.Shuffle(len(state.DrawPile), func(i, j int) {
		state.DrawPile[i], state.DrawPile[j] = state.DrawPile[j], state.DrawPile[i]
	})
}

func (engine *Engine) noCardsAvailable(state *gameState) bool {
	return len(state.DrawPile) == 0 && len(state.DiscardPile) <= 1
}

func (engine *Engine) finishByLowestHand(state *gameState) {
	if len(state.Players) == 0 {
		state.Status = "finished"
		state.LastAction = "round-ended-no-cards"
		return
	}
	winnerIndex := -1
	lowest := int(^uint(0) >> 1)
	for i := range state.Players {
		if state.Players[i].Finished {
			continue
		}
		points := calculateHandPoints(state.Players[i].Hand)
		if winnerIndex == -1 || points < lowest {
			winnerIndex, lowest = i, points
		}
	}
	if winnerIndex == -1 {
		winnerIndex = 0
	}
	state.Status = "finished"
	state.WinnerID = state.Players[winnerIndex].ID
	state.Results = calculateResults(state, winnerIndex)
	if len(state.Results) > 0 {
		state.WinnerScore = state.Results[0].PointsEarned
	}
	state.LastAction = "round-ended-no-cards"
}

func (engine *Engine) advance(state *gameState, current, steps int) {
	if steps < 1 {
		steps = 1
	}

	next := current
	for step := 0; step < steps; step++ {
		var ok bool
		next, ok = engine.nextPlayerIndex(state, next)
		if !ok {
			state.Status = "finished"
			return
		}
	}

	state.TurnPlayerID = state.Players[next].ID
}

func publicSnapshot(state *gameState, viewerID string) Snapshot {
	snapshot := Snapshot{GameID: state.GameID, Status: state.Status, Version: state.Version, TurnPlayerID: state.TurnPlayerID, TurnNumber: state.TurnNumber, CurrentColor: state.CurrentColor, DrawCount: len(state.DrawPile), LastAction: state.LastAction, WinnerID: state.WinnerID, WinnerScore: state.WinnerScore, Results: append([]PlayerResult(nil), state.Results...), PendingDraw: state.PendingDraw, CanChallenge: state.PendingW4By != "" && state.Rules.ChallengeDrawFour}
	if len(state.DiscardPile) > 0 {
		snapshot.TopCard = state.DiscardPile[len(state.DiscardPile)-1].Face
	}
	for _, current := range state.Players {
		item := PublicPlayer{ID: current.ID, Name: current.Name, Bot: current.Bot, Finished: current.Finished, CardCount: len(current.Hand)}
		if current.ID == viewerID && !current.Bot {
			item.Hand = make([]string, 0, len(current.Hand))
			for _, card := range current.Hand {
				item.Hand = append(item.Hand, card.Face)
			}
		}
		snapshot.Players = append(snapshot.Players, item)
	}
	return snapshot
}

func buildDeck(multiple bool) []card {
	decks := 1
	if multiple {
		decks = 2
	}
	colors := []string{"R", "G", "B", "Y"}
	deck := make([]card, 0, decks*108)
	for set := 0; set < decks; set++ {
		for _, color := range colors {
			for value := 0; value <= 9; value++ {
				deck = append(deck, card{Face: fmt.Sprintf("%s%d", color, value)})
			}
			for copy := 0; copy < 2; copy++ {
				deck = append(deck, card{Face: color + "A2"}, card{Face: color + "R"}, card{Face: color + "S"})
			}
		}
		for copy := 0; copy < 4; copy++ {
			deck = append(deck, card{Face: "WC"}, card{Face: "W4"})
		}
	}
	return deck
}

func canPlay(face, color, top string) bool {
	return isWild(face) || colorOf(face) == color || symbolOf(face) == symbolOf(top)
}

func isWild(face string) bool { return face == "WC" || face == "W4" }

func colorOf(face string) string {
	if len(face) == 0 || isWild(face) {
		return ""
	}
	return string(face[0])
}

func symbolOf(face string) string {
	if len(face) < 2 {
		return face
	}
	return face[1:]
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }