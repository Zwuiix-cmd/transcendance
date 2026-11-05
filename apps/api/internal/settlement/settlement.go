package settlement

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"card-arena/api/internal/rating"
	"github.com/google/uuid"
)

var (
	ErrGameNotFound  = errors.New("game not found")
	ErrGameNotActive = errors.New("game is not active")
)

type ValidationError struct {
	Message string
}

func (err *ValidationError) Error() string { return err.Message }

type Participant struct {
	UserID          string `json:"userId"`
	Position        int    `json:"position"`
	Score           int    `json:"score"`
	UnosCalled      int    `json:"unosCalled"`
	UnoPenalties    int    `json:"unoPenalties"`
	DrawFoursPlayed int    `json:"drawFoursPlayed"`
	CardsDrawn      int    `json:"cardsDrawn"`
}

type Result struct {
	GameID       string
	Participants []Participant
}

type ParticipantOutcome struct {
	UserID   string
	Position int
	Score    int
	EloDelta int
}

type Outcome struct {
	GameID         string
	Mode           string
	Status         string
	EndedAt        string
	AlreadySettled bool
	Participants   []ParticipantOutcome
}

type storedParticipant struct {
	UserID string
	Elo    int
}

// Apply settles a result inside the caller's transaction. It is deliberately
// idempotent: a retry after the game is finished returns the stored result and
// never increments statistics or applies Elo a second time.
func Apply(tx *sql.Tx, result Result, now time.Time) (Outcome, error) {
	if tx == nil {
		return Outcome{}, fmt.Errorf("settlement transaction is nil")
	}
	if result.GameID == "" {
		return Outcome{}, validation("game id is required")
	}
	if len(result.Participants) < 2 {
		return Outcome{}, validation("at least two participants are required")
	}
	if err := validateInput(result.Participants); err != nil {
		return Outcome{}, err
	}

	var status, mode string
	var endedAt sql.NullString
	if err := tx.QueryRow(`SELECT status,mode,ended_at FROM games WHERE id=?`, result.GameID).Scan(&status, &mode, &endedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Outcome{}, ErrGameNotFound
		}
		return Outcome{}, fmt.Errorf("read game: %w", err)
	}
	if status == "finished" {
		return loadFinished(tx, result.GameID, mode, endedAt)
	}
	if status != "waiting" && status != "in_progress" {
		return Outcome{}, ErrGameNotActive
	}

	stored, err := loadParticipants(tx, result.GameID)
	if err != nil {
		return Outcome{}, err
	}
	if err := validateParticipantsAgainstGame(result.Participants, stored); err != nil {
		return Outcome{}, err
	}

	changes := make(map[string]rating.Change, len(stored))
	if mode == "ranked" {
		players := make([]rating.Player, 0, len(stored))
		for _, participant := range result.Participants {
			players = append(players, rating.Player{ID: participant.UserID, Elo: stored[participant.UserID].Elo, Position: participant.Position})
		}
		calculated, calculateErr := rating.Calculate(players, rating.DefaultKFactor)
		if calculateErr != nil {
			return Outcome{}, fmt.Errorf("calculate Elo: %w", calculateErr)
		}
		for _, change := range calculated {
			changes[change.ID] = change
		}
	}

	ended := now.UTC().Format(time.RFC3339Nano)
	_, err = tx.Exec(`UPDATE games SET status='finished', started_at=COALESCE(started_at,?), ended_at=? WHERE id=? AND status IN ('waiting','in_progress')`, ended, ended, result.GameID)
	if err != nil {
		return Outcome{}, fmt.Errorf("finish game: %w", err)
	}

	ordered := append([]Participant(nil), result.Participants...)
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].Position == ordered[right].Position {
			return ordered[left].UserID < ordered[right].UserID
		}
		return ordered[left].Position < ordered[right].Position
	})
	outcome := Outcome{GameID: result.GameID, Mode: mode, Status: "finished", EndedAt: ended, Participants: make([]ParticipantOutcome, 0, len(ordered))}
	for _, participant := range ordered {
		change := changes[participant.UserID]
		if mode != "ranked" {
			change = rating.Change{ID: participant.UserID, PreviousElo: stored[participant.UserID].Elo, NewElo: stored[participant.UserID].Elo}
		}
		if _, err = tx.Exec(`UPDATE game_participants SET position=?,score=?,elo_delta=? WHERE game_id=? AND user_id=?`, participant.Position, participant.Score, change.Delta, result.GameID, participant.UserID); err != nil {
			return Outcome{}, fmt.Errorf("store participant result: %w", err)
		}
		if mode == "ranked" {
			if _, err = tx.Exec(`UPDATE users SET elo=? WHERE id=?`, change.NewElo, participant.UserID); err != nil {
				return Outcome{}, fmt.Errorf("update Elo: %w", err)
			}
			if _, err = tx.Exec(`INSERT INTO user_elo_history(id,user_id,game_id,previous_elo,new_elo,delta,reason,created_at) VALUES(?,?,?,?,?,?,?,?)`, newID(), participant.UserID, result.GameID, change.PreviousElo, change.NewElo, change.Delta, "ranked_game", ended); err != nil {
				return Outcome{}, fmt.Errorf("store Elo history: %w", err)
			}
		}
		if err = updateStats(tx, participant); err != nil {
			return Outcome{}, err
		}
		outcome.Participants = append(outcome.Participants, ParticipantOutcome{UserID: participant.UserID, Position: participant.Position, Score: participant.Score, EloDelta: change.Delta})
	}
	return outcome, nil
}

