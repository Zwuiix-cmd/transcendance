package realtime

import (
	"encoding/json"
	"testing"
)

func TestPublishReachesOnlyTopicSubscribers(t *testing.T) {
	hub := NewHub()
	alice := hub.Register("alice")
	bob := hub.Register("bob")
	if !hub.Subscribe(alice, GameTopic("g1")) || !hub.Subscribe(bob, GameTopic("g2")) {
		t.Fatal("subscriptions should succeed")
	}
	if delivered := hub.Publish(Event{Topic: GameTopic("g1"), Name: EventGameUpdated, Payload: "x"}); delivered != 1 {
		t.Fatalf("expected one delivery, got %d", delivered)
	}
	select {
	case event := <-alice.Events:
		if event.Name != EventGameUpdated {
			t.Fatalf("unexpected event %q", event.Name)
		}
	default:
		t.Fatal("alice should have received the event")
	}
	select {
	case <-bob.Events:
		t.Fatal("bob must not receive events from another topic")
	default:
	}
}

func TestSubscribeIsIdempotentAndUnsubscribeStopsDelivery(t *testing.T) {
	hub := NewHub()
	alice := hub.Register("alice")
	if !hub.Subscribe(alice, TopicLobbies) {
		t.Fatal("first subscribe should succeed")
	}
	if hub.Subscribe(alice, TopicLobbies) {
		t.Fatal("second subscribe to the same topic should be rejected")
	}
	if hub.SubscriptionCount(TopicLobbies, "alice") != 1 {
		t.Fatal("expected a single subscription")
	}
	if !hub.Unsubscribe(alice, TopicLobbies) {
		t.Fatal("unsubscribe should succeed")
	}
	if hub.Publish(Event{Topic: TopicLobbies, Name: EventLobbiesChanged}) != 0 {
		t.Fatal("no subscriber should remain")
	}
}

func TestSlowSubscriberIsDropped(t *testing.T) {
	hub := NewHub()
	slow := hub.Register("slow")
	hub.Subscribe(slow, TopicLeaderboard)
	for index := 0; index < subscriberBuffer; index++ {
		hub.Publish(Event{Topic: TopicLeaderboard, Name: EventLeaderboardChanged})
	}
	select {
	case <-slow.Dropped:
		t.Fatal("subscriber must not be dropped while its buffer has room")
	default:
	}
	hub.Publish(Event{Topic: TopicLeaderboard, Name: EventLeaderboardChanged})
	select {
	case <-slow.Dropped:
	default:
		t.Fatal("subscriber should be dropped once its buffer overflows")
	}
}

func TestUnregisterReturnsTopicsAndDrops(t *testing.T) {
	hub := NewHub()
	alice := hub.Register("alice")
	hub.Subscribe(alice, GameTopic("g1"))
	hub.Subscribe(alice, TopicLobbies)
	topics := hub.Unregister(alice)
	if len(topics) != 2 {
		t.Fatalf("expected two topics, got %v", topics)
	}
	select {
	case <-alice.Dropped:
	default:
		t.Fatal("unregistered subscriber should be marked dropped")
	}
	if hub.Subscribe(alice, TopicLobbies) {
		t.Fatal("an unregistered subscriber cannot subscribe again")
	}
}

func TestSubscriptionsGroupsUsersByTopicPrefix(t *testing.T) {
	hub := NewHub()
	first := hub.Register("alice")
	second := hub.Register("alice")
	third := hub.Register("")
	hub.Subscribe(first, GameTopic("g1"))
	hub.Subscribe(second, GameTopic("g1"))
	hub.Subscribe(third, GameTopic("g1"))
	hub.Subscribe(first, TopicLobbies)
	result := hub.Subscriptions("game:")
	if len(result) != 1 || len(result[GameTopic("g1")]) != 1 || result[GameTopic("g1")][0] != "alice" {
		t.Fatalf("unexpected subscriptions %v", result)
	}
}

func TestEncodeUsesRendererAndSkipsNil(t *testing.T) {
	event := Event{Topic: GameTopic("g1"), Name: EventUnoState, Render: func(viewerID string) any {
		if viewerID == "outsider" {
			return nil
		}
		return map[string]string{"viewer": viewerID}
	}}
	if _, ok := Encode(event, "outsider"); ok {
		t.Fatal("renderer returning nil must skip the viewer")
	}
	encoded, ok := Encode(event, "alice")
	if !ok {
		t.Fatal("expected an encoded frame")
	}
	var message ServerMessage
	if err := json.Unmarshal(encoded, &message); err != nil {
		t.Fatal(err)
	}
	if message.Type != ServerEvent || message.Event != EventUnoState || string(message.Payload) != `{"viewer":"alice"}` {
		t.Fatalf("unexpected frame %s", encoded)
	}
}

func TestValidTopic(t *testing.T) {
	for _, topic := range []string{"lobbies", "leaderboard", "game:abc-123", "profile:u1", "user:u1"} {
		if !ValidTopic(topic) {
			t.Fatalf("%q should be valid", topic)
		}
	}
	for _, topic := range []string{"", "game:", "games:1", "game:a b", "admin", "user:" + string(make([]byte, 70))} {
		if ValidTopic(topic) {
			t.Fatalf("%q should be rejected", topic)
		}
	}
	if TopicID("game:abc") != "abc" || TopicID("lobbies") != "" {
		t.Fatal("TopicID should return the scoped identifier")
	}
}
