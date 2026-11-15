package uno

import (
	"log/slog"
	"math/rand"
	"time"
)

const (
	DifficultyEasy         = "easy"
	DifficultyIntermediate = "intermediate"
	DifficultyExpert       = "expert"
)

type botProfile struct {
	Name        string
	ThinkMin    time.Duration
	ThinkMax    time.Duration
	CatchMin    time.Duration
	CatchMax    time.Duration
	UnoRate     float64
	LateUnoRate float64
	CatchRate   float64
	JumpRate    float64
	BlunderRate float64
	Level       int
}

var botProfiles = map[string]botProfile{
	DifficultyEasy:         {Name: DifficultyEasy, ThinkMin: 1600 * time.Millisecond, ThinkMax: 3000 * time.Millisecond, CatchMin: 3 * time.Second, CatchMax: 5 * time.Second, UnoRate: 0.55, LateUnoRate: 0.25, CatchRate: 0, JumpRate: 0, BlunderRate: 0.18, Level: 0},
	DifficultyIntermediate: {Name: DifficultyIntermediate, ThinkMin: 1000 * time.Millisecond, ThinkMax: 2000 * time.Millisecond, CatchMin: 1800 * time.Millisecond, CatchMax: 3200 * time.Millisecond, UnoRate: 0.88, LateUnoRate: 0.6, CatchRate: 0.5, JumpRate: 0.5, BlunderRate: 0, Level: 1},
	DifficultyExpert:       {Name: DifficultyExpert, ThinkMin: 700 * time.Millisecond, ThinkMax: 1400 * time.Millisecond, CatchMin: 700 * time.Millisecond, CatchMax: 1300 * time.Millisecond, UnoRate: 1, LateUnoRate: 1, CatchRate: 1, JumpRate: 0.9, BlunderRate: 0, Level: 2},
}

func profileFor(difficulty string) botProfile {
	if profile, ok := botProfiles[difficulty]; ok {
		return profile
	}
	return botProfiles[DifficultyIntermediate]
}

const (
	expertRollouts     = 240
	expertRolloutSteps = 400
)

type botTimers struct {
	turn *time.Timer
	side *time.Timer
}

func (engine *Engine) cancelBots(gameID string) {
	timers, ok := engine.timers[gameID]
	if !ok {
		return
	}
	if timers.turn != nil {
		timers.turn.Stop()
	}
	if timers.side != nil {
		timers.side.Stop()
	}
	delete(engine.timers, gameID)
}

func (engine *Engine) botDelay(min, max time.Duration) time.Duration {
	if max <= min {
		return time.Duration(float64(min) * engine.botSpeed)
	}
	jitter := time.Duration(engine.random.Int63n(int64(max - min)))
	return time.Duration(float64(min+jitter) * engine.botSpeed)
}

func (engine *Engine) scheduleBots(state *gameState) {
	engine.cancelBots(state.GameID)
	if engine.stopped || state.Status != "active" {
		return
	}
	profile := profileFor(state.BotDifficulty)
	timers := &botTimers{}
	gameID, version := state.GameID, state.Version
	if turnIndex := state.playerIndex(state.TurnPlayerID); turnIndex >= 0 && state.Players[turnIndex].Bot {
		timers.turn = time.AfterFunc(engine.botDelay(profile.ThinkMin, profile.ThinkMax), func() { engine.runBot(gameID, version, false) })
	}
	if _, side, ok := state.botSideCandidate(profile, engine.random); ok {
		rate := profile.CatchRate
		switch side.Type {
		case ActionUno:
			rate = profile.LateUnoRate
		case ActionPlay:
			rate = profile.JumpRate
		}
		if engine.random.Float64() < rate {
			timers.side = time.AfterFunc(engine.botDelay(profile.CatchMin, profile.CatchMax), func() { engine.runBot(gameID, version, true) })
		}
	}
	if timers.turn != nil || timers.side != nil {
		engine.timers[gameID] = timers
	}
}

