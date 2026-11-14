package uno

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
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
	ErrNotAPlayer       = errors.New("user is not seated at this table")
)

type Engine struct {
	database *sql.DB
	mu       sync.Mutex
	random   *rand.Rand
	onChange func(Update)
	timers   map[string]*botTimers
	stopped  bool
	botSpeed float64
}

// Update describes a committed state change. Snapshot renders the state for
// one viewer so that hands stay private to their owner.
type Update struct {
	GameID   string
	Status   string
	WinnerID string
	Snapshot func(viewerID string) Snapshot
}

type card struct {
	Face string `json:"face"`
}

type player struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Bot       bool   `json:"bot"`
	Finished  bool   `json:"finished"`
	SaidUno   bool   `json:"saidUno"`
	HandValue int    `json:"handValue"`
	Hand      []card `json:"hand"`
}

type gameState struct {
	GameID        string              `json:"gameId"`
	Status        string              `json:"status"`
	Version       int64               `json:"version"`
	TurnPlayerID  string              `json:"turnPlayerId"`
	TurnNumber    int                 `json:"turnNumber"`
	Direction     int                 `json:"direction"`
	Phase         string              `json:"phase"`
	DrawnCard     int                 `json:"drawnCard"`
	Pending       *pendingPenalty     `json:"pending,omitempty"`
	Catchable     string              `json:"catchable,omitempty"`
	Skipped       []string            `json:"skipped,omitempty"`
	CurrentColor  string              `json:"currentColor"`
	DrawPile      []card              `json:"drawPile"`
	DiscardPile   []card              `json:"discardPile"`
	Players       []player            `json:"players"`
	Results       []PlayerResult      `json:"results,omitempty"`
	WinnerID      string              `json:"winnerId,omitempty"`
	LastAction    string              `json:"lastAction"`
	LastEvent     lastEvent           `json:"lastEvent"`
	BotDifficulty string              `json:"botDifficulty,omitempty"`
	Lacks         map[string]lackNote `json:"lacks,omitempty"`
	Rules         gameRules           `json:"rules"`
	UpdatedAt     string              `json:"updatedAt"`
	botActing     bool
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
}

type PublicPlayer struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Bot       bool     `json:"bot"`
	Finished  bool     `json:"finished"`
	SaidUno   bool     `json:"saidUno"`
	CardCount int      `json:"cardCount"`
	HandValue int      `json:"handValue,omitempty"`
	Hand      []string `json:"hand,omitempty"`
}

type PublicEvent struct {
	Type     string `json:"type"`
	PlayerID string `json:"playerId,omitempty"`
	TargetID string `json:"targetId,omitempty"`
	Card     string `json:"card,omitempty"`
	Color    string `json:"color,omitempty"`
	Count    int    `json:"count,omitempty"`
	Jump     bool   `json:"jump,omitempty"`
}

type Snapshot struct {
	GameID        string         `json:"gameId"`
	Status        string         `json:"status"`
	Version       int64          `json:"version"`
	TurnPlayerID  string         `json:"turnPlayerId"`
	TurnNumber    int            `json:"turnNumber"`
	Direction     int            `json:"direction"`
	Phase         string         `json:"phase"`
	PendingDraw   int            `json:"pendingDraw,omitempty"`
	PendingFromID string         `json:"pendingFromId,omitempty"`
	Catchable     string         `json:"catchable,omitempty"`
	Skipped       []string       `json:"skipped,omitempty"`
	CurrentColor  string         `json:"currentColor"`
	TopCard       string         `json:"topCard"`
	DrawCount     int            `json:"drawCount"`
	Players       []PublicPlayer `json:"players"`
	Playable      []int          `json:"playable,omitempty"`
	Results       []PlayerResult `json:"results,omitempty"`
	WinnerID      string         `json:"winnerId,omitempty"`
	LastAction    string         `json:"lastAction"`
	LastEvent     PublicEvent    `json:"lastEvent"`
	Rules         gameRules      `json:"rules"`
}

func NewEngine(database *sql.DB) *Engine {
	return &Engine{database: database, random: rand.New(rand.NewSource(time.Now().UnixNano())), timers: map[string]*botTimers{}, botSpeed: 1}
}

// Stop cancels every pending bot move. It is safe to call more than once.
func (engine *Engine) Stop() {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.stopped = true
	for gameID := range engine.timers {
		engine.cancelBots(gameID)
	}
}

// OnChange registers the callback invoked after every committed mutation.
func (engine *Engine) OnChange(callback func(Update)) { engine.onChange = callback }

func (engine *Engine) emit(state *gameState) {
	if engine.onChange == nil {
		return
	}
	copied := *state
	engine.onChange(Update{GameID: copied.GameID, Status: copied.Status, WinnerID: copied.WinnerID, Snapshot: func(viewerID string) Snapshot { return publicSnapshot(&copied, viewerID) }})
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
	engine.scheduleBots(state)
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
	if _, pending := engine.timers[gameID]; !pending {
		engine.scheduleBots(state)
	}
	return publicSnapshot(state, viewerID), nil
}

func (engine *Engine) Draw(gameID, userID string) (Snapshot, error) {
	return engine.Act(gameID, userID, Action{Type: ActionDraw})
}

func (engine *Engine) Play(gameID, userID string, cardIndex int, color string) (Snapshot, error) {
	return engine.Act(gameID, userID, Action{Type: ActionPlay, CardIndex: cardIndex, Color: color})
}

