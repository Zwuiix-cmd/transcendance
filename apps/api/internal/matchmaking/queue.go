package matchmaking

import (
	"sort"
	"sync"
	"time"

	"card-arena/api/internal/model"
	"github.com/google/uuid"
)

type ticket struct {
	id        string
	userID    string
	mode      string
	elo       int
	createdAt time.Time
	matchedAt time.Time
	players   []ticket
}

// Queue is an in-memory ranked/unranked queue. Games are created by the game
// engine after a match; the REST API only exposes the queue state.
type Queue struct {
	mu      sync.Mutex
	tickets map[string]*ticket
	byUser  map[string]string
}

func New() *Queue { return &Queue{tickets: map[string]*ticket{}, byUser: map[string]string{}} }

func (queue *Queue) Join(userID, mode string, elo int) model.MatchmakingStatus {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if oldID, ok := queue.byUser[userID]; ok {
		delete(queue.tickets, oldID)
	}
	now := time.Now().UTC()
	entry := &ticket{id: uuid.NewString(), userID: userID, mode: mode, elo: elo, createdAt: now}
	queue.tickets[entry.id] = entry
	queue.byUser[userID] = entry.id
	queue.tryMatch(mode)
	return queue.statusLocked(userID)
}

func (queue *Queue) Leave(userID string) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.removeLocked(userID)
}

func (queue *Queue) Status(userID string) model.MatchmakingStatus {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.statusLocked(userID)
}

func (queue *Queue) removeLocked(userID string) {
	if id, ok := queue.byUser[userID]; ok {
		delete(queue.tickets, id)
		delete(queue.byUser, userID)
	}
}

func (queue *Queue) tryMatch(mode string) {
	entries := make([]*ticket, 0)
	for _, entry := range queue.tickets {
		if entry.mode == mode && entry.matchedAt.IsZero() {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].createdAt.Before(entries[j].createdAt) })
	for len(entries) >= 2 {
		base := entries[0]
		gap := searchGap(time.Since(base.createdAt))
		match := []*ticket{base}
		for _, candidate := range entries[1:] {
			if len(match) >= 4 {
				break
			}
			if abs(base.elo-candidate.elo) <= gap {
				match = append(match, candidate)
			}
		}
		if len(match) < 2 {
			return
		}
		matchedAt := time.Now().UTC()
		players := make([]ticket, len(match))
		for i, item := range match {
			players[i] = *item
		}
		for _, item := range match {
			item.matchedAt = matchedAt
			item.players = players
		}
		entries = entries[len(match):]
	}
}

func (queue *Queue) statusLocked(userID string) model.MatchmakingStatus {
	id, ok := queue.byUser[userID]
	if !ok {
		return model.MatchmakingStatus{Status: "not_waiting", Players: []model.MatchmakingPlayer{}}
	}
	entry := queue.tickets[id]
	if entry == nil {
		return model.MatchmakingStatus{Status: "not_waiting", Players: []model.MatchmakingPlayer{}}
	}
	waited := int64(time.Since(entry.createdAt).Seconds())
	if waited < 0 {
		waited = 0
	}
	gap := searchGap(time.Duration(waited) * time.Second)
	mode := entry.mode
	ticketID := entry.id
	created := entry.createdAt.Format(time.RFC3339Nano)
	players := make([]model.MatchmakingPlayer, 0, len(entry.players))
	for _, item := range entry.players {
		players = append(players, model.MatchmakingPlayer{UserID: item.userID, Mode: item.mode, Elo: item.elo, JoinedAt: item.createdAt.Format(time.RFC3339Nano)})
	}
	status := "waiting"
	matched := !entry.matchedAt.IsZero()
	if matched {
		status = "matched"
	}
	var minElo, maxElo, eloGap *int
	if entry.mode == "ranked" {
		min := entry.elo - gap
		if min < 0 {
			min = 0
		}
		max := entry.elo + gap
		minElo, maxElo, eloGap = &min, &max, &gap
	}
	return model.MatchmakingStatus{Status: status, Mode: &mode, Queued: true, Matched: matched, QueueSize: queue.sizeMode(entry.mode), Position: queue.position(entry), WaitedSeconds: waited, EstimatedWaitSeconds: 20, TicketID: &ticketID, CreatedAt: &created, Players: players, SearchEloMin: minElo, SearchEloMax: maxElo, SearchEloGap: eloGap}
}

func (queue *Queue) sizeMode(mode string) int {
	count := 0
	for _, entry := range queue.tickets {
		if entry.mode == mode && entry.matchedAt.IsZero() {
			count++
		}
	}
	return count
}
func (queue *Queue) position(entry *ticket) int {
	position := 1
	for _, candidate := range queue.tickets {
		if candidate.mode == entry.mode && candidate.matchedAt.IsZero() && candidate.createdAt.Before(entry.createdAt) {
			position++
		}
	}
	return position
}
func searchGap(waited time.Duration) int {
	gap := 200 + int(waited/(5*time.Second))*75
	if gap > 800 {
		return 800
	}
	return gap
}
func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
func intPtr(value int) *int { return &value }