func (engine *Engine) runBot(gameID string, version int64, side bool) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.stopped {
		return
	}
	tx, err := engine.database.Begin()
	if err != nil {
		slog.Warn("bot move skipped", "game", gameID, "error", err)
		return
	}
	defer tx.Rollback()
	state, err := engine.loadOrCreate(tx, gameID)
	if err != nil || state.Version != version || state.Status != "active" {
		return
	}
	profile := profileFor(state.BotDifficulty)
	var botIndex int
	var action Action
	if side {
		candidate, sideAction, ok := state.botSideCandidate(profile, engine.random)
		if !ok {
			return
		}
		botIndex, action = candidate, sideAction
	} else {
		botIndex = state.playerIndex(state.TurnPlayerID)
		if botIndex < 0 || !state.Players[botIndex].Bot {
			return
		}
		action = state.botDecision(botIndex, profile, engine.random)
	}
	state.botActing = true
	err = state.apply(botIndex, action, engine.random)
	if err != nil && !side {
		fallbacks := []Action{{Type: ActionDraw}, {Type: ActionPass}, {Type: ActionAccept}, {Type: ActionSwap, TargetID: state.Players[state.nextIndex(botIndex, 1)].ID}}
		for _, fallback := range fallbacks {
			if err = state.apply(botIndex, fallback, engine.random); err == nil {
				break
			}
		}
	}
	state.botActing = false
	if err != nil {
		slog.Warn("bot move rejected", "game", gameID, "bot", state.Players[botIndex].ID, "action", action.Type, "error", err)
		if side {
			return
		}
	}
	if err = engine.commit(tx, state); err != nil {
		slog.Warn("bot move not saved", "game", gameID, "error", err)
		retry := time.AfterFunc(2*time.Second, func() { engine.runBot(gameID, version, side) })
		engine.timers[gameID] = &botTimers{turn: retry}
	}
}

// botSideCandidate returns an off-turn move a bot may make: calling a late UNO
// for itself, catching a player who forgot to call it, or jumping in with an
// identical card when the house rule allows it.
func (state *gameState) botSideCandidate(profile botProfile, random *rand.Rand) (int, Action, bool) {
	if state.Catchable != "" {
		target := state.playerIndex(state.Catchable)
		if target < 0 {
			return -1, Action{}, false
		}
		if state.Players[target].Bot {
			return target, Action{Type: ActionUno}, true
		}
		for index, current := range state.Players {
			if current.Bot && !current.Finished && index != target {
				return index, Action{Type: ActionCatch, TargetID: state.Catchable}, true
			}
		}
		return -1, Action{}, false
	}
	if !state.Rules.JumpIn || profile.JumpRate <= 0 {
		return -1, Action{}, false
	}
	for index, current := range state.Players {
		if !current.Bot || current.Finished || current.ID == state.TurnPlayerID {
			continue
		}
		if indexes := state.jumpInIndexes(index); len(indexes) > 0 {
			return index, state.botPlay(index, indexes[0], profile, random), true
		}
	}
	return -1, Action{}, false
}

func (state *gameState) botDecision(botIndex int, profile botProfile, random *rand.Rand) Action {
	switch state.Phase {
	case PhaseSwap:
		return Action{Type: ActionSwap, TargetID: state.botSwapTarget(botIndex, profile.Level, random)}
	case PhaseChallenge, PhasePenalty:
		if stack := state.playableIndexes(botIndex); len(stack) > 0 && (profile.Level >= 1 || random.Float64() < 0.5) {
			return state.botPlay(botIndex, state.botStackChoice(botIndex, stack), profile, random)
		}
		if state.Phase == PhaseChallenge && state.botShouldChallenge(botIndex, profile, random) {
			return Action{Type: ActionChallenge}
		}
		return Action{Type: ActionAccept}
	case PhaseDrawn:
		if !state.Rules.ForcePlay && profile.BlunderRate > 0 && random.Float64() < profile.BlunderRate {
			return Action{Type: ActionPass}
		}
		if len(state.playableIndexes(botIndex)) == 0 {
			return Action{Type: ActionPass}
		}
		if profile.Level >= 2 {
			if action, ok := state.expertSearch(botIndex, []int{state.DrawnCard}, profile, random); ok {
				return action
			}
		}
		return state.botPlay(botIndex, state.DrawnCard, profile, random)
	}
	legal := state.playableIndexes(botIndex)
	if len(legal) == 0 {
		return Action{Type: ActionDraw}
	}
	if profile.BlunderRate > 0 && random.Float64() < profile.BlunderRate {
		return Action{Type: ActionDraw}
	}
	if profile.Level == 0 {
		return state.botPlay(botIndex, legal[random.Intn(len(legal))], profile, random)
	}
	if profile.Level >= 2 {
		if action, ok := state.expertSearch(botIndex, legal, profile, random); ok {
			return action
		}
	}
	best, bestScore := -1, -1e9
	for _, index := range legal {
		score := state.scorePlay(botIndex, index, profile.Level)
		score += random.Float64() * 0.5
		if score > bestScore {
			best, bestScore = index, score
		}
	}
	if best < 0 || bestScore < -500 {
		return Action{Type: ActionDraw}
	}
	return state.botPlay(botIndex, best, profile, random)
}

