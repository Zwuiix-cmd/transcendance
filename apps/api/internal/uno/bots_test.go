package uno

import (
	"math/rand"
	"testing"
)

func botTable(difficulty string, players int, random *rand.Rand) *gameState {
	state := &gameState{GameID: "bots", Status: "active", Version: 1, TurnNumber: 1, BotDifficulty: difficulty}
	for index := 0; index < players; index++ {
		state.Players = append(state.Players, player{ID: string(rune('a' + index)), Name: string(rune('A' + index)), Bot: true})
	}
	deck := buildDeck(false)
	random.Shuffle(len(deck), func(left, right int) { deck[left], deck[right] = deck[right], deck[left] })
	deal(state, deck, 7, random)
	return state
}

func playOut(t *testing.T, difficulty string, seed int64) (*gameState, int) {
	t.Helper()
	random := rand.New(rand.NewSource(seed))
	state := botTable(difficulty, 4, random)
	profile := profileFor(difficulty)
	steps := 0
	for state.Status == "active" && steps < 3000 {
		steps++
		if index, action, ok := state.botSideCandidate(profile, random); ok && random.Float64() < 0.5 {
			state.botActing = true
			if err := state.apply(index, action, random); err != nil {
				t.Fatalf("%s side action %s rejected at step %d: %v", difficulty, action.Type, steps, err)
			}
			state.botActing = false
			state.Version++
			state.TurnNumber++
			continue
		}
		botIndex := state.playerIndex(state.TurnPlayerID)
		action := state.botDecision(botIndex, profile, random)
		state.botActing = true
		err := state.apply(botIndex, action, random)
		state.botActing = false
		if err != nil {
			t.Fatalf("%s bot %s made an illegal %s at step %d: %v", difficulty, state.TurnPlayerID, action.Type, steps, err)
		}
		state.Version++
		state.TurnNumber++
	}
	return state, steps
}

func TestBotsAlwaysRespectTheRules(t *testing.T) {
	for _, difficulty := range []string{DifficultyEasy, DifficultyIntermediate, DifficultyExpert} {
		finished := 0
		for seed := int64(1); seed <= 40; seed++ {
			state, _ := playOut(t, difficulty, seed)
			if state.Status == "finished" {
				finished++
				if state.WinnerID == "" || len(state.Players[state.playerIndex(state.WinnerID)].Hand) != 0 {
					t.Fatalf("%s: finished game without a proper winner", difficulty)
				}
			}
		}
		if finished < 38 {
			t.Fatalf("%s bots finished only %d/40 games", difficulty, finished)
		}
	}
}

func TestBotsNeverActWhenBlocked(t *testing.T) {
	state := testState("R7", []string{"R1"}, []string{"R2"})
	state.Players[1].Bot = true
	if _, _, ok := state.botSideCandidate(profileFor(DifficultyExpert), rng()); ok {
		t.Fatalf("no side action should exist without a catchable player")
	}
	err := state.apply(1, Action{Type: ActionDraw}, rng())
	if err == nil {
		t.Fatalf("a bot acting outside the scheduler must be rejected")
	}
}

func realisticState(top string, hands ...[]string) *gameState {
	state := testState(top, hands...)
	used := map[string]int{top: 1}
	for _, hand := range hands {
		for _, face := range hand {
			used[face]++
		}
	}
	state.DrawPile = nil
	for _, current := range buildDeck(false) {
		if used[current.Face] > 0 {
			used[current.Face]--
			continue
		}
		state.DrawPile = append(state.DrawPile, current)
	}
	rng().Shuffle(len(state.DrawPile), func(left, right int) {
		state.DrawPile[left], state.DrawPile[right] = state.DrawPile[right], state.DrawPile[left]
	})
	return state
}