func loadFinished(tx *sql.Tx, gameID, mode string, endedAt sql.NullString) (Outcome, error) {
	rows, err := tx.Query(`SELECT user_id,COALESCE(position,0),score,elo_delta FROM game_participants WHERE game_id=? ORDER BY CASE WHEN position IS NULL THEN 1 ELSE 0 END,position,user_id`, gameID)
	if err != nil {
		return Outcome{}, fmt.Errorf("read settled participants: %w", err)
	}
	defer rows.Close()
	participants := []ParticipantOutcome{}
	for rows.Next() {
		var participant ParticipantOutcome
		if err := rows.Scan(&participant.UserID, &participant.Position, &participant.Score, &participant.EloDelta); err != nil {
			return Outcome{}, fmt.Errorf("scan settled participant: %w", err)
		}
		participants = append(participants, participant)
	}
	if err := rows.Err(); err != nil {
		return Outcome{}, fmt.Errorf("read settled participants: %w", err)
	}
	return Outcome{GameID: gameID, Mode: mode, Status: "finished", EndedAt: endedAt.String, AlreadySettled: true, Participants: participants}, nil
}

func loadParticipants(tx *sql.Tx, gameID string) (map[string]storedParticipant, error) {
	rows, err := tx.Query(`SELECT p.user_id,u.elo FROM game_participants p JOIN users u ON u.id=p.user_id WHERE p.game_id=?`, gameID)
	if err != nil {
		return nil, fmt.Errorf("read game participants: %w", err)
	}
	defer rows.Close()
	participants := map[string]storedParticipant{}
	for rows.Next() {
		var participant storedParticipant
		if err := rows.Scan(&participant.UserID, &participant.Elo); err != nil {
			return nil, fmt.Errorf("scan game participant: %w", err)
		}
		participants[participant.UserID] = participant
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read game participants: %w", err)
	}
	return participants, nil
}

func validateInput(participants []Participant) error {
	seenIDs := make(map[string]struct{}, len(participants))
	seenPositions := make(map[int]struct{}, len(participants))
	for _, participant := range participants {
		if participant.UserID == "" {
			return validation("participant user id is required")
		}
		if _, exists := seenIDs[participant.UserID]; exists {
			return validation("a participant appears more than once")
		}
		seenIDs[participant.UserID] = struct{}{}
		if participant.Position < 1 || participant.Position > len(participants) {
			return validation("participant positions must be between 1 and the number of participants")
		}
		if _, exists := seenPositions[participant.Position]; exists {
			return validation("participant positions must be unique")
		}
		seenPositions[participant.Position] = struct{}{}
		if participant.Score < 0 || participant.UnosCalled < 0 || participant.UnoPenalties < 0 || participant.DrawFoursPlayed < 0 || participant.CardsDrawn < 0 {
			return validation("scores and statistics cannot be negative")
		}
	}
	for position := 1; position <= len(participants); position++ {
		if _, exists := seenPositions[position]; !exists {
			return validation("participant positions must be contiguous")
		}
	}
	return nil
}

func validateParticipantsAgainstGame(input []Participant, stored map[string]storedParticipant) error {
	if len(input) != len(stored) {
		return validation("the result must include every game participant")
	}
	for _, participant := range input {
		if _, exists := stored[participant.UserID]; !exists {
			return validation("the result contains a user who is not in the game")
		}
	}
	return nil
}

func updateStats(tx *sql.Tx, participant Participant) error {
	if _, err := tx.Exec(`INSERT OR IGNORE INTO user_stats(user_id) VALUES(?)`, participant.UserID); err != nil {
		return fmt.Errorf("ensure user stats: %w", err)
	}
	var currentStreak, maxStreak int
	if err := tx.QueryRow(`SELECT current_streak,max_streak FROM user_stats WHERE user_id=?`, participant.UserID).Scan(&currentStreak, &maxStreak); err != nil {
		return fmt.Errorf("read user stats: %w", err)
	}
	nextStreak := 0
	if participant.Position == 1 {
		nextStreak = currentStreak + 1
	}
	if nextStreak > maxStreak {
		maxStreak = nextStreak
	}
	_, err := tx.Exec(`UPDATE user_stats SET games_played=games_played+1,games_won=games_won+?,current_streak=?,max_streak=?,unos_called=unos_called+?,uno_penalties=uno_penalties+?,draw_fours_played=draw_fours_played+?,cards_drawn=cards_drawn+? WHERE user_id=?`, boolInt(participant.Position == 1), nextStreak, maxStreak, participant.UnosCalled, participant.UnoPenalties, participant.DrawFoursPlayed, participant.CardsDrawn, participant.UserID)
	if err != nil {
		return fmt.Errorf("update user stats: %w", err)
	}
	return nil
}

func validation(message string) error { return &ValidationError{Message: message} }

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func newID() string {
	return uuid.NewString()
}
