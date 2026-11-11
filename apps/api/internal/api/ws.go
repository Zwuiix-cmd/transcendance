package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	routecontract "card-arena/api/internal/api/route"
	"card-arena/api/internal/realtime"
	"github.com/coder/websocket"
)

const (
	wsAuthTimeout       = 10 * time.Second
	wsWriteTimeout      = 10 * time.Second
	wsHeartbeatInterval = 25 * time.Second
	wsReadLimit         = 4096
	wsMaxSubscriptions  = 64

	wsCloseUnauthorized websocket.StatusCode = 4401
	wsCloseResync       websocket.StatusCode = 4409
)

type websocketRoute struct{ routecontract.Route }

func (websocketRoute) Handler() routecontract.Handler {
	return func(context *Context) {
		server, ok := context.Server.(*Server)
		if !ok {
			routecontract.NotImplemented(context)
			return
		}
		server.serveWebSocket(context.Writer, context.Request)
	}
}

var websocketDefinition = websocketRoute{Route: routecontract.NewRoute(http.MethodGet, "/api/v1/ws", routecontract.AccessPublic)}

type wsSession struct {
	server   *Server
	conn     *websocket.Conn
	user     *authUser
	sub      *realtime.Subscriber
	outbound chan []byte
	topics   int
}

func (server *Server) serveWebSocket(writer http.ResponseWriter, request *http.Request) {
	conn, err := websocket.Accept(writer, request, &websocket.AcceptOptions{OriginPatterns: server.cfg.CORSAllowedOrigins, InsecureSkipVerify: server.allowsAnyOrigin()})
	if err != nil {
		return
	}
	conn.SetReadLimit(wsReadLimit)
	session := &wsSession{server: server, conn: conn, outbound: make(chan []byte, 16)}
	session.run()
}

func (server *Server) allowsAnyOrigin() bool {
	for _, origin := range server.cfg.CORSAllowedOrigins {
		if origin == "*" {
			return true
		}
	}
	return false
}

func (session *wsSession) run() {
	ctx, cancel := context.WithCancel(session.server.ctx)
	defer cancel()
	if !session.authenticate(ctx) {
		return
	}
	userID := ""
	if session.user != nil {
		userID = session.user.ID
	}
	session.sub = session.server.hub.Register(userID)
	session.server.userConnected(userID)
	defer session.cleanup()
	session.enqueue(realtime.ServerMessage{Type: realtime.ServerReady, UserID: optionalString(userID)})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		session.writeLoop(ctx)
		cancel()
	}()
	session.readLoop(ctx)
	cancel()
	<-writerDone
}

func (session *wsSession) authenticate(ctx context.Context) bool {
	authCtx, cancel := context.WithTimeout(ctx, wsAuthTimeout)
	defer cancel()
	message, ok := session.read(authCtx)
	if !ok || message.Type != realtime.ClientAuth {
		session.conn.Close(websocket.StatusPolicyViolation, "authentication frame expected")
		return false
	}
	token := strings.TrimSpace(message.Token)
	if token == "" {
		return true
	}
	user := session.server.authenticateToken(token)
	if user == nil {
		session.send(ctx, realtime.ServerMessage{Type: realtime.ServerError, Code: "invalid_token", Message: "The session token is invalid or expired"})
		session.conn.Close(wsCloseUnauthorized, "invalid token")
		return false
	}
	session.user = user
	return true
}

func (session *wsSession) read(ctx context.Context) (realtime.ClientMessage, bool) {
	kind, data, err := session.conn.Read(ctx)
	if err != nil {
		return realtime.ClientMessage{}, false
	}
	if kind != websocket.MessageText {
		return realtime.ClientMessage{}, false
	}
	var message realtime.ClientMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return realtime.ClientMessage{}, false
	}
	return message, true
}

func (session *wsSession) readLoop(ctx context.Context) {
	for {
		message, ok := session.read(ctx)
		if !ok {
			return
		}
		switch message.Type {
		case realtime.ClientSubscribe:
			session.subscribe(message.Topic)
		case realtime.ClientUnsubscribe:
			session.unsubscribe(message.Topic)
		case realtime.ClientPing:
			session.enqueue(realtime.ServerMessage{Type: realtime.ServerPong})
		case realtime.ClientAuth:
			session.enqueue(realtime.ServerMessage{Type: realtime.ServerError, Code: "already_authenticated", Message: "Open a new connection to change the session"})
		default:
			session.enqueue(realtime.ServerMessage{Type: realtime.ServerError, Code: "unknown_message", Message: "Unsupported message type"})
		}
	}
}

