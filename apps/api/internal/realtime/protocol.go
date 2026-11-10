package realtime

import "encoding/json"

// ClientMessage is a JSON frame sent by a browser.
type ClientMessage struct {
	Type  string `json:"type"`
	Token string `json:"token,omitempty"`
	Topic string `json:"topic,omitempty"`
}

// ServerMessage is a JSON frame sent to a browser.
type ServerMessage struct {
	Type    string          `json:"type"`
	Topic   string          `json:"topic,omitempty"`
	Event   string          `json:"event,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	UserID  *string         `json:"userId,omitempty"`
	Code    string          `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
}

const (
	ClientAuth        = "auth"
	ClientSubscribe   = "subscribe"
	ClientUnsubscribe = "unsubscribe"
	ClientPing        = "ping"

	ServerReady        = "ready"
	ServerSubscribed   = "subscribed"
	ServerUnsubscribed = "unsubscribed"
	ServerEvent        = "event"
	ServerError        = "error"
	ServerHeartbeat    = "heartbeat"
	ServerPong         = "pong"

	EventGameUpdated        = "game.updated"
	EventGameDeleted        = "game.deleted"
	EventUnoState           = "uno.state"
	EventLobbiesChanged     = "lobbies.changed"
	EventLeaderboardChanged = "leaderboard.changed"
	EventProfileChanged     = "profile.changed"
	EventMatchmakingUpdated = "matchmaking.updated"
)

// Encode renders an event for one viewer. The second value is false when the
// viewer must not receive this event.
func Encode(event Event, viewerID string) ([]byte, bool) {
	payload := event.Payload
	if event.Render != nil {
		payload = event.Render(viewerID)
		if payload == nil {
			return nil, false
		}
	}
	message := ServerMessage{Type: ServerEvent, Topic: event.Topic, Event: event.Name}
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, false
		}
		message.Payload = encoded
	}
	encoded, err := json.Marshal(message)
	if err != nil {
		return nil, false
	}
	return encoded, true
}
