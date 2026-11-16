package uno

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

const (
	PhaseTurn      = "turn"
	PhaseDrawn     = "drawn"
	PhaseChallenge = "challenge"
	PhasePenalty   = "penalty"
	PhaseSwap      = "swap"

	ActionPlay      = "play"
	ActionDraw      = "draw"
	ActionPass      = "pass"
	ActionAccept    = "accept"
	ActionChallenge = "challenge"
	ActionUno       = "uno"
	ActionCatch     = "catch"
	ActionSwap      = "swap"

	faceWild     = "WC"
	faceWildFour = "W4"
	symbolSkip   = "S"
	symbolRevers = "R"
	symbolDraw2  = "A2"
	symbolSeven  = "7"
	symbolZero   = "0"

	unoPenalty           = 2
	drawTwoPenalty       = 2
	drawFourPenalty      = 4
	failedChallengeExtra = 2
)

var colors = []string{"R", "G", "B", "Y"}

type Action struct {
	Type      string `json:"type"`
	CardIndex int    `json:"cardIndex"`
	Color     string `json:"color"`
	Uno       bool   `json:"uno"`
	TargetID  string `json:"targetId"`
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

func defaultRules() gameRules {
	return gameRules{DrawPenalty: drawTwoPenalty, ChallengeDrawFour: true}
}

func (rules gameRules) drawTwo() int {
	if rules.DrawPenalty < 1 {
		return drawTwoPenalty
	}
	return rules.DrawPenalty
}

func (rules gameRules) penaltyFor(face string) int {
	switch {
	case face == faceWildFour:
		return drawFourPenalty
	case symbolOf(face) == symbolDraw2:
		return rules.drawTwo()
	}
	return 0
}

type pendingPenalty struct {
	FromID    string `json:"fromId"`
	Count     int    `json:"count"`
	Card      string `json:"card"`
	Stacked   bool   `json:"stacked"`
	HadMatch  bool   `json:"hadMatch"`
	PrevColor string `json:"prevColor"`
}

func (pending *pendingPenalty) accepts(face string) bool {
	if pending == nil {
		return false
	}
	if face == faceWildFour {
		return true
	}
	return pending.Card != faceWildFour && symbolOf(face) == symbolDraw2
}

func (pending *pendingPenalty) challengeable() bool {
	return pending != nil && pending.Card == faceWildFour && !pending.Stacked
}

type lackNote struct {
	Color string `json:"color"`
	Turn  int    `json:"turn"`
}

func (state *gameState) noteLack(playerID, color string) {
	if color == "" {
		return
	}
	if state.Lacks == nil {
		state.Lacks = map[string]lackNote{}
	}
	state.Lacks[playerID] = lackNote{Color: color, Turn: state.TurnNumber}
}

func (state *gameState) clearLack(playerID, color string) {
	if note, ok := state.Lacks[playerID]; ok && (color == "" || note.Color == color) {
		delete(state.Lacks, playerID)
	}
}

type lastEvent struct {
	Type     string `json:"type"`
	PlayerID string `json:"playerId,omitempty"`
	TargetID string `json:"targetId,omitempty"`
	Card     string `json:"card,omitempty"`
	Color    string `json:"color,omitempty"`
	Count    int    `json:"count,omitempty"`
	Jump     bool   `json:"jump,omitempty"`
}

func buildDeck(multiple bool) []card {
	decks := 1
	if multiple {
		decks = 2
	}
	deck := make([]card, 0, decks*108)
	for set := 0; set < decks; set++ {
		for _, color := range colors {
			deck = append(deck, card{Face: color + "0"})
			for copy := 0; copy < 2; copy++ {
				for value := 1; value <= 9; value++ {
					deck = append(deck, card{Face: fmt.Sprintf("%s%d", color, value)})
				}
				deck = append(deck, card{Face: color + symbolDraw2}, card{Face: color + symbolRevers}, card{Face: color + symbolSkip})
			}
		}
		for copy := 0; copy < 4; copy++ {
			deck = append(deck, card{Face: faceWild}, card{Face: faceWildFour})
		}
	}
	return deck
}

func isWild(face string) bool { return face == faceWild || face == faceWildFour }

func colorOf(face string) string {
	if len(face) == 0 || isWild(face) {
		return ""
	}
	return string(face[0])
}

func symbolOf(face string) string {
	if isWild(face) || len(face) < 2 {
		return face
	}
	return face[1:]
}

func cardValue(face string) int {
	switch {
	case isWild(face):
		return 50
	case symbolOf(face) == symbolSkip, symbolOf(face) == symbolRevers, symbolOf(face) == symbolDraw2:
		return 20
	default:
		return int(face[1] - '0')
	}
}

func handValue(hand []card) int {
	total := 0
	for _, current := range hand {
		total += cardValue(current.Face)
	}
	return total
}

func normalizeColor(color string) (string, bool) {
	color = strings.ToUpper(strings.TrimSpace(color))
	for _, candidate := range colors {
		if candidate == color {
			return color, true
		}
	}
	return "", false
}

func (state *gameState) topCard() card {
	return state.DiscardPile[len(state.DiscardPile)-1]
}

func (state *gameState) playerIndex(id string) int {
	for index := range state.Players {
		if state.Players[index].ID == id {
			return index
		}
	}
	return -1
}

func (state *gameState) activeCount() int {
	count := 0
	for _, current := range state.Players {
		if !current.Finished {
			count++
		}
	}
	return count
}

func (state *gameState) canPlay(face string) bool {
	if state.CurrentColor == "" {
		return true
	}
	return isWild(face) || colorOf(face) == state.CurrentColor || symbolOf(face) == symbolOf(state.topCard().Face)
}

func (state *gameState) playableIndexes(playerIndex int) []int {
	hand := state.Players[playerIndex].Hand
	indexes := make([]int, 0, len(hand))
	switch state.Phase {
	case PhaseChallenge, PhasePenalty:
		if !state.Rules.StackDrawCards {
			return indexes
		}
		for index, current := range hand {
			if state.Pending.accepts(current.Face) {
				indexes = append(indexes, index)
			}
		}
		return indexes
	case PhaseSwap:
		return indexes
	case PhaseDrawn:
		if state.DrawnCard >= 0 && state.DrawnCard < len(hand) && state.canPlay(hand[state.DrawnCard].Face) {
			indexes = append(indexes, state.DrawnCard)
		}
		return indexes
	}
	for index, current := range hand {
		if state.canPlay(current.Face) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func (state *gameState) jumpInIndexes(playerIndex int) []int {
	if !state.Rules.JumpIn || state.Status != "active" || state.Phase != PhaseTurn || len(state.DiscardPile) == 0 {
		return nil
	}
	player := state.Players[playerIndex]
	if player.Finished || player.ID == state.TurnPlayerID {
		return nil
	}
	top := state.topCard().Face
	if isWild(top) {
		return nil
	}
	var indexes []int
	for index, current := range player.Hand {
		if current.Face == top {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func (state *gameState) nextIndex(from, steps int) int {
	count := len(state.Players)
	index := from
	for moved := 0; moved < steps; moved++ {
		for attempt := 0; attempt < count; attempt++ {
			index = ((index+state.Direction)%count + count) % count
			if !state.Players[index].Finished {
				break
			}
		}
	}
	return index
}

func (state *gameState) setTurn(index int) {
	state.TurnPlayerID = state.Players[index].ID
	state.Phase = PhaseTurn
	state.DrawnCard = -1
	state.Pending = nil
}

func (state *gameState) refreshUnoFlags(playerIndex int) {
	player := &state.Players[playerIndex]
	if len(player.Hand) > 1 {
		player.SaidUno = false
	}
	if state.Catchable == player.ID && len(player.Hand) > 1 {
		state.Catchable = ""
	}
}

func (state *gameState) drawCards(playerIndex, count int, random *rand.Rand) int {
	drawn := 0
	for ; drawn < count; drawn++ {
		if len(state.DrawPile) == 0 {
			state.recycle(random)
		}
		if len(state.DrawPile) == 0 {
			break
		}
		state.Players[playerIndex].Hand = append(state.Players[playerIndex].Hand, state.DrawPile[len(state.DrawPile)-1])
		state.DrawPile = state.DrawPile[:len(state.DrawPile)-1]
		state.Players[playerIndex].CardsDrawn++
	}
	state.refreshUnoFlags(playerIndex)
	return drawn
}

func (state *gameState) recycle(random *rand.Rand) {
	if len(state.DiscardPile) <= 1 {
		return
	}
	top := state.topCard()
	state.DrawPile = append(state.DrawPile, state.DiscardPile[:len(state.DiscardPile)-1]...)
	state.DiscardPile = []card{top}
	random.Shuffle(len(state.DrawPile), func(left, right int) {
		state.DrawPile[left], state.DrawPile[right] = state.DrawPile[right], state.DrawPile[left]
	})
}

func deal(state *gameState, deck []card, handSize int, random *rand.Rand) {
	if state.Rules.DrawPenalty == 0 {
		state.Rules = defaultRules()
	}
	state.Direction = 1
	state.Phase = PhaseTurn
	state.DrawnCard = -1
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
		if candidate.Face == faceWildFour {
			deck = append([]card{candidate}, deck...)
			continue
		}
		state.DiscardPile = append(state.DiscardPile, candidate)
		break
	}
	state.DrawPile = deck
	first := state.topCard().Face
	state.CurrentColor = colorOf(first)
	dealer := len(state.Players) - 1
	starter := 0
	state.LastEvent = lastEvent{Type: "start", Card: first}
	switch {
	case first == faceWild:
		state.CurrentColor = ""
	case symbolOf(first) == symbolRevers:
		state.Direction = -1
		starter = dealer
	case symbolOf(first) == symbolSkip:
		starter = state.nextIndex(0, 1)
	case symbolOf(first) == symbolDraw2:
		state.drawCards(0, state.Rules.drawTwo(), random)
		starter = state.nextIndex(0, 1)
	}
	if len(state.Players) == 1 {
		starter = 0
	}
	state.setTurn(starter)
	state.skipStuckTurns()
}

func (state *gameState) apply(actorIndex int, action Action, random *rand.Rand) error {
	actor := &state.Players[actorIndex]
	if actor.Bot && !state.botActing {
		return fmt.Errorf("%w: only a real player can act", ErrInvalidAction)
	}
	switch action.Type {
	case ActionUno:
		return state.declareUno(actorIndex)
	case ActionCatch:
		return state.catchPlayer(actorIndex, action.TargetID, random)
	}
	jump := false
	if state.TurnPlayerID != actor.ID {
		if action.Type != ActionPlay || !state.canJumpIn(actorIndex, action.CardIndex) {
			return ErrNotYourTurn
		}
		state.TurnPlayerID = actor.ID
		state.Phase = PhaseTurn
		state.DrawnCard = -1
		jump = true
	}
	var err error
	switch action.Type {
	case ActionPlay:
		err = state.play(actorIndex, action.CardIndex, action.Color, action.Uno, jump, random)
	case ActionDraw:
		err = state.draw(actorIndex, random)
	case ActionPass:
		err = state.pass(actorIndex)
	case ActionAccept:
		err = state.acceptPenalty(actorIndex, random)
	case ActionChallenge:
		err = state.challenge(actorIndex, random)
	case ActionSwap:
		err = state.swapHands(actorIndex, action.TargetID)
	default:
		err = fmt.Errorf("%w: unknown action %q", ErrInvalidAction, action.Type)
	}
	if err != nil {
		return err
	}
	if state.Catchable != "" && state.Catchable != actor.ID {
		state.Catchable = ""
	}
	state.skipStuckTurns()
	return nil
}

func (state *gameState) canJumpIn(playerIndex, cardIndex int) bool {
	for _, index := range state.jumpInIndexes(playerIndex) {
		if index == cardIndex {
			return true
		}
	}
	return false
}

// skipStuckTurns passes the turn of any player who has no playable card while
// the draw pile is empty and the discard pile holds nothing to recycle. When
// every seat is stuck the round ends and the lowest hand wins.
func (state *gameState) skipStuckTurns() {
	state.Skipped = nil
	if state.Status != "active" || state.Phase != PhaseTurn {
		return
	}
	for attempts := 0; attempts < len(state.Players); attempts++ {
		if len(state.DrawPile) > 0 || len(state.DiscardPile) > 1 {
			return
		}
		current := state.playerIndex(state.TurnPlayerID)
		if current < 0 || len(state.playableIndexes(current)) > 0 {
			return
		}
		state.Skipped = append(state.Skipped, state.Players[current].ID)
		state.setTurn(state.nextIndex(current, 1))
	}
	best, bestValue := -1, 0
	for index := range state.Players {
		if state.Players[index].Finished {
			continue
		}
		value := handValue(state.Players[index].Hand)
		if best < 0 || value < bestValue {
			best, bestValue = index, value
		}
	}
	if best >= 0 {
		state.finishRound(best, "", nil)
	}
}

func (state *gameState) play(playerIndex, cardIndex int, color string, sayUno, jump bool, random *rand.Rand) error {
	if state.Phase == PhaseSwap {
		return fmt.Errorf("%w: choose a player to swap hands with first", ErrInvalidAction)
	}
	hand := state.Players[playerIndex].Hand
	if cardIndex < 0 || cardIndex >= len(hand) {
		return fmt.Errorf("%w: card index is out of range", ErrInvalidCard)
	}
	if state.Phase == PhaseDrawn && cardIndex != state.DrawnCard {
		return fmt.Errorf("%w: only the drawn card can be played this turn", ErrInvalidCard)
	}
	played := hand[cardIndex]
	if state.Pending != nil {
		if !state.Rules.StackDrawCards {
			if state.Phase == PhaseChallenge {
				return fmt.Errorf("%w: accept or challenge the Wild Draw Four first", ErrInvalidAction)
			}
			return fmt.Errorf("%w: accept the draw penalty first", ErrInvalidAction)
		}
		if !state.Pending.accepts(played.Face) {
			return fmt.Errorf("%w: only a matching draw card can be stacked on this penalty", ErrInvalidCard)
		}
	} else if !state.canPlay(played.Face) {
		return fmt.Errorf("%w: card does not match the active color or symbol", ErrInvalidCard)
	}
	previousColor := state.CurrentColor
	if isWild(played.Face) {
		chosen, ok := normalizeColor(color)
		if !ok {
			return fmt.Errorf("%w: a color is required for a wild card", ErrInvalidAction)
		}
		state.CurrentColor = chosen
	} else {
		state.CurrentColor = colorOf(played.Face)
	}
	hadMatch := false
	for index, current := range hand {
		if index != cardIndex && previousColor != "" && colorOf(current.Face) == previousColor {
			hadMatch = true
			break
		}
	}
	if played.Face == faceWildFour {
		state.Players[playerIndex].DrawFoursPlayed++
	}
	remaining := make([]card, 0, len(hand)-1)
	remaining = append(remaining, hand[:cardIndex]...)
	remaining = append(remaining, hand[cardIndex+1:]...)
	state.Players[playerIndex].Hand = remaining
	state.DiscardPile = append(state.DiscardPile, played)
	state.clearLack(state.Players[playerIndex].ID, colorOf(played.Face))
	state.LastAction = ActionPlay
	state.LastEvent = lastEvent{Type: ActionPlay, PlayerID: state.Players[playerIndex].ID, Card: played.Face, Color: state.CurrentColor, Jump: jump}
	if len(remaining) == 1 {
		if sayUno {
			state.Players[playerIndex].SaidUno = true
			state.Players[playerIndex].UnosCalled++
		} else if !state.Players[playerIndex].SaidUno {
			state.Catchable = state.Players[playerIndex].ID
		}
	}
	if len(remaining) == 0 {
		state.finishRound(playerIndex, played.Face, random)
		return nil
	}
	symbol := symbolOf(played.Face)
	next := state.nextIndex(playerIndex, 1)
	switch {
	case played.Face == faceWildFour || symbol == symbolDraw2:
		state.queuePenalty(playerIndex, next, played.Face, hadMatch, previousColor, random)
	case played.Face == faceWild:
		state.setTurn(next)
	case state.Rules.SevenZero && symbol == symbolSeven && state.activeCount() > 1:
		state.Phase = PhaseSwap
		state.DrawnCard = -1
		state.Pending = nil
	case state.Rules.SevenZero && symbol == symbolZero && state.activeCount() > 1:
		state.rotateHands()
		state.LastEvent.Type = "rotate"
		state.setTurn(next)
	case symbol == symbolSkip:
		state.LastEvent.TargetID = state.Players[next].ID
		state.setTurn(state.nextIndex(next, 1))
	case symbol == symbolRevers:
		state.Direction = -state.Direction
		if state.activeCount() == 2 {
			state.LastEvent.TargetID = state.Players[next].ID
			state.setTurn(playerIndex)
		} else {
			state.setTurn(state.nextIndex(playerIndex, 1))
		}
	default:
		state.setTurn(next)
	}
	return nil
}

func (state *gameState) queuePenalty(playerIndex, next int, face string, hadMatch bool, previousColor string, random *rand.Rand) {
	count := state.Rules.penaltyFor(face)
	stacked := state.Pending != nil
	if stacked {
		count += state.Pending.Count
	}
	state.LastEvent.TargetID = state.Players[next].ID
	state.LastEvent.Count = count
	fromID := state.Players[playerIndex].ID
	switch {
	case face == faceWildFour && state.Rules.ChallengeDrawFour && !stacked:
		state.Pending = &pendingPenalty{FromID: fromID, Count: count, Card: face, HadMatch: hadMatch, PrevColor: previousColor}
		state.TurnPlayerID = state.Players[next].ID
		state.Phase = PhaseChallenge
		state.DrawnCard = -1
	case state.Rules.StackDrawCards:
		state.Pending = &pendingPenalty{FromID: fromID, Count: count, Card: face, Stacked: stacked}
		state.TurnPlayerID = state.Players[next].ID
		state.Phase = PhasePenalty
		state.DrawnCard = -1
	default:
		state.Pending = nil
		state.LastEvent.Count = state.drawCards(next, count, random)
		state.setTurn(state.nextIndex(next, 1))
	}
}

func (state *gameState) draw(playerIndex int, random *rand.Rand) error {
	if state.Phase != PhaseTurn {
		return fmt.Errorf("%w: you cannot draw right now", ErrInvalidAction)
	}
	state.noteLack(state.Players[playerIndex].ID, state.CurrentColor)
	drawn := 0
	for {
		if state.drawCards(playerIndex, 1, random) == 0 {
			break
		}
		drawn++
		hand := state.Players[playerIndex].Hand
		if !state.Rules.DrawUntilPlayable || state.canPlay(hand[len(hand)-1].Face) {
			break
		}
	}
	state.LastAction = ActionDraw
	state.LastEvent = lastEvent{Type: ActionDraw, PlayerID: state.Players[playerIndex].ID, Count: drawn}
	hand := state.Players[playerIndex].Hand
	if drawn >= 1 && state.canPlay(hand[len(hand)-1].Face) {
		state.Phase = PhaseDrawn
		state.DrawnCard = len(hand) - 1
		return nil
	}
	state.setTurn(state.nextIndex(playerIndex, 1))
	return nil
}

func (state *gameState) pass(playerIndex int) error {
	if state.Phase != PhaseDrawn {
		return fmt.Errorf("%w: you can only pass after drawing a playable card", ErrInvalidAction)
	}
	if state.Rules.ForcePlay {
		return fmt.Errorf("%w: force play is on, the drawn card must be played", ErrInvalidAction)
	}
	state.LastAction = ActionPass
	state.LastEvent = lastEvent{Type: ActionPass, PlayerID: state.Players[playerIndex].ID}
	state.setTurn(state.nextIndex(playerIndex, 1))
	return nil
}

func (state *gameState) acceptPenalty(playerIndex int, random *rand.Rand) error {
	if (state.Phase != PhaseChallenge && state.Phase != PhasePenalty) || state.Pending == nil {
		return fmt.Errorf("%w: there is no penalty to accept", ErrInvalidAction)
	}
	count := state.drawCards(playerIndex, state.Pending.Count, random)
	state.LastAction = ActionAccept
	state.LastEvent = lastEvent{Type: ActionAccept, PlayerID: state.Players[playerIndex].ID, Count: count}
	state.setTurn(state.nextIndex(playerIndex, 1))
	return nil
}

func (state *gameState) challenge(playerIndex int, random *rand.Rand) error {
	if state.Phase != PhaseChallenge || !state.Pending.challengeable() {
		return fmt.Errorf("%w: there is nothing to challenge", ErrInvalidAction)
	}
	pending := *state.Pending
	fromIndex := state.playerIndex(pending.FromID)
	state.LastAction = ActionChallenge
	if pending.HadMatch && fromIndex >= 0 {
		count := state.drawCards(fromIndex, pending.Count, random)
		state.LastEvent = lastEvent{Type: "challenge_won", PlayerID: state.Players[playerIndex].ID, TargetID: pending.FromID, Count: count}
		state.setTurn(playerIndex)
		return nil
	}
	count := state.drawCards(playerIndex, pending.Count+failedChallengeExtra, random)
	state.LastEvent = lastEvent{Type: "challenge_lost", PlayerID: state.Players[playerIndex].ID, TargetID: pending.FromID, Count: count}
	state.setTurn(state.nextIndex(playerIndex, 1))
	return nil
}

func (state *gameState) swapHands(playerIndex int, targetID string) error {
	if state.Phase != PhaseSwap {
		return fmt.Errorf("%w: there is no hand swap pending", ErrInvalidAction)
	}
	targetIndex := state.playerIndex(targetID)
	if targetIndex < 0 || targetIndex == playerIndex || state.Players[targetIndex].Finished {
		return fmt.Errorf("%w: choose another player at the table to swap with", ErrInvalidAction)
	}
	state.Players[playerIndex].Hand, state.Players[targetIndex].Hand = state.Players[targetIndex].Hand, state.Players[playerIndex].Hand
	state.refreshUnoFlags(playerIndex)
	state.refreshUnoFlags(targetIndex)
	state.clearLack(state.Players[playerIndex].ID, "")
	state.clearLack(targetID, "")
	state.LastAction = ActionSwap
	state.LastEvent = lastEvent{Type: ActionSwap, PlayerID: state.Players[playerIndex].ID, TargetID: targetID}
	state.setTurn(state.nextIndex(playerIndex, 1))
	return nil
}

func (state *gameState) rotateHands() {
	hands := map[int][]card{}
	for index := range state.Players {
		if state.Players[index].Finished {
			continue
		}
		hands[state.nextIndex(index, 1)] = state.Players[index].Hand
	}
	for index, hand := range hands {
		state.Players[index].Hand = hand
		state.refreshUnoFlags(index)
		state.clearLack(state.Players[index].ID, "")
	}
}

func (state *gameState) declareUno(playerIndex int) error {
	player := &state.Players[playerIndex]
	switch {
	case len(player.Hand) == 2 && state.TurnPlayerID == player.ID:
		player.SaidUno = true
	case len(player.Hand) == 1 && state.Catchable == player.ID:
		player.SaidUno = true
		state.Catchable = ""
	default:
		return fmt.Errorf("%w: you can only call UNO with two cards on your turn or one card before being caught", ErrInvalidAction)
	}
	player.UnosCalled++
	state.LastEvent = lastEvent{Type: ActionUno, PlayerID: player.ID}
	return nil
}

func (state *gameState) catchPlayer(actorIndex int, targetID string, random *rand.Rand) error {
	targetIndex := state.playerIndex(targetID)
	if targetIndex < 0 || targetIndex == actorIndex {
		return fmt.Errorf("%w: invalid catch target", ErrInvalidAction)
	}
	if state.Catchable != targetID || len(state.Players[targetIndex].Hand) != 1 {
		return fmt.Errorf("%w: this player cannot be caught", ErrInvalidAction)
	}
	count := state.drawCards(targetIndex, unoPenalty, random)
	state.Players[targetIndex].UnoPenalties++
	state.Catchable = ""
	state.LastAction = ActionCatch
	state.LastEvent = lastEvent{Type: ActionCatch, PlayerID: state.Players[actorIndex].ID, TargetID: targetID, Count: count}
	return nil
}

func (state *gameState) finishRound(winnerIndex int, lastFace string, random *rand.Rand) {
	if random != nil {
		penalty := state.Rules.penaltyFor(lastFace)
		if state.Pending != nil {
			penalty += state.Pending.Count
		}
		if penalty > 0 {
			state.drawCards(state.nextIndex(winnerIndex, 1), penalty, random)
		}
	}
	total := 0
	results := make([]PlayerResult, 0, len(state.Players))
	for index := range state.Players {
		value := handValue(state.Players[index].Hand)
		state.Players[index].HandValue = value
		if index != winnerIndex {
			total += value
		}
		current := state.Players[index]
		results = append(results, PlayerResult{PlayerID: current.ID, Name: current.Name, Bot: current.Bot, Winner: index == winnerIndex, CardsRemaining: len(current.Hand), HandPoints: value, UnosCalled: current.UnosCalled, UnoPenalties: current.UnoPenalties, DrawFoursPlayed: current.DrawFoursPlayed, CardsDrawn: current.CardsDrawn})
	}
	sort.SliceStable(results, func(left, right int) bool {
		if results[left].Winner != results[right].Winner {
			return results[left].Winner
		}
		return results[left].HandPoints < results[right].HandPoints
	})
	for index := range results {
		results[index].Rank = index + 1
		if results[index].Winner {
			results[index].PointsEarned = total
		}
	}
	state.Results = results
	state.Players[winnerIndex].Finished = true
	state.WinnerID = state.Players[winnerIndex].ID
	state.Status = "finished"
	state.Catchable = ""
	state.Pending = nil
	state.Phase = PhaseTurn
	state.DrawnCard = -1
	state.LastEvent = lastEvent{Type: "finish", PlayerID: state.WinnerID, Card: lastFace, Count: total}
}
