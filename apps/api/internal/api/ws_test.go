package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"card-arena/api/internal/config"
	"card-arena/api/internal/db"
	"card-arena/api/internal/model"
	"card-arena/api/internal/realtime"
	"github.com/coder/websocket"
)

type harness struct {
	t      *testing.T
	server *Server
	http   *httptest.Server
}

func schemaPath(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{"../../db/schema.sql", "../../schema.sql"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Fatal("schema.sql was not found next to the api module")
	return ""
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store, err := db.Open(":memory:", schemaPath(t))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	cfg := config.Config{APIKey: "", SessionTTL: time.Hour, CORSAllowedOrigins: []string{"*"}, ValidRequestsPerMinute: 10000, InvalidRequestsPerMinute: 100, AdminRequestsPerMinute: 1000, CDNPublicURL: "/files"}
	server := NewServer(cfg, store)
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(func() {
		httpServer.Close()
		server.Close()
		store.Close()
	})
	return &harness{t: t, server: server, http: httpServer}
}

func (h *harness) request(method, path, token string, body any) (*http.Response, []byte) {
	h.t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	request, err := http.NewRequest(method, h.http.URL+path, bytes.NewReader(payload))
	if err != nil {
		h.t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		h.t.Fatal(err)
	}
	defer response.Body.Close()
	var buffer bytes.Buffer
	_, _ = buffer.ReadFrom(response.Body)
	return response, buffer.Bytes()
}

func (h *harness) register(username string) (string, model.User) {
	h.t.Helper()
	response, body := h.request(http.MethodPost, "/api/v1/auth/register", "", map[string]string{"username": username, "email": strings.ToLower(username) + "@learner.42.tech", "password": "a-very-long-password-123"})
	if response.StatusCode != http.StatusCreated {
		h.t.Fatalf("register %s: %d %s", username, response.StatusCode, body)
	}
	var auth model.AuthResponse
	if err := json.Unmarshal(body, &auth); err != nil {
		h.t.Fatal(err)
	}
	return auth.AccessToken, auth.User
}

type client struct {
	t    *testing.T
	conn *websocket.Conn
	ctx  context.Context
}

func (h *harness) connect(token string) *client {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	h.t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(h.http.URL, "http")+"/api/v1/ws", nil)
	if err != nil {
		h.t.Fatalf("dial: %v", err)
	}
	h.t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })
	c := &client{t: h.t, conn: conn, ctx: ctx}
	c.send(realtime.ClientMessage{Type: realtime.ClientAuth, Token: token})
	return c
}

func (c *client) send(message realtime.ClientMessage) {
	c.t.Helper()
	data, _ := json.Marshal(message)
	if err := c.conn.Write(c.ctx, websocket.MessageText, data); err != nil {
		c.t.Fatalf("write: %v", err)
	}
}

func (c *client) next() realtime.ServerMessage {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(c.ctx, 3*time.Second)
	defer cancel()
	_, data, err := c.conn.Read(ctx)
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	var message realtime.ServerMessage
	if err := json.Unmarshal(data, &message); err != nil {
		c.t.Fatalf("decode %s: %v", data, err)
	}
	return message
}

func (c *client) expect(kind string) realtime.ServerMessage {
	c.t.Helper()
	message := c.next()
	if message.Type != kind {
		c.t.Fatalf("expected %q frame, got %+v", kind, message)
	}
	return message
}

func (c *client) expectEvent(name string) realtime.ServerMessage {
	c.t.Helper()
	message := c.expect(realtime.ServerEvent)
	if message.Event != name {
		c.t.Fatalf("expected event %q, got %+v", name, message)
	}
	return message
}

func (c *client) subscribe(topic string) {
	c.t.Helper()
	c.send(realtime.ClientMessage{Type: realtime.ClientSubscribe, Topic: topic})
	message := c.expect(realtime.ServerSubscribed)
	if message.Topic != topic {
		c.t.Fatalf("subscribed to %q instead of %q", message.Topic, topic)
	}
}

func TestWebSocketRequiresAuthFrameFirst(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(h.http.URL, "http")+"/api/v1/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(realtime.ClientMessage{Type: realtime.ClientSubscribe, Topic: realtime.TopicLobbies})
	_ = conn.Write(ctx, websocket.MessageText, data)
	_, _, err = conn.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("expected a policy violation close, got %v", err)
	}
}

func TestWebSocketRejectsInvalidToken(t *testing.T) {
	h := newHarness(t)
	c := h.connect("not-a-real-token")
	message := c.expect(realtime.ServerError)
	if message.Code != "invalid_token" {
		t.Fatalf("unexpected error %+v", message)
	}
	_, _, err := c.conn.Read(c.ctx)
	if websocket.CloseStatus(err) != wsCloseUnauthorized {
		t.Fatalf("expected close %d, got %v", wsCloseUnauthorized, err)
	}
}

