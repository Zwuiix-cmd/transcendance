package api

import (
	"log/slog"
	"sync"

	"card-arena/api/internal/db"
	"github.com/google/uuid"
)

const auditQueueSize = 512

type auditEvent struct {
	actorID, actorUsername string
	action, targetType     string
	targetID, details      string
	severity               string
	ipAddress, userAgent   string
}

// auditWriter decouples request latency from audit persistence while keeping a
// bounded queue. Close drains the queue before returning.
type auditWriter struct {
	store  *db.Store
	events chan auditEvent
	done   chan struct{}
	wg     sync.WaitGroup
	mu     sync.RWMutex
	closed bool
}

func newAuditWriter(store *db.Store) *auditWriter {
	writer := &auditWriter{
		store:  store,
		events: make(chan auditEvent, auditQueueSize),
		done:   make(chan struct{}),
	}
	writer.wg.Add(1)
	go writer.run()
	return writer
}

func (writer *auditWriter) enqueue(event auditEvent) {
	writer.mu.RLock()
	defer writer.mu.RUnlock()
	if writer.closed {
		return
	}
	select {
	case writer.events <- event:
	default:
		slog.Warn("audit queue is full; dropping event", "action", event.action)
	}
}

func (writer *auditWriter) run() {
	defer writer.wg.Done()
	for event := range writer.events {
		writer.persist(event)
	}
	close(writer.done)
}

func (writer *auditWriter) persist(event auditEvent) {
	_, err := writer.store.DB.Exec(`INSERT INTO audit_logs (id,actor_user_id,actor_username,action,target_type,target_id,details,severity,ip_address,user_agent,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, uuid.NewString(), nilIfBlank(event.actorID), nilIfBlank(event.actorUsername), event.action, event.targetType, nilIfBlank(event.targetID), event.details, event.severity, nilIfBlank(event.ipAddress), nilIfBlank(event.userAgent), nowString())
	if err != nil {
		slog.Warn("could not persist audit log", "error", err, "action", event.action)
	}
}

func (writer *auditWriter) close() {
	if writer == nil {
		return
	}
	writer.mu.Lock()
	if !writer.closed {
		writer.closed = true
		close(writer.events)
	}
	writer.mu.Unlock()
	writer.wg.Wait()
}
