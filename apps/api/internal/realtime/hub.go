package realtime

import (
	"regexp"
	"sync"
)

const subscriberBuffer = 64

// Event is published on a topic. Payload is shared by every subscriber;
// Render, when set, builds a viewer-specific payload and may return nil to
// skip a subscriber that must not receive the event.
type Event struct {
	Topic   string
	Name    string
	Payload any
	Render  func(viewerID string) any
}

// Subscriber is one connected client. Events are delivered on Events; Dropped
// is closed when the subscriber fell behind and must resynchronise.
type Subscriber struct {
	UserID  string
	Events  chan Event
	Dropped chan struct{}
	once    sync.Once
}

func (subscriber *Subscriber) drop() {
	subscriber.once.Do(func() { close(subscriber.Dropped) })
}

// Hub fans events out to subscribers grouped by topic.
type Hub struct {
	mu     sync.RWMutex
	topics map[string]map[*Subscriber]struct{}
	subs   map[*Subscriber]map[string]struct{}
	closed bool
}

func NewHub() *Hub {
	return &Hub{topics: map[string]map[*Subscriber]struct{}{}, subs: map[*Subscriber]map[string]struct{}{}}
}

func (hub *Hub) Register(userID string) *Subscriber {
	subscriber := &Subscriber{UserID: userID, Events: make(chan Event, subscriberBuffer), Dropped: make(chan struct{})}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.closed {
		subscriber.drop()
		return subscriber
	}
	hub.subs[subscriber] = map[string]struct{}{}
	return subscriber
}

// Unregister removes every subscription and returns the topics that were
// subscribed so the caller can release per-topic resources.
func (hub *Hub) Unregister(subscriber *Subscriber) []string {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	topics := make([]string, 0, len(hub.subs[subscriber]))
	for topic := range hub.subs[subscriber] {
		topics = append(topics, topic)
		hub.removeLocked(topic, subscriber)
	}
	delete(hub.subs, subscriber)
	subscriber.drop()
	return topics
}

func (hub *Hub) Subscribe(subscriber *Subscriber, topic string) bool {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	owned, ok := hub.subs[subscriber]
	if !ok {
		return false
	}
	if _, already := owned[topic]; already {
		return false
	}
	owned[topic] = struct{}{}
	if hub.topics[topic] == nil {
		hub.topics[topic] = map[*Subscriber]struct{}{}
	}
	hub.topics[topic][subscriber] = struct{}{}
	return true
}

func (hub *Hub) Unsubscribe(subscriber *Subscriber, topic string) bool {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	owned, ok := hub.subs[subscriber]
	if !ok {
		return false
	}
	if _, present := owned[topic]; !present {
		return false
	}
	delete(owned, topic)
	hub.removeLocked(topic, subscriber)
	return true
}

func (hub *Hub) removeLocked(topic string, subscriber *Subscriber) {
	if set := hub.topics[topic]; set != nil {
		delete(set, subscriber)
		if len(set) == 0 {
			delete(hub.topics, topic)
		}
	}
}

// Publish delivers the event without blocking. A subscriber whose buffer is
// full is dropped so that its connection resynchronises instead of stalling
// every other subscriber.
func (hub *Hub) Publish(event Event) int {
	hub.mu.RLock()
	targets := make([]*Subscriber, 0, len(hub.topics[event.Topic]))
	for subscriber := range hub.topics[event.Topic] {
		targets = append(targets, subscriber)
	}
	hub.mu.RUnlock()
	delivered := 0
	for _, subscriber := range targets {
		select {
		case subscriber.Events <- event:
			delivered++
		default:
			subscriber.drop()
		}
	}
	return delivered
}

// SubscriptionCount returns how many subscribers of userID follow topic.
func (hub *Hub) SubscriptionCount(topic, userID string) int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	count := 0
	for subscriber := range hub.topics[topic] {
		if subscriber.UserID == userID {
			count++
		}
	}
	return count
}

// Subscriptions lists the distinct user IDs subscribed to each topic that
// matches the given prefix.
func (hub *Hub) Subscriptions(prefix string) map[string][]string {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	result := map[string][]string{}
	for topic, set := range hub.topics {
		if len(topic) < len(prefix) || topic[:len(prefix)] != prefix {
			continue
		}
		seen := map[string]struct{}{}
		for subscriber := range set {
			if subscriber.UserID == "" {
				continue
			}
			if _, ok := seen[subscriber.UserID]; ok {
				continue
			}
			seen[subscriber.UserID] = struct{}{}
			result[topic] = append(result[topic], subscriber.UserID)
		}
	}
	return result
}

// ConnectedUsers lists the distinct authenticated users holding a connection.
func (hub *Hub) ConnectedUsers() []string {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	seen := map[string]struct{}{}
	users := make([]string, 0, len(hub.subs))
	for subscriber := range hub.subs {
		if subscriber.UserID == "" {
			continue
		}
		if _, ok := seen[subscriber.UserID]; ok {
			continue
		}
		seen[subscriber.UserID] = struct{}{}
		users = append(users, subscriber.UserID)
	}
	return users
}

// Drop marks the subscriber as fallen behind so its connection resynchronises.
func (subscriber *Subscriber) Drop() { subscriber.drop() }

func (hub *Hub) Close() {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.closed = true
	for subscriber := range hub.subs {
		subscriber.drop()
	}
	hub.subs = map[*Subscriber]map[string]struct{}{}
	hub.topics = map[string]map[*Subscriber]struct{}{}
}

var topicPattern = regexp.MustCompile(`^(lobbies|leaderboard|game:[A-Za-z0-9-]{1,64}|profile:[A-Za-z0-9-]{1,64}|user:[A-Za-z0-9-]{1,64})$`)

// ValidTopic reports whether a client-supplied topic name is well formed.
func ValidTopic(topic string) bool { return topicPattern.MatchString(topic) }

// TopicID returns the identifier part of a scoped topic such as game:{id}.
func TopicID(topic string) string {
	for index := 0; index < len(topic); index++ {
		if topic[index] == ':' {
			return topic[index+1:]
		}
	}
	return ""
}

func GameTopic(id string) string    { return "game:" + id }
func ProfileTopic(id string) string { return "profile:" + id }
func UserTopic(id string) string    { return "user:" + id }

const (
	TopicLobbies     = "lobbies"
	TopicLeaderboard = "leaderboard"
)