func TestAnonymousClientFollowsPublicTopicsOnly(t *testing.T) {
	h := newHarness(t)
	c := h.connect("")
	ready := c.expect(realtime.ServerReady)
	if ready.UserID != nil {
		t.Fatalf("anonymous ready frame must not carry a user id: %+v", ready)
	}
	c.subscribe(realtime.TopicLeaderboard)
	c.send(realtime.ClientMessage{Type: realtime.ClientSubscribe, Topic: realtime.UserTopic("someone")})
	if message := c.expect(realtime.ServerError); message.Code != "forbidden" {
		t.Fatalf("expected forbidden, got %+v", message)
	}
	c.send(realtime.ClientMessage{Type: realtime.ClientSubscribe, Topic: realtime.GameTopic("missing")})
	if message := c.expect(realtime.ServerError); message.Code != "authentication_required" {
		t.Fatalf("expected authentication_required, got %+v", message)
	}
	c.send(realtime.ClientMessage{Type: realtime.ClientSubscribe, Topic: "nonsense"})
	if message := c.expect(realtime.ServerError); message.Code != "invalid_topic" {
		t.Fatalf("expected invalid_topic, got %+v", message)
	}
	c.send(realtime.ClientMessage{Type: realtime.ClientPing})
	c.expect(realtime.ServerPong)
}

func TestLobbyEventsReachSubscribersWithViewerSpecificPayload(t *testing.T) {
	h := newHarness(t)
	hostToken, host := h.register("Host")
	guestToken, guest := h.register("Guest")

	hostClient := h.connect(hostToken)
	if ready := hostClient.expect(realtime.ServerReady); ready.UserID == nil || *ready.UserID != host.ID {
		t.Fatalf("ready frame should identify the host: %+v", ready)
	}
	hostClient.subscribe(realtime.TopicLobbies)

	response, body := h.request(http.MethodPost, "/api/v1/games", hostToken, map[string]any{"turnTimeLimit": 15, "privateGame": true, "mode": "custom"})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create game: %d %s", response.StatusCode, body)
	}
	var game model.Game
	_ = json.Unmarshal(body, &game)
	hostClient.expectEvent(realtime.EventLobbiesChanged)

	hostClient.subscribe(realtime.GameTopic(game.ID))
	guestClient := h.connect(guestToken)
	guestClient.expect(realtime.ServerReady)
	guestClient.subscribe(realtime.GameTopic(game.ID))

	response, body = h.request(http.MethodPost, "/api/v1/games/"+game.ID+"/join", guestToken, map[string]string{"code": *game.JoinCode})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("join: %d %s", response.StatusCode, body)
	}

	hostEvent := hostClient.expectEvent(realtime.EventGameUpdated)
	var hostDetail model.GameDetail
	_ = json.Unmarshal(hostEvent.Payload, &hostDetail)
	if len(hostDetail.Participants) != 2 || !hostDetail.Game.Host || hostDetail.Game.JoinCode == nil {
		t.Fatalf("host should see both players and the invite code: %+v", hostDetail.Game)
	}
	hostClient.expectEvent(realtime.EventLobbiesChanged)

	guestEvent := guestClient.expectEvent(realtime.EventGameUpdated)
	var guestDetail model.GameDetail
	_ = json.Unmarshal(guestEvent.Payload, &guestDetail)
	if len(guestDetail.Participants) != 2 || guestDetail.Game.Host || guestDetail.Game.JoinCode != nil || !guestDetail.Game.Joined {
		t.Fatalf("guest must not see the invite code and must be marked joined: %+v", guestDetail.Game)
	}
	if guest.ID == host.ID {
		t.Fatal("test users must differ")
	}
}

