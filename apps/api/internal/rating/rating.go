package rating

import (
	"errors"
	"fmt"
	"math"
)

const DefaultKFactor = 32

var (
	ErrNotEnoughPlayers = errors.New("at least two players are required")
	ErrInvalidKFactor   = errors.New("the K-factor must be positive")
)

// Player is the minimum information needed to settle a ranked match.
// Lower positions are better: position 1 is the winner.
type Player struct {
	ID       string
	Elo      int
	Position int
}

type Change struct {
	ID          string
	PreviousElo int
	NewElo      int
	Delta       int
}

// Calculate returns deterministic multi-player Elo changes. Every player is
// compared with every other player, while the K-factor is divided across the
// opponents so a four-player match does not create four times the rating
// movement of a two-player match.
func Calculate(players []Player, kFactor int) ([]Change, error) {
	if len(players) < 2 {
		return nil, ErrNotEnoughPlayers
	}
	if kFactor <= 0 {
		return nil, ErrInvalidKFactor
	}

	seen := make(map[string]struct{}, len(players))
	for _, player := range players {
		if player.ID == "" {
			return nil, fmt.Errorf("player id cannot be empty")
		}
		if _, exists := seen[player.ID]; exists {
			return nil, fmt.Errorf("player %q appears more than once", player.ID)
		}
		seen[player.ID] = struct{}{}
		if player.Elo < 0 {
			return nil, fmt.Errorf("player %q has a negative Elo", player.ID)
		}
		if player.Position < 1 {
			return nil, fmt.Errorf("player %q has an invalid position", player.ID)
		}
	}

	raw := make([]float64, len(players))
	pairK := float64(kFactor) / float64(len(players)-1)
	for left := 0; left < len(players); left++ {
		for right := left + 1; right < len(players); right++ {
			leftActual := outcome(players[left].Position, players[right].Position)
			leftExpected := expected(players[left].Elo, players[right].Elo)
			movement := pairK * (leftActual - leftExpected)
			raw[left] += movement
			raw[right] -= movement
		}
	}

	changes := make([]Change, len(players))
	for index, player := range players {
		delta := int(math.Round(raw[index]))
		newElo := player.Elo + delta
		if newElo < 0 {
			newElo = 0
			delta = newElo - player.Elo
		}
		changes[index] = Change{ID: player.ID, PreviousElo: player.Elo, NewElo: newElo, Delta: delta}
	}
	return changes, nil
}

func expected(playerElo, opponentElo int) float64 {
	return 1 / (1 + math.Pow(10, float64(opponentElo-playerElo)/400))
}

func outcome(playerPosition, opponentPosition int) float64 {
	switch {
	case playerPosition < opponentPosition:
		return 1
	case playerPosition > opponentPosition:
		return 0
	default:
		return 0.5
	}
}
