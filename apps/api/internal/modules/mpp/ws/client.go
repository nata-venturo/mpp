package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

const (
	// sendBuffer absorbs a burst without blocking the fan-out. A client
	// that cannot keep up is dropped rather than allowed to stall every
	// other socket on the instance.
	sendBuffer = 32

	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second

	maxMessageSize = 4 << 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// The socket is authenticated by JWTAuth before the upgrade, and the
	// browser cannot read cross-origin socket data without the token, so
	// origin is not the security boundary here. CORS still governs REST.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client is one connected socket.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	channels map[string]struct{}
}

// subscribeFrame is the only client → server message the hub accepts.
type subscribeFrame struct {
	Type     string   `json:"type"`
	Channels []string `json:"channels"`
}

// Serve upgrades the request and runs the socket until it closes.
func Serve(hub *Hub, c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade already wrote its own HTTP error.
		logger.Warn("WebSocket upgrade failed", logger.Err(err))
		return
	}

	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, sendBuffer),
		channels: make(map[string]struct{}),
	}

	go client.writeLoop()
	client.readLoop(c.Request.Context())
}

// trySend queues a payload, dropping the client when its buffer is full.
// A stuck TV must not become back-pressure on the queue engine.
func (c *Client) trySend(payload []byte) {
	select {
	case c.send <- payload:
	default:
		logger.Warn("Dropping slow WebSocket client")
		close(c.send)
		c.hub.remove(c)
	}
}

func (c *Client) readLoop(ctx context.Context) {
	defer func() {
		c.hub.remove(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var frame subscribeFrame
		if err := json.Unmarshal(message, &frame); err != nil || frame.Type != "subscribe" {
			// Unknown frames are ignored: the protocol is server→client
			// apart from subscribe, and a noisy client must not kill its
			// own connection.
			continue
		}

		c.hub.subscribe(c, frame.Channels)

		// Reply with one snapshot per channel so a reconnecting device
		// re-syncs from current state instead of replaying deltas.
		for _, ch := range frame.Channels {
			payload, err := json.Marshal(c.hub.snapshotFor(ctx, ch))
			if err != nil {
				continue
			}
			c.trySend(payload)
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case payload, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