func TestUnoStateIsPushedWithPrivateHands(t *testing.T) {
	h := newHarness(t)
	hostToken, host := h.register("Dealer")
	guestToken, guest := h.register("Player")
	watcherToken, _ := h.register("Watcher")

	response, body := h.request(http.MethodPost, "/api/v1/games", hostToken, map[string]any{"turnTimeLimit": 15, "mode": "custom"})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create game: %d %s", response.StatusCode, body)
	}
	var game model.Game
	_ = json.Unmarshal(body, &game)
	if response, body = h.request(http.MethodPost, "/api/v1/games/"+game.ID+"/join", guestToken, map[string]string{}); response.StatusCode != http.StatusOK {
		t.Fatalf("join: %d %s", response.StatusCode, body)
	}

	hostClient := h.connect(hostToken)
	hostClient.expect(realtime.ServerReady)
	hostClient.subscribe(realtime.GameTopic(game.ID))
	guestClient := h.connect(guestToken)
	guestClient.expect(realtime.ServerReady)
	guestClient.subscribe(realtime.GameTopic(game.ID))
	watcherClient := h.connect(watcherToken)
	watcherClient.expect(realtime.ServerReady)
	watcherClient.subscribe(realtime.GameTopic(game.ID))

	if response, body = h.request(http.MethodPost, "/api/v1/games/"+game.ID+"/uno/start", hostToken, nil); response.StatusCode != http.StatusOK {
		t.Fatalf("start: %d %s", response.StatusCode, body)
	}

	hostState := hostClient.expectEvent(realtime.EventUnoState)
	guestState := guestClient.expectEvent(realtime.EventUnoState)
	var hostSnapshot, guestSnapshot struct {
		Version int64 `json:"version"`
		Players []struct {
			ID   string   `json:"id"`
			Hand []string `json:"hand"`
		} `json:"players"`
	}
	_ = json.Unmarshal(hostState.Payload, &hostSnapshot)
	_ = json.Unmarshal(guestState.Payload, &guestSnapshot)
	for _, player := range hostSnapshot.Players {
		if player.ID == host.ID && len(player.Hand) != 7 {
			t.Fatalf("host should see own hand: %+v", player)
		}
		if player.ID == guest.ID && len(player.Hand) != 0 {
			t.Fatalf("host must not see the guest hand: %+v", player)
		}
	}
	for _, player := range guestSnapshot.Players {
		if player.ID == guest.ID && len(player.Hand) != 7 {
			t.Fatalf("guest should see own hand: %+v", player)
		}
	}
	hostClient.expectEvent(realtime.EventGameUpdated)
	guestClient.expectEvent(realtime.EventGameUpdated)

	watcherEvent := watcherClient.expectEvent(realtime.EventGameUpdated)
	if watcherEvent.Event == realtime.EventUnoState {
		t.Fatal("a user who neither plays nor spectates must not receive the uno state")
	}
}

func TestMatchmakingStatusIsPushedToTheUserTopic(t *testing.T) {
	h := newHarness(t)
	aliceToken, alice := h.register("Alice")
	bobToken, _ := h.register("Bob")
	aliceClient := h.connect(aliceToken)
	aliceClient.expect(realtime.ServerReady)
	aliceClient.subscribe(realtime.UserTopic(alice.ID))

	if response, body := h.request(http.MethodPost, "/api/v1/matchmaking/queue", aliceToken, map[string]string{"mode": "ranked"}); response.StatusCode != http.StatusOK {
		t.Fatalf("alice join: %d %s", response.StatusCode, body)
	}
	if response, body := h.request(http.MethodPost, "/api/v1/matchmaking/queue", bobToken, map[string]string{"mode": "ranked"}); response.StatusCode != http.StatusOK {
		t.Fatalf("bob join: %d %s", response.StatusCode, body)
	}
	event := aliceClient.expectEvent(realtime.EventMatchmakingUpdated)
	var status model.MatchmakingStatus
	_ = json.Unmarshal(event.Payload, &status)
	if !status.Matched || len(status.Players) != 2 {
		t.Fatalf("alice should be matched with bob: %+v", status)
	}
}

func TestSpectatorIsRemovedWhenItsLastConnectionCloses(t *testing.T) {
	h := newHarness(t)
	hostToken, _ := h.register("Owner")
	guestToken, _ := h.register("Second")
	watcherToken, watcher := h.register("Viewer")
	response, body := h.request(http.MethodPost, "/api/v1/games", hostToken, map[string]any{"turnTimeLimit": 15, "mode": "custom"})
	var game model.Game
	_ = json.Unmarshal(body, &game)
	h.request(http.MethodPost, "/api/v1/games/"+game.ID+"/join", guestToken, map[string]string{})
	if response, body = h.request(http.MethodPost, "/api/v1/games/"+game.ID+"/uno/start", hostToken, nil); response.StatusCode != http.StatusOK {
		t.Fatalf("start: %d %s", response.StatusCode, body)
	}
	if response, body = h.request(http.MethodPost, "/api/v1/games/"+game.ID+"/spectate", watcherToken, nil); response.StatusCode != http.StatusOK {
		t.Fatalf("spectate: %d %s", response.StatusCode, body)
	}
	watcherClient := h.connect(watcherToken)
	watcherClient.expect(realtime.ServerReady)
	watcherClient.subscribe(realtime.GameTopic(game.ID))
	hostClient := h.connect(hostToken)
	hostClient.expect(realtime.ServerReady)
	hostClient.subscribe(realtime.GameTopic(game.ID))

	watcherClient.conn.Close(websocket.StatusNormalClosure, "leaving")
	event := hostClient.expectEvent(realtime.EventGameUpdated)
	var detail model.GameDetail
	_ = json.Unmarshal(event.Payload, &detail)
	for _, spectator := range detail.Spectators {
		if spectator.UserID == watcher.ID {
			t.Fatal("the spectator should be removed once its connection closes")
		}
	}
	var remaining int
	_ = h.server.store.DB.QueryRow(`SELECT COUNT(*) FROM game_spectators WHERE game_id=? AND user_id=?`, game.ID, watcher.ID).Scan(&remaining)
	if remaining != 0 {
		t.Fatal("spectator row should be deleted")
	}
}