func (state *gameState) botSwapTarget(botIndex, level int, random *rand.Rand) string {
	candidates := make([]int, 0, len(state.Players))
	for index, current := range state.Players {
		if index != botIndex && !current.Finished {
			candidates = append(candidates, index)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	if level == 0 {
		return state.Players[candidates[random.Intn(len(candidates))]].ID
	}
	best := candidates[0]
	for _, index := range candidates[1:] {
		if len(state.Players[index].Hand) < len(state.Players[best].Hand) {
			best = index
		}
	}
	return state.Players[best].ID
}

func (state *gameState) botStackChoice(botIndex int, stack []int) int {
	hand := state.Players[botIndex].Hand
	for _, index := range stack {
		if hand[index].Face != faceWildFour {
			return index
		}
	}
	return stack[0]
}

func (state *gameState) botPlay(botIndex, cardIndex int, profile botProfile, random *rand.Rand) Action {
	hand := state.Players[botIndex].Hand
	action := Action{Type: ActionPlay, CardIndex: cardIndex}
	if isWild(hand[cardIndex].Face) {
		action.Color = state.botColor(botIndex, cardIndex, profile.Level, random)
	}
	if len(hand) == 2 && random.Float64() < profile.UnoRate {
		action.Uno = true
	}
	return action
}

func (state *gameState) botColor(botIndex, skipIndex, level int, random *rand.Rand) string {
	if level == 0 {
		return colors[random.Intn(len(colors))]
	}
	counts := state.colorCounts(botIndex, skipIndex)
	next := state.nextIndex(botIndex, 1)
	best, bestScore := "", -1.0
	for _, color := range colors {
		score := float64(counts[color]) * 2
		if level >= 2 {
			if note, ok := state.Lacks[state.Players[next].ID]; ok && note.Color == color && state.TurnNumber-note.Turn <= 8 {
				score += 3
			}
		}
		score += random.Float64() * 0.1
		if score > bestScore {
			best, bestScore = color, score
		}
	}
	return best
}

func (state *gameState) colorCounts(playerIndex, skipIndex int) map[string]int {
	counts := map[string]int{}
	for index, current := range state.Players[playerIndex].Hand {
		if index == skipIndex {
			continue
		}
		if color := colorOf(current.Face); color != "" {
			counts[color]++
		}
	}
	return counts
}

// scorePlay rates one legal card for an intermediate (level 1) or expert
// (level 2) bot. Higher is better; anything below -500 means "never".
func (state *gameState) scorePlay(botIndex, cardIndex, level int) float64 {
	hand := state.Players[botIndex].Hand
	face := hand[cardIndex].Face
	color := colorOf(face)
	symbol := symbolOf(face)
	remaining := len(hand) - 1
	counts := state.colorCounts(botIndex, cardIndex)
	next := state.nextIndex(botIndex, 1)
	nextCount := len(state.Players[next].Hand)
	keepsWild := false
	for index, current := range hand {
		if index != cardIndex && isWild(current.Face) {
			keepsWild = true
			break
		}
	}
	score := 0.0
	nextLacks := ""
	if note, ok := state.Lacks[state.Players[next].ID]; ok && state.TurnNumber-note.Turn <= 8 {
		nextLacks = note.Color
	}
	switch {
	case face == faceWildFour:
		bluff := state.CurrentColor != "" && counts[state.CurrentColor] > 0
		if bluff && (level < 2 || (nextCount > 2 && remaining > 2)) {
			return -1000
		}
		score = -20
		if nextCount <= 2 {
			score += 60
		}
		if remaining == 0 {
			score += 40
		}
	case face == faceWild:
		score = -12
		if remaining == 0 {
			score += 40
		}
		if level >= 2 && nextCount <= 2 && counts[state.CurrentColor] == 0 {
			score += 10
		}
	case symbol == symbolDraw2 || symbol == symbolSkip || symbol == symbolRevers:
		score = 8 + float64(counts[color])*2.5
		if nextCount <= 2 {
			score += 30
			if symbol == symbolDraw2 {
				score += 4
			}
		}
		if level >= 2 && state.activeCount() == 2 {
			score += 25
		}
		if symbol == symbolRevers && state.activeCount() > 2 {
			previous := state.nextIndex(botIndex, len(state.Players)-1)
			if level >= 2 {
				if len(state.Players[previous].Hand) <= 2 {
					score -= 25
				} else if len(state.Players[previous].Hand) > nextCount {
					score += 6
				}
			}
			if nextCount <= 2 {
				score -= 20
			}
		}
		if level >= 1 && nextCount <= 3 && symbol != symbolRevers {
			score += 8
		}
	default:
		score = 10 + float64(cardValue(face))*0.3 + float64(counts[color])*2.5
	}
	if state.Rules.SevenZero && level >= 1 && (symbol == symbolSeven || symbol == symbolZero) && state.activeCount() > 1 {
		incoming := remaining
		if symbol == symbolSeven {
			for index, current := range state.Players {
				if index != botIndex && !current.Finished && len(current.Hand) < incoming {
					incoming = len(current.Hand)
				}
			}
		} else {
			incoming = len(state.Players[state.nextIndex(botIndex, state.activeCount()-1)].Hand)
		}
		if incoming < remaining {
			score += float64(remaining-incoming) * 4
		} else if incoming > remaining {
			score -= float64(incoming-remaining) * 3
		}
	}
	if level >= 2 {
		if color != "" && color == nextLacks {
			score += 6
		}
		if keepsWild && remaining == 1 {
			score += 15
		}
		if remaining >= 5 {
			score += float64(cardValue(face)) * 0.2
		}
	}
	return score
}

func (state *gameState) botShouldChallenge(botIndex int, profile botProfile, random *rand.Rand) bool {
	if state.Pending == nil || profile.Level == 0 {
		return false
	}
	fromIndex := state.playerIndex(state.Pending.FromID)
	if fromIndex < 0 {
		return false
	}
	fromCount := len(state.Players[fromIndex].Hand)
	if profile.Level == 1 {
		if fromCount >= 6 {
			return random.Float64() < 0.35
		}
		return random.Float64() < 0.1
	}
	if note, ok := state.Lacks[state.Pending.FromID]; ok && note.Color == state.Pending.PrevColor && state.TurnNumber-note.Turn <= 6 {
		return false
	}
	probability := state.matchProbability(botIndex, state.Pending.PrevColor, fromCount)
	myCount := len(state.Players[botIndex].Hand)
	return probability >= 0.85 && (myCount <= 2 || fromCount >= 7)
}

// matchProbability estimates how likely a hand of handSize unseen cards holds
// at least one card of the given color, using every card the bot can see.
func (state *gameState) matchProbability(botIndex int, color string, handSize int) float64 {
	if color == "" || handSize <= 0 {
		return 0
	}
	total := len(state.DrawPile) + len(state.DiscardPile)
	for _, current := range state.Players {
		total += len(current.Hand)
	}
	decks := (total + 107) / 108
	if decks < 1 {
		decks = 1
	}
	colorTotal := 25 * decks
	seen := 0
	colorSeen := 0
	for _, current := range state.DiscardPile {
		seen++
		if colorOf(current.Face) == color {
			colorSeen++
		}
	}
	for _, current := range state.Players[botIndex].Hand {
		seen++
		if colorOf(current.Face) == color {
			colorSeen++
		}
	}
	unseen := total - seen
	unseenColor := colorTotal - colorSeen
	if unseen <= 0 || unseenColor <= 0 {
		return 0
	}
	if unseenColor > unseen {
		unseenColor = unseen
	}
	none := 1.0
	for draw := 0; draw < handSize && draw < unseen; draw++ {
		none *= float64(unseen-unseenColor-draw) / float64(unseen-draw)
		if none <= 0 {
			return 1
		}
	}
	return 1 - none
}

// expertSearch evaluates every candidate move with Monte Carlo rollouts. The
// hidden hands and the draw pile are reshuffled for each rollout so the bot
// only ever reasons about cards it is allowed to know, then every seat plays
// the intermediate policy until the round ends.
func (state *gameState) expertSearch(botIndex int, legal []int, profile botProfile, random *rand.Rand) (Action, bool) {
	candidates := make([]Action, 0, len(legal)+1)
	for _, index := range legal {
		if state.scorePlay(botIndex, index, 2) < -500 {
			continue
		}
		candidates = append(candidates, state.botPlay(botIndex, index, botProfiles[DifficultyExpert], random))
	}
	if state.Phase == PhaseDrawn && !state.Rules.ForcePlay {
		candidates = append(candidates, Action{Type: ActionPass})
	}
	if len(candidates) == 0 {
		return Action{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	botID := state.Players[botIndex].ID
	rollout := botProfiles[DifficultyIntermediate]
	totals := make([]float64, len(candidates))
	for run := 0; run < expertRollouts; run++ {
		seed := random.Int63()
		base := state.sampleHidden(botIndex, rand.New(rand.NewSource(seed)))
		for candidateIndex, candidate := range candidates {
			sample := base.clone()
			sample.botActing = true
			runRandom := rand.New(rand.NewSource(seed ^ 0x5DEECE66D))
			if err := sample.apply(botIndex, candidate, runRandom); err != nil {
				totals[candidateIndex] -= 1
				continue
			}
			totals[candidateIndex] += sample.playout(botID, rollout, runRandom)
		}
	}
	bestIndex, bestValue := 0, -1e9
	for candidateIndex, candidate := range candidates {
		value := totals[candidateIndex] / expertRollouts
		if candidate.Type == ActionPlay {
			value += state.scorePlay(botIndex, candidate.CardIndex, 2) * 0.0005
		}
		if value > bestValue {
			bestIndex, bestValue = candidateIndex, value
		}
	}
	return candidates[bestIndex], true
}

func (state *gameState) clone() *gameState {
	copied := *state
	copied.Players = make([]player, len(state.Players))
	for index := range state.Players {
		copied.Players[index] = state.Players[index]
		copied.Players[index].Hand = append([]card(nil), state.Players[index].Hand...)
	}
	copied.DrawPile = append([]card(nil), state.DrawPile...)
	copied.DiscardPile = append([]card(nil), state.DiscardPile...)
	if state.Pending != nil {
		pending := *state.Pending
		copied.Pending = &pending
	}
	copied.Lacks = nil
	copied.Results = nil
	return &copied
}

func (state *gameState) sampleHidden(botIndex int, random *rand.Rand) *gameState {
	clone := state.clone()
	hidden := clone.DrawPile
	for index := range state.Players {
		if index == botIndex {
			continue
		}
		hidden = append(hidden, state.Players[index].Hand...)
	}
	random.Shuffle(len(hidden), func(left, right int) { hidden[left], hidden[right] = hidden[right], hidden[left] })
	for index := range state.Players {
		if index == botIndex {
			continue
		}
		size := len(state.Players[index].Hand)
		clone.Players[index].Hand = append([]card(nil), hidden[:size]...)
		hidden = hidden[size:]
	}
	clone.DrawPile = hidden
	for index := range state.Players {
		note, ok := state.Lacks[state.Players[index].ID]
		if !ok || index == botIndex || state.TurnNumber-note.Turn > len(state.Players) {
			continue
		}
		clone.respectLack(index, note.Color)
	}
	if clone.Pending != nil {
		fromIndex := clone.playerIndex(clone.Pending.FromID)
		clone.Pending.HadMatch = fromIndex >= 0 && fromIndex != botIndex && clone.colorCounts(fromIndex, -1)[clone.Pending.PrevColor] > 0
	}
	return clone
}

// respectLack swaps cards of a color the player recently failed to play out
// of their sampled hand, so the sample matches what the bot has observed.
func (state *gameState) respectLack(playerIndex int, color string) {
	hand := state.Players[playerIndex].Hand
	pile := 0
	for index := range hand {
		if colorOf(hand[index].Face) != color {
			continue
		}
		for pile < len(state.DrawPile) && colorOf(state.DrawPile[pile].Face) == color {
			pile++
		}
		if pile >= len(state.DrawPile) {
			return
		}
		hand[index], state.DrawPile[pile] = state.DrawPile[pile], hand[index]
		pile++
	}
}

// playout finishes a sampled round with every seat on the intermediate policy
// and scores it from the point of view of botID: 1 for a win, otherwise a
// small negative value that grows with the cards left in hand.
func (state *gameState) playout(botID string, policy botProfile, random *rand.Rand) float64 {
	for step := 0; step < expertRolloutSteps && state.Status == "active"; step++ {
		if index, action, ok := state.botSideCandidate(policy, random); ok && random.Float64() < 0.5 {
			_ = state.apply(index, action, random)
			state.TurnNumber++
			continue
		}
		turn := state.playerIndex(state.TurnPlayerID)
		if turn < 0 {
			break
		}
		action := state.botDecision(turn, policy, random)
		if err := state.apply(turn, action, random); err != nil {
			if err = state.apply(turn, Action{Type: ActionDraw}, random); err != nil {
				break
			}
		}
		state.TurnNumber++
	}
	if state.WinnerID == botID {
		return 1
	}
	index := state.playerIndex(botID)
	if index < 0 {
		return 0
	}
	return -0.02 * float64(len(state.Players[index].Hand))
}
