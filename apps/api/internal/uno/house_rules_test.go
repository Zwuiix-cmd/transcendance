package uno

import (
	"math/rand"
	"testing"
)

func houseState(rules gameRules, top string, hands ...[]string) *gameState {
	state := testState(top, hands...)
	state.Rules = rules
	return state
}

func allHouseRules() gameRules {
	return gameRules{DrawPenalty: 2, StackDrawCards: true, JumpIn: true, SevenZero: true, DrawUntilPlayable: true, ForcePlay: true, ChallengeDrawFour: true}
}

func TestStackingAccumulatesDrawPenalties(t *testing.T) {
	rules := defaultRules()
	rules.StackDrawCards = true
	state := houseState(rules, "R7", []string{"RA2", "R1"}, []string{"GA2", "G1"}, []string{"W4", "B1"}, []string{"Y1", "Y2"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.Phase != PhasePenalty || state.TurnPlayerID != "b" || state.Pending == nil || state.Pending.Count != 2 {
		t.Fatalf("b should face a stackable +2: phase=%s turn=%s pending=%+v", state.Phase, state.TurnPlayerID, state.Pending)
	}
	expectErr(t, state, "b", Action{Type: ActionPlay, CardIndex: 1}, ErrInvalidCard)
	if playable := state.playableIndexes(1); len(playable) != 1 || playable[0] != 0 {
		t.Fatalf("only the +2 should be playable while a penalty is pending, got %v", playable)
	}
	act(t, state, "b", Action{Type: ActionPlay, CardIndex: 0})
	if state.TurnPlayerID != "c" || state.Pending.Count != 4 || state.CurrentColor != "G" {
		t.Fatalf("stacked +2 should move a +4 penalty to c with color G: turn=%s pending=%+v color=%s", state.TurnPlayerID, state.Pending, state.CurrentColor)
	}
	act(t, state, "c", Action{Type: ActionPlay, CardIndex: 0, Color: "Y"})
	if state.TurnPlayerID != "d" || state.Pending.Count != 8 || state.Pending.Card != faceWildFour || state.Phase != PhasePenalty {
		t.Fatalf("a +4 on the stack should raise the penalty to 8 without a challenge: turn=%s phase=%s pending=%+v", state.TurnPlayerID, state.Phase, state.Pending)
	}
	expectErr(t, state, "d", Action{Type: ActionChallenge}, ErrInvalidAction)
	act(t, state, "d", Action{Type: ActionAccept})
	if handSize(state, "d") != 10 || state.TurnPlayerID != "a" || state.Pending != nil {
		t.Fatalf("d should draw 8 and lose the turn: hand=%d turn=%s", handSize(state, "d"), state.TurnPlayerID)
	}
}

func TestDrawTwoCannotStackOnWildDrawFour(t *testing.T) {
	rules := defaultRules()
	rules.StackDrawCards = true
	state := houseState(rules, "R7", []string{"W4", "R1"}, []string{"GA2", "W4", "G1"}, []string{"B1", "B2"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0, Color: "G"})
	if state.Phase != PhaseChallenge {
		t.Fatalf("first +4 stays challengeable, got phase %s", state.Phase)
	}
	expectErr(t, state, "b", Action{Type: ActionPlay, CardIndex: 0}, ErrInvalidCard)
	act(t, state, "b", Action{Type: ActionPlay, CardIndex: 1, Color: "B"})
	if state.TurnPlayerID != "c" || state.Pending.Count != 8 || state.Pending.Stacked != true {
		t.Fatalf("+4 on +4 should stack to 8: turn=%s pending=%+v", state.TurnPlayerID, state.Pending)
	}
}

func TestStackedLastCardPassesWholePenalty(t *testing.T) {
	rules := defaultRules()
	rules.StackDrawCards = true
	state := houseState(rules, "R7", []string{"RA2", "R1"}, []string{"GA2"}, []string{"B1", "B2"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	act(t, state, "b", Action{Type: ActionPlay, CardIndex: 0})
	if state.Status != "finished" || state.WinnerID != "b" || handSize(state, "c") != 6 {
		t.Fatalf("b wins and c must draw the whole stack of 4: status=%s winner=%s c=%d", state.Status, state.WinnerID, handSize(state, "c"))
	}
}

func TestWithoutStackingDrawTwoResolvesImmediately(t *testing.T) {
	state := testState("R7", []string{"RA2", "R1"}, []string{"GA2", "G1"}, []string{"B1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if handSize(state, "b") != 4 || state.TurnPlayerID != "c" || state.Pending != nil {
		t.Fatalf("official rules: b draws 2 and is skipped, got b=%d turn=%s", handSize(state, "b"), state.TurnPlayerID)
	}
}

func TestConfigurableDrawPenalty(t *testing.T) {
	rules := defaultRules()
	rules.DrawPenalty = 4
	state := houseState(rules, "R7", []string{"RA2", "R1"}, []string{"G1"}, []string{"B1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if handSize(state, "b") != 5 {
		t.Fatalf("a +2 under a draw penalty of 4 should deal 4 cards, b holds %d", handSize(state, "b"))
	}
}

func TestChallengeCanBeDisabled(t *testing.T) {
	rules := defaultRules()
	rules.ChallengeDrawFour = false
	state := houseState(rules, "R7", []string{"W4", "R1"}, []string{"G1"}, []string{"B1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0, Color: "B"})
	if state.Phase != PhaseTurn || state.TurnPlayerID != "c" || handSize(state, "b") != 5 || state.Pending != nil {
		t.Fatalf("without challenges the +4 resolves at once: phase=%s turn=%s b=%d", state.Phase, state.TurnPlayerID, handSize(state, "b"))
	}
}

func TestJumpInWithIdenticalCard(t *testing.T) {
	rules := defaultRules()
	rules.JumpIn = true
	state := houseState(rules, "R7", []string{"R1", "R2"}, []string{"G1", "G2"}, []string{"R7", "B5", "W4"})
	expectErr(t, state, "c", Action{Type: ActionPlay, CardIndex: 1}, ErrNotYourTurn)
	expectErr(t, state, "c", Action{Type: ActionDraw}, ErrNotYourTurn)
	if jump := state.jumpInIndexes(2); len(jump) != 1 || jump[0] != 0 {
		t.Fatalf("c should see its R7 as a jump-in candidate, got %v", jump)
	}
	act(t, state, "c", Action{Type: ActionPlay, CardIndex: 0})
	if !state.LastEvent.Jump || state.LastEvent.PlayerID != "c" || state.TurnPlayerID != "a" || handSize(state, "c") != 2 {
		t.Fatalf("jump-in should play c's card and continue from c: event=%+v turn=%s", state.LastEvent, state.TurnPlayerID)
	}
	plain := testState("R7", []string{"R1"}, []string{"G1"}, []string{"R7"})
	expectErr(t, plain, "c", Action{Type: ActionPlay, CardIndex: 0}, ErrNotYourTurn)
}

func TestJumpInIsNotAllowedDuringPendingPhases(t *testing.T) {
	rules := defaultRules()
	rules.JumpIn = true
	state := houseState(rules, "R7", []string{"W4", "R1"}, []string{"G1"}, []string{"W4", "B5"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0, Color: "B"})
	expectErr(t, state, "c", Action{Type: ActionPlay, CardIndex: 0, Color: "R"}, ErrNotYourTurn)
}

func TestSevenSwapsHandsWithChosenPlayer(t *testing.T) {
	rules := defaultRules()
	rules.SevenZero = true
	state := houseState(rules, "R3", []string{"R7", "R1", "R2"}, []string{"G1"}, []string{"B1", "B2", "B3", "B4"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.Phase != PhaseSwap || state.TurnPlayerID != "a" {
		t.Fatalf("playing a 7 should ask a to pick a player: phase=%s turn=%s", state.Phase, state.TurnPlayerID)
	}
	expectErr(t, state, "a", Action{Type: ActionPlay, CardIndex: 0}, ErrInvalidAction)
	expectErr(t, state, "a", Action{Type: ActionSwap, TargetID: "a"}, ErrInvalidAction)
	expectErr(t, state, "b", Action{Type: ActionSwap, TargetID: "a"}, ErrNotYourTurn)
	act(t, state, "a", Action{Type: ActionSwap, TargetID: "c"})
	if handSize(state, "a") != 4 || handSize(state, "c") != 2 || state.Players[2].Hand[0].Face != "R1" || state.TurnPlayerID != "b" || state.Phase != PhaseTurn {
		t.Fatalf("a and c should have swapped hands and b plays next: a=%d c=%d turn=%s", handSize(state, "a"), handSize(state, "c"), state.TurnPlayerID)
	}
	plain := testState("R3", []string{"R7", "R1"}, []string{"G1"})
	act(t, plain, "a", Action{Type: ActionPlay, CardIndex: 0})
	if plain.Phase != PhaseTurn || plain.TurnPlayerID != "b" {
		t.Fatalf("without seven-zero a 7 is a plain number")
	}
}

func TestSevenAsLastCardWinsWithoutSwap(t *testing.T) {
	rules := defaultRules()
	rules.SevenZero = true
	state := houseState(rules, "R3", []string{"R7"}, []string{"G1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.Status != "finished" || state.WinnerID != "a" {
		t.Fatalf("last 7 ends the round, got %s / %s", state.Status, state.WinnerID)
	}
}

func TestZeroRotatesHandsInPlayDirection(t *testing.T) {
	rules := defaultRules()
	rules.SevenZero = true
	state := houseState(rules, "R3", []string{"R0", "R1"}, []string{"G1", "G2", "G3"}, []string{"B1", "B2", "B3", "B4"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.LastEvent.Type != "rotate" || state.TurnPlayerID != "b" {
		t.Fatalf("a 0 should rotate hands then pass the turn: event=%s turn=%s", state.LastEvent.Type, state.TurnPlayerID)
	}
	if handSize(state, "a") != 4 || handSize(state, "b") != 1 || handSize(state, "c") != 3 || state.Players[1].Hand[0].Face != "R1" {
		t.Fatalf("hands should move forward (a→b, b→c, c→a): a=%d b=%d c=%d", handSize(state, "a"), handSize(state, "b"), handSize(state, "c"))
	}
	reversed := houseState(rules, "R3", []string{"R0", "R1"}, []string{"G1", "G2", "G3"}, []string{"B1", "B2", "B3", "B4"})
	reversed.Direction = -1
	act(t, reversed, "a", Action{Type: ActionPlay, CardIndex: 0})
	if handSize(reversed, "a") != 3 || handSize(reversed, "c") != 1 || handSize(reversed, "b") != 4 || reversed.TurnPlayerID != "c" {
		t.Fatalf("reversed rotation should send hands backwards: a=%d b=%d c=%d turn=%s", handSize(reversed, "a"), handSize(reversed, "b"), handSize(reversed, "c"), reversed.TurnPlayerID)
	}
}

func TestDrawUntilPlayableKeepsDrawing(t *testing.T) {
	rules := defaultRules()
	rules.DrawUntilPlayable = true
	state := houseState(rules, "R7", []string{"B1"}, []string{"G1"})
	state.DrawPile = []card{{Face: "R9"}, {Face: "G2"}, {Face: "G3"}, {Face: "Y4"}}
	act(t, state, "a", Action{Type: ActionDraw})
	if handSize(state, "a") != 5 || state.Phase != PhaseDrawn || state.DrawnCard != 4 || state.LastEvent.Count != 4 {
		t.Fatalf("a should draw until the R9 appears: hand=%d phase=%s drawn=%d count=%d", handSize(state, "a"), state.Phase, state.DrawnCard, state.LastEvent.Count)
	}
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 4})
	if state.TurnPlayerID != "b" || state.topCard().Face != "R9" {
		t.Fatalf("the drawn card should be playable")
	}
}

func TestForcePlayForbidsPassing(t *testing.T) {
	rules := defaultRules()
	rules.ForcePlay = true
	state := houseState(rules, "R7", []string{"B1"}, []string{"G1"})
	state.DrawPile = []card{{Face: "R9"}}
	act(t, state, "a", Action{Type: ActionDraw})
	expectErr(t, state, "a", Action{Type: ActionPass}, ErrInvalidAction)
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 1})
	if state.TurnPlayerID != "b" {
		t.Fatalf("the forced card should have been played")
	}
}

func TestSnapshotExposesRulesResultsAndJumpIn(t *testing.T) {
	rules := defaultRules()
	rules.JumpIn = true
	state := houseState(rules, "R7", []string{"R1"}, []string{"R7", "G2"})
	snapshot := publicSnapshot(state, "b")
	if !snapshot.Rules.JumpIn || len(snapshot.Playable) != 1 || snapshot.Playable[0] != 0 {
		t.Fatalf("the off-turn viewer should see its jump-in card: %+v playable=%v", snapshot.Rules, snapshot.Playable)
	}
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	snapshot = publicSnapshot(state, "b")
	if len(snapshot.Results) != 2 || !snapshot.Results[0].Winner || snapshot.Results[0].PointsEarned != 9 || snapshot.Results[1].CardsRemaining != 2 {
		t.Fatalf("finished snapshot should carry ranked results: %+v", snapshot.Results)
	}
}

func TestBotsRespectEveryHouseRule(t *testing.T) {
	for _, difficulty := range []string{DifficultyEasy, DifficultyIntermediate, DifficultyExpert} {
		finished := 0
		for seed := int64(1); seed <= 25; seed++ {
			random := rand.New(rand.NewSource(seed))
			state := &gameState{GameID: "house", Status: "active", Version: 1, TurnNumber: 1, BotDifficulty: difficulty, Rules: allHouseRules()}
			for index := 0; index < 4; index++ {
				state.Players = append(state.Players, player{ID: string(rune('a' + index)), Name: string(rune('A' + index)), Bot: true})
			}
			deck := buildDeck(false)
			random.Shuffle(len(deck), func(left, right int) { deck[left], deck[right] = deck[right], deck[left] })
			deal(state, deck, 7, random)
			profile := profileFor(difficulty)
			swaps, stacks, jumps := 0, 0, 0
			for steps := 0; state.Status == "active" && steps < 3000; steps++ {
				if index, action, ok := state.botSideCandidate(profile, random); ok && random.Float64() < 0.5 {
					state.botActing = true
					if err := state.apply(index, action, random); err != nil {
						t.Fatalf("%s side action %s rejected: %v", difficulty, action.Type, err)
					}
					state.botActing = false
					if action.Type == ActionPlay {
						jumps++
					}
					state.TurnNumber++
					continue
				}
				botIndex := state.playerIndex(state.TurnPlayerID)
				action := state.botDecision(botIndex, profile, random)
				pendingBefore := state.Pending != nil
				state.botActing = true
				err := state.apply(botIndex, action, random)
				state.botActing = false
				if err != nil {
					t.Fatalf("%s bot made an illegal %s under house rules: %v", difficulty, action.Type, err)
				}
				if action.Type == ActionSwap {
					swaps++
				}
				if pendingBefore && action.Type == ActionPlay {
					stacks++
				}
				state.TurnNumber++
			}
			if state.Status == "finished" {
				finished++
			}
			if difficulty == DifficultyExpert && seed == 1 {
				t.Logf("expert house-rule game: swaps=%d stacks=%d jumps=%d", swaps, stacks, jumps)
			}
		}
		minimum := 23
		if difficulty == DifficultyEasy {
			minimum = 15
		}
		if finished < minimum {
			t.Fatalf("%s bots finished only %d/25 house-rule games", difficulty, finished)
		}
	}
}
