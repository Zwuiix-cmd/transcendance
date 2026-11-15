package uno

import (
	"errors"
	"math/rand"
	"testing"
)

func testState(top string, hands ...[]string) *gameState {
	state := &gameState{GameID: "g", Status: "active", Version: 1, Direction: 1, Phase: PhaseTurn, DrawnCard: -1, TurnNumber: 1, Rules: defaultRules()}
	for index, hand := range hands {
		current := player{ID: string(rune('a' + index)), Name: string(rune('A' + index))}
		for _, face := range hand {
			current.Hand = append(current.Hand, card{Face: face})
		}
		state.Players = append(state.Players, current)
	}
	state.DiscardPile = []card{{Face: top}}
	state.CurrentColor = colorOf(top)
	for value := 0; value < 40; value++ {
		state.DrawPile = append(state.DrawPile, card{Face: "G5"})
	}
	state.TurnPlayerID = state.Players[0].ID
	return state
}

func rng() *rand.Rand { return rand.New(rand.NewSource(1)) }

func act(t *testing.T, state *gameState, playerID string, action Action) {
	t.Helper()
	if err := state.apply(state.playerIndex(playerID), action, rng()); err != nil {
		t.Fatalf("%s %s: unexpected error %v", playerID, action.Type, err)
	}
}

func expectErr(t *testing.T, state *gameState, playerID string, action Action, want error) {
	t.Helper()
	err := state.apply(state.playerIndex(playerID), action, rng())
	if !errors.Is(err, want) {
		t.Fatalf("%s %s: want %v, got %v", playerID, action.Type, want, err)
	}
}

func handSize(state *gameState, playerID string) int {
	return len(state.Players[state.playerIndex(playerID)].Hand)
}

func TestDeckMatchesOfficialComposition(t *testing.T) {
	deck := buildDeck(false)
	if len(deck) != 108 {
		t.Fatalf("deck has %d cards, want 108", len(deck))
	}
	counts := map[string]int{}
	for _, current := range deck {
		counts[current.Face]++
	}
	for _, color := range colors {
		if counts[color+"0"] != 1 {
			t.Fatalf("%s0 appears %d times, want 1", color, counts[color+"0"])
		}
		for _, symbol := range []string{"1", "5", "9", "S", "R", "A2"} {
			if counts[color+symbol] != 2 {
				t.Fatalf("%s%s appears %d times, want 2", color, symbol, counts[color+symbol])
			}
		}
	}
	if counts["WC"] != 4 || counts["W4"] != 4 {
		t.Fatalf("wild counts %d/%d, want 4/4", counts["WC"], counts["W4"])
	}
	if len(buildDeck(true)) != 216 {
		t.Fatalf("double deck should hold 216 cards")
	}
}

func TestMatchingRules(t *testing.T) {
	state := testState("R7", []string{"R2", "B7", "B3", "WC", "W4", "BS"}, []string{"G1"})
	for _, face := range []string{"R2", "B7", "WC", "W4"} {
		if !state.canPlay(face) {
			t.Fatalf("%s should be playable on R7", face)
		}
	}
	for _, face := range []string{"B3", "BS", "G1"} {
		if state.canPlay(face) {
			t.Fatalf("%s should not be playable on R7", face)
		}
	}
	expectErr(t, state, "a", Action{Type: ActionPlay, CardIndex: 2}, ErrInvalidCard)
	expectErr(t, state, "a", Action{Type: ActionPlay, CardIndex: 9}, ErrInvalidCard)
	expectErr(t, state, "b", Action{Type: ActionPlay, CardIndex: 0}, ErrNotYourTurn)
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 1})
	if state.CurrentColor != "B" || state.topCard().Face != "B7" || state.TurnPlayerID != "b" {
		t.Fatalf("after playing B7: color=%s top=%s turn=%s", state.CurrentColor, state.topCard().Face, state.TurnPlayerID)
	}
	if got := state.playableIndexes(1); len(got) != 0 {
		t.Fatalf("player b should have no playable card, got %v", got)
	}
}

