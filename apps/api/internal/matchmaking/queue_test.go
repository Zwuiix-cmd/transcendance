package matchmaking

import (
	"sync"
	"testing"
	"time"

	"card-arena/api/internal/model"
)

type recorder struct {
	mu      sync.Mutex
	updates map[string][]model.MatchmakingStatus
}

func newRecorder() *recorder { return &recorder{updates: map[string][]model.MatchmakingStatus{}} }

func (r *recorder) notify(userID string, status model.MatchmakingStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates[userID] = append(r.updates[userID], status)
}

func (r *recorder) last(userID string) (model.MatchmakingStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.updates[userID]
	if len(list) == 0 {
		return model.MatchmakingStatus{}, false
	}
	return list[len(list)-1], true
}

func (r *recorder) count(userID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.updates[userID])
}

func newTestQueue(rec *recorder) (*Queue, *time.Time) {
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	queue := New(rec.notify)
	queue.now = func() time.Time { return clock }
	return queue, &clock
}

func TestJoinNotifiesEarlierTicketsWhenQueueChanges(t *testing.T) {
	rec := newRecorder()
	queue, _ := newTestQueue(rec)
	first := queue.Join("alice", "ranked", 1000)
	if first.Status != "waiting" || first.QueueSize != 1 {
		t.Fatalf("unexpected first status %+v", first)
	}
	if rec.count("alice") != 0 {
		t.Fatal("the joining user already holds the status returned by Join and must not be notified again")
	}
	queue.Join("bob", "ranked", 2500)
	alice, ok := rec.last("alice")
	if !ok || alice.QueueSize != 2 || alice.Matched {
		t.Fatalf("alice should learn the queue grew without matching: %+v ok=%v", alice, ok)
	}
}

func TestCloseEloPlayersMatchImmediatelyAndBothAreNotified(t *testing.T) {
	rec := newRecorder()
	queue, _ := newTestQueue(rec)
	queue.Join("alice", "ranked", 1000)
	bob := queue.Join("bob", "ranked", 1100)
	if !bob.Matched || len(bob.Players) != 2 {
		t.Fatalf("bob should be matched with alice: %+v", bob)
	}
	alice, ok := rec.last("alice")
	if !ok || !alice.Matched || alice.Status != "matched" {
		t.Fatalf("alice should be notified about the match: %+v ok=%v", alice, ok)
	}
}

func TestTickWidensSearchWindowAndMatchesLater(t *testing.T) {
	rec := newRecorder()
	queue, clock := newTestQueue(rec)
	queue.Join("alice", "ranked", 1000)
	queue.Join("bob", "ranked", 1500)
	if status, _ := rec.last("alice"); status.Matched {
		t.Fatal("a 500 ELO gap must not match at once")
	}
	*clock = clock.Add(10 * time.Second)
	queue.Tick()
	alice, _ := rec.last("alice")
	if alice.Matched || alice.SearchEloGap == nil || *alice.SearchEloGap != 350 {
		t.Fatalf("after 10s the gap should widen to 350 without matching: %+v", alice)
	}
	*clock = clock.Add(15 * time.Second)
	queue.Tick()
	alice, _ = rec.last("alice")
	bob, _ := rec.last("bob")
	if !alice.Matched || !bob.Matched {
		t.Fatalf("after 25s the 500 gap fits the 575 window: alice=%+v bob=%+v", alice, bob)
	}
}

func TestTickIsSilentWhenNothingChanged(t *testing.T) {
	rec := newRecorder()
	queue, clock := newTestQueue(rec)
	queue.Join("alice", "ranked", 1000)
	*clock = clock.Add(2 * time.Second)
	queue.Tick()
	if rec.count("alice") != 0 {
		t.Fatalf("no structural change happened, got %d notifications", rec.count("alice"))
	}
	*clock = clock.Add(3 * time.Second)
	queue.Tick()
	if rec.count("alice") != 1 {
		t.Fatalf("the window widened after 5s, expected one notification, got %d", rec.count("alice"))
	}
}

func TestLeaveNotifiesLeaverAndRemainingPlayers(t *testing.T) {
	rec := newRecorder()
	queue, _ := newTestQueue(rec)
	queue.Join("alice", "unranked", 1000)
	queue.Join("bob", "unranked", 1000)
	queue.Join("carol", "unranked", 1000)
	queue.Leave("alice")
	alice, ok := rec.last("alice")
	if !ok || alice.Status != "not_waiting" || alice.Queued {
		t.Fatalf("alice should be told she left: %+v", alice)
	}
	if queue.Status("alice").Queued {
		t.Fatal("alice must be removed from the queue")
	}
}

func TestStartStopRunsTicks(t *testing.T) {
	rec := newRecorder()
	queue, clock := newTestQueue(rec)
	queue.Join("alice", "ranked", 1000)
	queue.Join("bob", "ranked", 1500)
	*clock = clock.Add(25 * time.Second)
	queue.Start(5 * time.Millisecond)
	defer queue.Stop()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if queue.Status("alice").Matched {
			queue.Stop()
			queue.Stop()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the background ticker should have matched the widened window")
}
