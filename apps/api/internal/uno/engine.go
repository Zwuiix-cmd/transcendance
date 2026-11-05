package uno

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
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
	GameID       string   `json:"gameId"`
	Status       string   `json:"status"`
	Version      int64    `json:"version"`
	TurnPlayerID string   `json:"turnPlayerId"`
	TurnNumber   int      `json:"turnNumber"`
	Direction    int      `json:"direction"`
	CurrentColor string   `json:"currentColor"`
	DrawPile     []card   `json:"drawPile"`
	DiscardPile  []card   `json:"discardPile"`
	Players      []player `json:"players"`
	WinnerID     string   `json:"winnerId,omitempty"`
	LastAction   string   `json:"lastAction"`
	UpdatedAt    string   `json:"updatedAt"`
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
	LastAction   string         `json:"lastAction"`
}

func NewEngine(database *sql.DB) *Engine { return &Engine{database: database} }

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
		if len(state.DrawPile) == 0 {
			engine.recycle(state)
		}
		if len(state.DrawPile) == 0 {
			return fmt.Errorf("%w: draw pile is empty", ErrInvalidAction)
		}
		state.Players[playerIndex].Hand = append(state.Players[playerIndex].Hand, state.DrawPile[len(state.DrawPile)-1])
		state.DrawPile = state.DrawPile[:len(state.DrawPile)-1]
		state.LastAction = "draw"
		engine.advance(state, playerIndex)
		return nil
	})
}

func (engine *Engine) Play(gameID, userID string, cardIndex int, color string) (Snapshot, error) {
	return engine.mutate(gameID, userID, func(state *gameState, playerIndex int) error {
		hand := state.Players[playerIndex].Hand
		if cardIndex < 0 || cardIndex >= len(hand) {
			return fmt.Errorf("%w: card index is out of range", ErrInvalidCard)
		}
		played := hand[cardIndex]
		if !canPlay(played.Face, state.CurrentColor, state.DiscardPile[len(state.DiscardPile)-1].Face) {
			return fmt.Errorf("%w: card does not match the active color or symbol", ErrInvalidCard)
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
		if len(state.Players[playerIndex].Hand) == 0 {
			state.Status = "finished"
			state.WinnerID = userID
			state.Players[playerIndex].Finished = true
			return nil
		}
		engine.advance(state, playerIndex)
		return nil
	})
}

func (engine *Engine) mutate(gameID, userID string, action func(*gameState, int) error) (Snapshot, error) {
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
		return Snapshot{}, ErrNotYourTurn
	}
	if err = action(state, playerIndex); err != nil {
		return Snapshot{}, err
	}
	state.Version++
	state.TurnNumber++
	state.UpdatedAt = now()
	if state.Status == "finished" {
		if _, err = tx.Exec(`UPDATE games SET status='finished',ended_at=? WHERE id=?`, state.UpdatedAt, gameID); err != nil {
			return Snapshot{}, err
		}
	}
	if err = save(tx, state); err != nil {
		return Snapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return Snapshot{}, err
	}
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
	var handSize, maxPlayers, botsEnabled, multipleDecks int
	if err := tx.QueryRow(`SELECT COALESCE(max_players,4),COALESCE(starting_hand_size,7),COALESCE(bots_enabled,0),COALESCE(allow_multiple_decks,0) FROM game_settings WHERE game_id=?`, gameID).Scan(&maxPlayers, &handSize, &botsEnabled, &multipleDecks); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT p.user_id,u.username FROM game_participants p JOIN users u ON u.id=p.user_id JOIN games g ON g.id=p.game_id WHERE p.game_id=? ORDER BY CASE WHEN p.user_id=g.created_by THEN 0 ELSE 1 END,p.user_id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	state := &gameState{GameID: gameID, Status: "active", Version: 1, Direction: 1, TurnNumber: 1, UpdatedAt: now()}
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
}

func (engine *Engine) advance(state *gameState, current int) {
	for step := 1; step <= len(state.Players); step++ {
		next := (current + state.Direction*step) % len(state.Players)
		if next < 0 {
			next += len(state.Players)
		}
		if !state.Players[next].Finished {
			state.TurnPlayerID = state.Players[next].ID
			return
		}
	}
	state.Status = "finished"
}

func publicSnapshot(state *gameState, viewerID string) Snapshot {
	snapshot := Snapshot{GameID: state.GameID, Status: state.Status, Version: state.Version, TurnPlayerID: state.TurnPlayerID, TurnNumber: state.TurnNumber, CurrentColor: state.CurrentColor, DrawCount: len(state.DrawPile), LastAction: state.LastAction, WinnerID: state.WinnerID}
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