func TestActionSymbolMatchesAcrossColors(t *testing.T) {
	state := testState("RS", []string{"BS", "G1"}, []string{"G1", "G2"}, []string{"Y1", "Y2"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.CurrentColor != "B" || state.TurnPlayerID != "c" {
		t.Fatalf("Skip on Skip should change color to B and skip b, got color=%s turn=%s", state.CurrentColor, state.TurnPlayerID)
	}
}

func TestDrawTwoMakesNextPlayerDrawAndSkip(t *testing.T) {
	state := testState("R7", []string{"RA2", "R1"}, []string{"G1"}, []string{"Y1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if handSize(state, "b") != 3 {
		t.Fatalf("b should hold 3 cards, has %d", handSize(state, "b"))
	}
	if state.TurnPlayerID != "c" {
		t.Fatalf("turn should skip to c, got %s", state.TurnPlayerID)
	}
}

func TestReverseChangesDirection(t *testing.T) {
	state := testState("R7", []string{"RR", "R1"}, []string{"G1"}, []string{"Y1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.Direction != -1 || state.TurnPlayerID != "c" {
		t.Fatalf("reverse should hand the turn to c, got direction=%d turn=%s", state.Direction, state.TurnPlayerID)
	}
}

func TestReverseActsAsSkipWithTwoPlayers(t *testing.T) {
	state := testState("R7", []string{"RR", "R1"}, []string{"G1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.TurnPlayerID != "a" {
		t.Fatalf("two-player reverse should let a play again, got %s", state.TurnPlayerID)
	}
}

func TestWildRequiresColor(t *testing.T) {
	state := testState("R7", []string{"WC", "R1"}, []string{"G1"})
	expectErr(t, state, "a", Action{Type: ActionPlay, CardIndex: 0}, ErrInvalidAction)
	expectErr(t, state, "a", Action{Type: ActionPlay, CardIndex: 0, Color: "purple"}, ErrInvalidAction)
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0, Color: "g"})
	if state.CurrentColor != "G" || state.TurnPlayerID != "b" {
		t.Fatalf("wild should set color G and pass the turn, got %s/%s", state.CurrentColor, state.TurnPlayerID)
	}
}

func TestWildDrawFourAccepted(t *testing.T) {
	state := testState("R7", []string{"W4", "B1"}, []string{"G1"}, []string{"Y1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0, Color: "B"})
	if state.Phase != PhaseChallenge || state.TurnPlayerID != "b" || state.Pending == nil || state.Pending.Count != 4 {
		t.Fatalf("W4 should open a challenge window for b, got phase=%s turn=%s", state.Phase, state.TurnPlayerID)
	}
	expectErr(t, state, "b", Action{Type: ActionDraw}, ErrInvalidAction)
	expectErr(t, state, "b", Action{Type: ActionPlay, CardIndex: 0}, ErrInvalidAction)
	act(t, state, "b", Action{Type: ActionAccept})
	if handSize(state, "b") != 5 || state.TurnPlayerID != "c" || state.Phase != PhaseTurn {
		t.Fatalf("accepting should draw 4 and skip b, got hand=%d turn=%s phase=%s", handSize(state, "b"), state.TurnPlayerID, state.Phase)
	}
}

func TestWildDrawFourBluffIsPunished(t *testing.T) {
	state := testState("R7", []string{"W4", "R1"}, []string{"G1"}, []string{"Y1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0, Color: "B"})
	act(t, state, "b", Action{Type: ActionChallenge})
	if handSize(state, "a") != 5 {
		t.Fatalf("bluffer should draw 4, holds %d", handSize(state, "a"))
	}
	if handSize(state, "b") != 1 || state.TurnPlayerID != "b" || state.Phase != PhaseTurn {
		t.Fatalf("challenger should keep the turn with 1 card, got hand=%d turn=%s", handSize(state, "b"), state.TurnPlayerID)
	}
	if state.CurrentColor != "B" {
		t.Fatalf("chosen color should stay in play, got %s", state.CurrentColor)
	}
}

func TestWildDrawFourFailedChallengeCostsSix(t *testing.T) {
	state := testState("R7", []string{"W4", "B1"}, []string{"G1"}, []string{"Y1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0, Color: "B"})
	act(t, state, "b", Action{Type: ActionChallenge})
	if handSize(state, "b") != 7 || state.TurnPlayerID != "c" {
		t.Fatalf("failed challenge should cost 6 cards and the turn, got hand=%d turn=%s", handSize(state, "b"), state.TurnPlayerID)
	}
}

func TestDrawnPlayableCardCanBePlayedOrPassed(t *testing.T) {
	state := testState("G7", []string{"R1"}, []string{"B1"})
	act(t, state, "a", Action{Type: ActionDraw})
	if state.Phase != PhaseDrawn || state.TurnPlayerID != "a" || state.DrawnCard != 1 {
		t.Fatalf("drawing a playable card should keep the turn, got phase=%s turn=%s drawn=%d", state.Phase, state.TurnPlayerID, state.DrawnCard)
	}
	if got := state.playableIndexes(0); len(got) != 1 || got[0] != 1 {
		t.Fatalf("only the drawn card should be playable, got %v", got)
	}
	expectErr(t, state, "a", Action{Type: ActionDraw}, ErrInvalidAction)
	act(t, state, "a", Action{Type: ActionPass})
	if state.TurnPlayerID != "b" || state.Phase != PhaseTurn || handSize(state, "a") != 2 {
		t.Fatalf("pass should move to b, got turn=%s phase=%s", state.TurnPlayerID, state.Phase)
	}
	expectErr(t, state, "b", Action{Type: ActionPass}, ErrInvalidAction)
	act(t, state, "b", Action{Type: ActionDraw})
	act(t, state, "b", Action{Type: ActionPlay, CardIndex: 1})
	if state.topCard().Face != "G5" || state.TurnPlayerID != "a" {
		t.Fatalf("drawn card should be playable immediately, got top=%s turn=%s", state.topCard().Face, state.TurnPlayerID)
	}
}

func TestDrawnUnplayableCardEndsTurn(t *testing.T) {
	state := testState("R7", []string{"B1"}, []string{"B1"})
	act(t, state, "a", Action{Type: ActionDraw})
	if state.TurnPlayerID != "b" || handSize(state, "a") != 2 {
		t.Fatalf("unplayable draw should end the turn, got turn=%s hand=%d", state.TurnPlayerID, handSize(state, "a"))
	}
}

func TestUnoCallAndCatch(t *testing.T) {
	state := testState("R7", []string{"R1", "B2"}, []string{"G1", "G2"}, []string{"Y1", "Y2"})
	expectErr(t, state, "b", Action{Type: ActionUno}, ErrInvalidAction)
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.Catchable != "a" {
		t.Fatalf("a forgot UNO and should be catchable, got %q", state.Catchable)
	}
	expectErr(t, state, "a", Action{Type: ActionCatch, TargetID: "a"}, ErrInvalidAction)
	act(t, state, "c", Action{Type: ActionCatch, TargetID: "a"})
	if handSize(state, "a") != 3 || state.Catchable != "" {
		t.Fatalf("caught player should draw 2, got hand=%d catchable=%q", handSize(state, "a"), state.Catchable)
	}
	expectErr(t, state, "c", Action{Type: ActionCatch, TargetID: "a"}, ErrInvalidAction)
}

func TestUnoCallProtectsPlayer(t *testing.T) {
	state := testState("R7", []string{"R1", "B2"}, []string{"G1", "G2"})
	act(t, state, "a", Action{Type: ActionUno})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.Catchable != "" || !state.Players[0].SaidUno {
		t.Fatalf("player who said UNO must not be catchable")
	}
	expectErr(t, state, "b", Action{Type: ActionCatch, TargetID: "a"}, ErrInvalidAction)
}

func TestUnoFlagOnPlayAndLateCall(t *testing.T) {
	state := testState("R7", []string{"R1", "B2"}, []string{"G1", "G2"}, []string{"Y1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	act(t, state, "a", Action{Type: ActionUno})
	if state.Catchable != "" {
		t.Fatalf("late UNO before being caught should be safe")
	}
	state = testState("R7", []string{"R1", "B2"}, []string{"G1", "G2"}, []string{"Y1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0, Uno: true})
	if state.Catchable != "" {
		t.Fatalf("UNO flag on play should protect the player")
	}
}

func TestCatchWindowClosesWhenNextPlayerActs(t *testing.T) {
	state := testState("R7", []string{"R1", "B2"}, []string{"R3", "G2", "G4"}, []string{"Y1"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	act(t, state, "b", Action{Type: ActionPlay, CardIndex: 0})
	if state.Catchable != "" {
		t.Fatalf("catch window should close once b has played")
	}
	expectErr(t, state, "c", Action{Type: ActionCatch, TargetID: "a"}, ErrInvalidAction)
}

func TestDrawingResetsUnoCall(t *testing.T) {
	state := testState("R7", []string{"B1", "B2"}, []string{"G1", "G2"})
	act(t, state, "a", Action{Type: ActionUno})
	act(t, state, "a", Action{Type: ActionDraw})
	if state.Players[0].SaidUno {
		t.Fatalf("drawing back above one card must reset the UNO call")
	}
}

func TestLastCardDrawTwoStillPenalizesAndScores(t *testing.T) {
	state := testState("R7", []string{"RA2"}, []string{"G1", "GS", "WC"}, []string{"W4", "Y9"})
	act(t, state, "a", Action{Type: ActionPlay, CardIndex: 0})
	if state.Status != "finished" || state.WinnerID != "a" {
		t.Fatalf("a should win, got status=%s winner=%s", state.Status, state.WinnerID)
	}
	if handSize(state, "b") != 5 {
		t.Fatalf("b must draw 2 from the final Draw Two, holds %d", handSize(state, "b"))
	}
	wantB := 1 + 20 + 50 + 5 + 5
	wantC := 50 + 9
	if state.Players[1].HandValue != wantB || state.Players[2].HandValue != wantC {
		t.Fatalf("hand values b=%d c=%d, want %d/%d", state.Players[1].HandValue, state.Players[2].HandValue, wantB, wantC)
	}
	if len(state.Results) != 3 || state.Results[0].PlayerID != "a" || state.Results[0].PointsEarned != wantB+wantC || state.Results[1].PlayerID != "c" || state.Results[1].Rank != 2 || state.Results[2].HandPoints != wantB {
		t.Fatalf("results should rank the winner first then the lowest hands: %+v", state.Results)
	}
	expectErr(t, state, "b", Action{Type: ActionDraw}, ErrNotYourTurn)
}

func TestBotsCannotAct(t *testing.T) {
	state := testState("R7", []string{"R1"}, []string{"R2"})
	state.Players[1].Bot = true
	state.TurnPlayerID = "b"
	expectErr(t, state, "b", Action{Type: ActionDraw}, ErrInvalidAction)
}

func TestRecycleReshufflesDiscardWhenDrawPileEmpty(t *testing.T) {
	state := testState("R7", []string{"B1"}, []string{"B2"})
	state.DrawPile = nil
	for value := 0; value < 10; value++ {
		state.DiscardPile = append(state.DiscardPile, card{Face: "Y1"})
	}
	top := state.topCard().Face
	act(t, state, "a", Action{Type: ActionDraw})
	if handSize(state, "a") != 2 || state.topCard().Face != top || len(state.DiscardPile) != 1 || len(state.DrawPile) != 9 {
		t.Fatalf("discard pile should be reshuffled under the top card: hand=%d discard=%d draw=%d", handSize(state, "a"), len(state.DiscardPile), len(state.DrawPile))
	}
}

func dealWithTop(top string, players int) *gameState {
	state := &gameState{GameID: "g", Status: "active"}
	for index := 0; index < players; index++ {
		state.Players = append(state.Players, player{ID: string(rune('a' + index))})
	}
	deck := make([]card, 0, 40)
	for value := 0; value < 30; value++ {
		deck = append(deck, card{Face: "G5"})
	}
	deck = append(deck, card{Face: top})
	for value := 0; value < 7*players; value++ {
		deck = append(deck, card{Face: "G5"})
	}
	deal(state, deck, 7, rng())
	return state
}

func TestDealFirstCardEffects(t *testing.T) {
	state := dealWithTop("R7", 3)
	if state.TurnPlayerID != "a" || state.CurrentColor != "R" || handSize(state, "a") != 7 {
		t.Fatalf("plain first card: turn=%s color=%s hand=%d", state.TurnPlayerID, state.CurrentColor, handSize(state, "a"))
	}
	state = dealWithTop("RS", 3)
	if state.TurnPlayerID != "b" {
		t.Fatalf("Skip first card should skip a, got %s", state.TurnPlayerID)
	}
	state = dealWithTop("RR", 3)
	if state.TurnPlayerID != "c" || state.Direction != -1 {
		t.Fatalf("Reverse first card should start with the dealer going right, got turn=%s direction=%d", state.TurnPlayerID, state.Direction)
	}
	state = dealWithTop("RA2", 3)
	if handSize(state, "a") != 9 || state.TurnPlayerID != "b" {
		t.Fatalf("Draw Two first card should make a draw 2 and skip, got hand=%d turn=%s", handSize(state, "a"), state.TurnPlayerID)
	}
	state = dealWithTop("WC", 3)
	if state.CurrentColor != "" || !state.canPlay("Y3") {
		t.Fatalf("Wild first card should let the first player pick any color")
	}
	state = dealWithTop("W4", 3)
	if state.topCard().Face == "W4" {
		t.Fatalf("Wild Draw Four must never start the discard pile")
	}
}

func TestSnapshotHidesOtherHandsAndListsPlayable(t *testing.T) {
	state := testState("R7", []string{"R1", "B2", "WC"}, []string{"G1", "G2"})
	snapshot := publicSnapshot(state, "a")
	if len(snapshot.Players[0].Hand) != 3 || snapshot.Players[1].Hand != nil {
		t.Fatalf("viewer must only see their own hand")
	}
	if len(snapshot.Playable) != 2 || snapshot.Playable[0] != 0 || snapshot.Playable[1] != 2 {
		t.Fatalf("playable indexes should be [0 2], got %v", snapshot.Playable)
	}
	if other := publicSnapshot(state, "b"); other.Playable != nil {
		t.Fatalf("player not on turn should not get playable indexes")
	}
}

func TestStuckPlayerIsSkippedWhenNothingCanBeDrawn(t *testing.T) {
	state := testState("R7", []string{"B2", "B3"}, []string{"B1"}, []string{"R9"})
	state.DrawPile = nil
	act(t, state, "a", Action{Type: ActionDraw})
	if state.TurnPlayerID != "c" || len(state.Skipped) != 1 || state.Skipped[0] != "b" {
		t.Fatalf("b holds no red card and nothing can be drawn, the turn should skip to c: turn=%s skipped=%v", state.TurnPlayerID, state.Skipped)
	}
	act(t, state, "c", Action{Type: ActionPlay, CardIndex: 0})
	if state.Status != "finished" || state.WinnerID != "c" {
		t.Fatalf("c should win by emptying their hand: status=%s winner=%s", state.Status, state.WinnerID)
	}
}

func TestEveryoneStuckEndsRoundOnLowestHand(t *testing.T) {
	state := testState("R7", []string{"B5", "B6"}, []string{"G9"}, []string{"Y2", "Y3"})
	state.DrawPile = nil
	act(t, state, "a", Action{Type: ActionDraw})
	if state.Status != "finished" || state.WinnerID != "c" {
		t.Fatalf("with every seat stuck the lowest hand (c, 5 points) should win: status=%s winner=%s skipped=%v", state.Status, state.WinnerID, state.Skipped)
	}
	if state.Results[0].PlayerID != "c" || state.Results[0].PointsEarned != 11+9 {
		t.Fatalf("winner should collect the other hands, got %+v", state.Results)
	}
}