func TestExpertBlocksPlayerAboutToWin(t *testing.T) {
	state := realisticState("R7", []string{"RS", "R9"}, []string{"B2", "B4", "B6"})
	state.Players[0].Bot = true
	action := state.botDecision(0, profileFor(DifficultyExpert), rng())
	if action.Type != ActionPlay || state.Players[0].Hand[action.CardIndex].Face != "RS" {
		t.Fatalf("expert should take the extra turn with Skip and win next, chose %+v", action)
	}
	state = realisticState("R7", []string{"W4", "G9"}, []string{"B2"}, []string{"Y1", "Y2", "Y3"})
	state.Players[0].Bot = true
	action = state.botDecision(0, profileFor(DifficultyExpert), rng())
	if action.Type != ActionPlay || state.Players[0].Hand[action.CardIndex].Face != "W4" || action.Color == "" || !action.Uno {
		t.Fatalf("expert should play its only legal card with a color and call UNO, chose %+v", action)
	}
}

func TestIntermediateNeverBluffsWildDrawFour(t *testing.T) {
	state := testState("R7", []string{"R1", "W4", "G9"}, []string{"B2"}, []string{"Y1", "Y2", "Y3"})
	state.Players[0].Bot = true
	for seed := int64(1); seed <= 20; seed++ {
		action := state.botDecision(0, profileFor(DifficultyIntermediate), rand.New(rand.NewSource(seed)))
		if action.Type == ActionPlay && state.Players[0].Hand[action.CardIndex].Face == "W4" {
			t.Fatalf("intermediate bot bluffed with a Wild +4 while holding a red card")
		}
	}
}

func TestBotsChooseWildColorTheyHold(t *testing.T) {
	state := testState("R7", []string{"WC", "G1", "G2", "B3"}, []string{"B2", "B4"})
	state.Players[0].Bot = true
	state.CurrentColor = "Y"
	state.DiscardPile = []card{{Face: "Y7"}}
	action := state.botDecision(0, profileFor(DifficultyIntermediate), rng())
	if action.Type != ActionPlay || action.Color != "G" {
		t.Fatalf("bot should pick the color it holds most, chose %+v", action)
	}
}

func TestExpertAlwaysCallsUnoAndCatches(t *testing.T) {
	state := testState("R7", []string{"R1", "B2"}, []string{"G1", "G2"}, []string{"Y1"})
	state.Players[0].Bot = true
	action := state.botDecision(0, profileFor(DifficultyExpert), rng())
	if !action.Uno {
		t.Fatalf("expert must call UNO when playing down to one card")
	}
	state = testState("R7", []string{"R1", "B2"}, []string{"G1", "G2"}, []string{"Y1", "Y4"})
	state.Players[2].Bot = true
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	index, side, ok := state.botSideCandidate(profileFor(DifficultyExpert), rng())
	if !ok || index != 2 || side.Type != ActionCatch || side.TargetID != "a" {
		t.Fatalf("a bot should be able to catch the forgetful player, got %v %+v", ok, side)
	}
}

func TestEasyBotSometimesForgetsUno(t *testing.T) {
	state := testState("R7", []string{"R1", "B2"}, []string{"G1", "G2"})
	state.Players[0].Bot = true
	forgot := 0
	for seed := int64(1); seed <= 60; seed++ {
		if action := state.botDecision(0, profileFor(DifficultyEasy), rand.New(rand.NewSource(seed))); action.Type == ActionPlay && !action.Uno {
			forgot++
		}
	}
	if forgot == 0 || forgot == 60 {
		t.Fatalf("easy bot should forget UNO sometimes, forgot %d/60", forgot)
	}
}

func TestExpertChallengeUsesCardCounting(t *testing.T) {
	state := testState("R7", []string{"B1", "B2", "B3"}, []string{"W4", "G1", "G2", "G3", "G4", "G5", "G6", "G7"}, []string{"Y1"})
	state.Players[0].Bot = true
	state.TurnPlayerID = "b"
	act(t, state, "b", Action{Type: ActionPlay, CardIndex: 0, Color: "G"})
	if !state.botShouldChallenge(0, profileFor(DifficultyExpert), rng()) {
		t.Fatalf("expert should challenge a Wild +4 from a player holding seven unseen cards")
	}
	state.noteLack("b", "R")
	if state.botShouldChallenge(0, profileFor(DifficultyExpert), rng()) {
		t.Fatalf("expert should trust a player who just failed to play red")
	}
	if state.botShouldChallenge(0, profileFor(DifficultyEasy), rng()) {
		t.Fatalf("easy bots never challenge")
	}
}
