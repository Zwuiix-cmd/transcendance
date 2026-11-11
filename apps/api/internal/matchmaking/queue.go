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
	lastSent  fingerprint
	announced bool
}

type fingerprint struct {
	matched   bool
	position  int
	queueSize int
	gap       int
}

// Notifier receives the new queue status of a user every time it changes.
type Notifier func(userID string, status model.MatchmakingStatus)

// Queue is an in-memory ranked/unranked queue. It widens the search window
// over time, so matching is re-evaluated on a timer and not only on join.
type Queue struct {
	mu      sync.Mutex
	tickets map[string]*ticket
	byUser  map[string]string
	notify  Notifier
	now     func() time.Time
	stop    chan struct{}
	done    chan struct{}
}

func New(notify Notifier) *Queue {
	if notify == nil {
		notify = func(string, model.MatchmakingStatus) {}
	}
	return &Queue{tickets: map[string]*ticket{}, byUser: map[string]string{}, notify: notify, now: func() time.Time { return time.Now().UTC() }}
}

// Start runs the periodic matching pass until Stop is called.
func (queue *Queue) Start(interval time.Duration) {
	queue.mu.Lock()
	if queue.stop != nil {
		queue.mu.Unlock()
		return
	}
	queue.stop = make(chan struct{})
	queue.done = make(chan struct{})
	stop, done := queue.stop, queue.done
	queue.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				queue.Tick()
			case <-stop:
				return
			}
		}
	}()
}

func (queue *Queue) Stop() {
	queue.mu.Lock()
	stop, done := queue.stop, queue.done
	queue.stop, queue.done = nil, nil
	queue.mu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

// Tick re-runs matching for every mode and pushes the statuses that changed.
func (queue *Queue) Tick() {
	queue.mu.Lock()
	for _, mode := range []string{"ranked", "unranked"} {
		queue.tryMatch(mode)
	}
	updates := queue.collectChangesLocked()
	queue.mu.Unlock()
	queue.dispatch(updates)
}

func (queue *Queue) Join(userID, mode string, elo int) model.MatchmakingStatus {
	queue.mu.Lock()
	if oldID, ok := queue.byUser[userID]; ok {
		delete(queue.tickets, oldID)
	}
	entry := &ticket{id: uuid.NewString(), userID: userID, mode: mode, elo: elo, createdAt: queue.now()}
	queue.tickets[entry.id] = entry
	queue.byUser[userID] = entry.id
	queue.tryMatch(mode)
	status := queue.statusLocked(userID)
	entry.lastSent = fingerprintOf(status)
	entry.announced = true
	updates := queue.collectChangesLocked()
	queue.mu.Unlock()
	queue.dispatch(updates)
	return status
}

func (queue *Queue) Leave(userID string) {
	queue.mu.Lock()
	removed := queue.removeLocked(userID)
	updates := queue.collectChangesLocked()
	if removed {
		updates = append(updates, update{userID: userID, status: notWaiting()})
	}
	queue.mu.Unlock()
	queue.dispatch(updates)
}

func (queue *Queue) Status(userID string) model.MatchmakingStatus {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.statusLocked(userID)
}

type update struct {
	userID string
	status model.MatchmakingStatus
}

func (queue *Queue) dispatch(updates []update) {
	for _, item := range updates {
		queue.notify(item.userID, item.status)
	}
}

func (queue *Queue) collectChangesLocked() []update {
	updates := make([]update, 0)
	for _, entry := range queue.tickets {
		status := queue.statusLocked(entry.userID)
		current := fingerprintOf(status)
		if entry.announced && current == entry.lastSent {
			continue
		}
		entry.lastSent = current
		entry.announced = true
		updates = append(updates, update{userID: entry.userID, status: status})
	}
	return updates
}

func fingerprintOf(status model.MatchmakingStatus) fingerprint {
	gap := 0
	if status.SearchEloGap != nil {
		gap = *status.SearchEloGap
	}
	return fingerprint{matched: status.Matched, position: status.Position, queueSize: status.QueueSize, gap: gap}
}

func (queue *Queue) removeLocked(userID string) bool {
	id, ok := queue.byUser[userID]
	if !ok {
		return false
	}
	delete(queue.tickets, id)
	delete(queue.byUser, userID)
	return true
}

func (queue *Queue) tryMatch(mode string) {
	entries := make([]*ticket, 0)
	for _, entry := range queue.tickets {
		if entry.mode == mode && entry.matchedAt.IsZero() {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].createdAt.Before(entries[j].createdAt) })
	now := queue.now()
	for len(entries) >= 2 {
		base := entries[0]
		gap := searchGap(now.Sub(base.createdAt))
		match := []*ticket{base}
		rest := make([]*ticket, 0, len(entries))
		for _, candidate := range entries[1:] {
			if len(match) < 4 && abs(base.elo-candidate.elo) <= gap {
				match = append(match, candidate)
				continue
			}
			rest = append(rest, candidate)
		}
		if len(match) < 2 {
			return
		}
		players := make([]ticket, len(match))
		for i, item := range match {
			players[i] = *item
		}
		for _, item := range match {
			item.matchedAt = now
			item.players = players
		}
		entries = rest
	}
}

func notWaiting() model.MatchmakingStatus {
	return model.MatchmakingStatus{Status: "not_waiting", Players: []model.MatchmakingPlayer{}}
}

func (queue *Queue) statusLocked(userID string) model.MatchmakingStatus {
	id, ok := queue.byUser[userID]
	if !ok {
		return notWaiting()
	}
	entry := queue.tickets[id]
	if entry == nil {
		return notWaiting()
	}
	waited := int64(queue.now().Sub(entry.createdAt).Seconds())
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