// Act applies one player action under the official UNO rules and persists the
// resulting state. Every rule check lives in rules.go so the engine stays the
// single source of truth for what a player may do.
func (engine *Engine) Act(gameID, userID string, action Action) (Snapshot, error) {
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
	if state.Status != "active" {
		return Snapshot{}, ErrGameNotActive
	}
	actorIndex := state.playerIndex(userID)
	if actorIndex < 0 {
		return Snapshot{}, ErrNotAPlayer
	}
	if err = state.apply(actorIndex, action, engine.random); err != nil {
		return Snapshot{}, err
	}
	if err = engine.commit(tx, state); err != nil {
		return Snapshot{}, err
	}
	return publicSnapshot(state, userID), nil
}

func (engine *Engine) commit(tx *sql.Tx, state *gameState) error {
	state.Version++
	state.TurnNumber++
	state.UpdatedAt = now()
	if state.Status == "finished" {
		if _, err := tx.Exec(`UPDATE games SET status='finished',ended_at=? WHERE id=?`, state.UpdatedAt, state.GameID); err != nil {
			return err
		}
	}
	if err := save(tx, state); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	engine.emit(state)
	engine.scheduleBots(state)
	return nil
}

func (engine *Engine) loadOrCreate(tx *sql.Tx, gameID string) (*gameState, error) {
	var encoded string
	err := tx.QueryRow(`SELECT state_json FROM uno_game_states WHERE game_id=?`, gameID).Scan(&encoded)
	if err == nil {
		state := gameState{DrawnCard: -1}
		if err = json.Unmarshal([]byte(encoded), &state); err != nil {
			return nil, fmt.Errorf("decode uno state: %w", err)
		}
		if state.Phase == "" {
			state.Phase = PhaseTurn
		}
		if state.Rules.DrawPenalty == 0 {
			state.Rules = defaultRules()
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
	return engine.initialize(tx, gameID)
}

func (engine *Engine) initialize(tx *sql.Tx, gameID string) (*gameState, error) {
	var handSize, maxPlayers, botsEnabled, multipleDecks, drawPenalty, stack, jump, sevenZero, drawUntil, forcePlay, challenge int
	var difficulty string
	if err := tx.QueryRow(`SELECT COALESCE(max_players,4),COALESCE(starting_hand_size,7),COALESCE(bots_enabled,0),COALESCE(allow_multiple_decks,0),COALESCE(bot_difficulty,'intermediate'),COALESCE(draw_penalty,2),COALESCE(stack_draw_cards,0),COALESCE(jump_in,0),COALESCE(seven_zero,0),COALESCE(draw_until_playable,0),COALESCE(force_play,0),COALESCE(challenge_draw_four,1) FROM game_settings WHERE game_id=?`, gameID).Scan(&maxPlayers, &handSize, &botsEnabled, &multipleDecks, &difficulty, &drawPenalty, &stack, &jump, &sevenZero, &drawUntil, &forcePlay, &challenge); err != nil {
		return nil, err
	}
	rules := gameRules{DrawPenalty: drawPenalty, StackDrawCards: stack != 0, JumpIn: jump != 0, SevenZero: sevenZero != 0, DrawUntilPlayable: drawUntil != 0, ForcePlay: forcePlay != 0, ChallengeDrawFour: challenge != 0}
	rows, err := tx.Query(`SELECT p.user_id,u.username FROM game_participants p JOIN users u ON u.id=p.user_id JOIN games g ON g.id=p.game_id WHERE p.game_id=? ORDER BY CASE WHEN p.user_id=g.created_by THEN 0 ELSE 1 END,p.user_id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	state := &gameState{GameID: gameID, Status: "active", Version: 1, TurnNumber: 1, UpdatedAt: now(), BotDifficulty: difficulty, Rules: rules}
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
	engine.random.Shuffle(len(deck), func(left, right int) { deck[left], deck[right] = deck[right], deck[left] })
	deal(state, deck, handSize, engine.random)
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

func publicSnapshot(state *gameState, viewerID string) Snapshot {
	snapshot := Snapshot{
		GameID: state.GameID, Status: state.Status, Version: state.Version, TurnPlayerID: state.TurnPlayerID, TurnNumber: state.TurnNumber,
		Direction: state.Direction, Phase: state.Phase, Catchable: state.Catchable, Skipped: state.Skipped, CurrentColor: state.CurrentColor, DrawCount: len(state.DrawPile),
		Results: state.Results, LastAction: state.LastAction, WinnerID: state.WinnerID, LastEvent: PublicEvent(state.LastEvent), Rules: state.Rules,
	}
	if state.Pending != nil {
		snapshot.PendingDraw = state.Pending.Count
		snapshot.PendingFromID = state.Pending.FromID
	}
	if len(state.DiscardPile) > 0 {
		snapshot.TopCard = state.topCard().Face
	}
	for index, current := range state.Players {
		item := PublicPlayer{ID: current.ID, Name: current.Name, Bot: current.Bot, Finished: current.Finished, SaidUno: current.SaidUno, CardCount: len(current.Hand)}
		if state.Status == "finished" {
			item.HandValue = current.HandValue
		}
		if current.ID == viewerID && !current.Bot {
			item.Hand = make([]string, 0, len(current.Hand))
			for _, card := range current.Hand {
				item.Hand = append(item.Hand, card.Face)
			}
			if state.Status == "active" && state.TurnPlayerID == viewerID {
				snapshot.Playable = state.playableIndexes(index)
			} else if state.Status == "active" {
				snapshot.Playable = state.jumpInIndexes(index)
			}
		}
		snapshot.Players = append(snapshot.Players, item)
	}
	return snapshot
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