func (session *wsSession) writeLoop(ctx context.Context) {
	heartbeat := time.NewTicker(wsHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-session.sub.Dropped:
			session.conn.Close(wsCloseResync, "subscriber fell behind")
			return
		case data := <-session.outbound:
			if !session.write(ctx, data) {
				return
			}
		case event := <-session.sub.Events:
			data, ok := realtime.Encode(event, session.sub.UserID)
			if !ok {
				continue
			}
			if !session.write(ctx, data) {
				return
			}
		case <-heartbeat.C:
			data, _ := json.Marshal(realtime.ServerMessage{Type: realtime.ServerHeartbeat})
			if !session.write(ctx, data) {
				return
			}
		}
	}
}

func (session *wsSession) write(ctx context.Context, data []byte) bool {
	writeCtx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
	defer cancel()
	return session.conn.Write(writeCtx, websocket.MessageText, data) == nil
}

func (session *wsSession) send(ctx context.Context, message realtime.ServerMessage) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}
	session.write(ctx, data)
}

func (session *wsSession) enqueue(message realtime.ServerMessage) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}
	select {
	case session.outbound <- data:
	default:
		session.sub.Drop()
	}
}

func (session *wsSession) subscribe(topic string) {
	if !realtime.ValidTopic(topic) {
		session.enqueue(realtime.ServerMessage{Type: realtime.ServerError, Topic: topic, Code: "invalid_topic", Message: "Unknown topic"})
		return
	}
	if session.topics >= wsMaxSubscriptions {
		session.enqueue(realtime.ServerMessage{Type: realtime.ServerError, Topic: topic, Code: "too_many_subscriptions", Message: "Subscription limit reached"})
		return
	}
	if code, message := session.server.authorizeTopic(topic, session.user); code != "" {
		session.enqueue(realtime.ServerMessage{Type: realtime.ServerError, Topic: topic, Code: code, Message: message})
		return
	}
	if !session.server.hub.Subscribe(session.sub, topic) {
		session.enqueue(realtime.ServerMessage{Type: realtime.ServerSubscribed, Topic: topic})
		return
	}
	session.topics++
	if strings.HasPrefix(topic, "game:") {
		session.server.gameSubscribed(realtime.TopicID(topic), session.sub.UserID)
	}
	session.enqueue(realtime.ServerMessage{Type: realtime.ServerSubscribed, Topic: topic})
}

func (session *wsSession) unsubscribe(topic string) {
	if !session.server.hub.Unsubscribe(session.sub, topic) {
		return
	}
	session.topics--
	if strings.HasPrefix(topic, "game:") {
		session.server.gameUnsubscribed(realtime.TopicID(topic), session.sub.UserID)
	}
	session.enqueue(realtime.ServerMessage{Type: realtime.ServerUnsubscribed, Topic: topic})
}

func (session *wsSession) cleanup() {
	topics := session.server.hub.Unregister(session.sub)
	for _, topic := range topics {
		if strings.HasPrefix(topic, "game:") {
			session.server.gameUnsubscribed(realtime.TopicID(topic), session.sub.UserID)
		}
	}
	session.server.userDisconnected(session.sub.UserID)
	session.conn.Close(websocket.StatusNormalClosure, "")
}

func (server *Server) authorizeTopic(topic string, user *authUser) (string, string) {
	switch {
	case topic == realtime.TopicLobbies, topic == realtime.TopicLeaderboard, strings.HasPrefix(topic, "profile:"):
		return "", ""
	case strings.HasPrefix(topic, "user:"):
		if user == nil || user.ID != realtime.TopicID(topic) {
			return "forbidden", "You can only follow your own account"
		}
		return "", ""
	case strings.HasPrefix(topic, "game:"):
		if user == nil {
			return "authentication_required", "Sign in to follow a game"
		}
		var exists int
		if err := server.store.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM games WHERE id=?)`, realtime.TopicID(topic)).Scan(&exists); err != nil || exists == 0 {
			return "game_not_found", "Game was not found"
		}
		return "", ""
	}
	return "invalid_topic", "Unknown topic"
}

func (server *Server) authenticateToken(token string) *authUser {
	if server.cfg.DebugAuthEnabled && token == "debug" {
		return server.findAuthUserByUsername(server.cfg.DebugAuthUsername)
	}
	hash := tokenHash(token)
	var user authUser
	err := server.store.DB.QueryRow(`
		SELECT u.id, u.username, u.role, u.elo
		FROM auth_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND u.deleted_at IS NULL`, hash, nowString()).Scan(&user.ID, &user.Username, &user.Role, &user.Elo)
	if err != nil {
		return nil
	}
	return &user
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
