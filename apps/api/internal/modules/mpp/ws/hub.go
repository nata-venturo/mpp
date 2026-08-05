// Package ws is the MPP realtime fan-out: one WebSocket endpoint, one
// hub per API instance, Redis pub/sub between instances.
//
// Redis is what makes the hub horizontally safe. A call published by the
// instance serving the loket app must reach the TV socket held by a
// different instance, so every event goes out through Redis and comes
// back in through one subscription that fans out to local sockets. An
// instance never delivers straight to its own clients — that would make
// the local path and the remote path behave differently.
package ws

import (
	"context"
	"encoding/json"
	"sync"

	goredis "github.com/redis/go-redis/v9"

	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// channelPrefix namespaces MPP pub/sub keys inside a shared Redis DB.
const channelPrefix = "mpp:ws:"

// Event is the server → client envelope. `Type` is the event name from
// docs/04-api/websocket-events.md; the rest is the event's own payload,
// flattened into the same object.
type Event struct {
	Type    string         `json:"type"`
	Channel string         `json:"channel,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// SnapshotProvider rebuilds current state for a channel so a late joiner
// or a reconnecting device re-syncs without replaying missed deltas.
// The display module implements it; until then subscribes get an empty
// snapshot, which is a valid answer.
type SnapshotProvider interface {
	SnapshotForChannel(ctx context.Context, channel string) (map[string]any, error)
}

// Hub owns the local socket registry and the Redis subscription.
type Hub struct {
	redis *goredis.Client

	mu       sync.RWMutex
	channels map[string]map[*Client]struct{}

	snapshots SnapshotProvider
}

func NewHub(redis *goredis.Client) *Hub {
	return &Hub{
		redis:    redis,
		channels: make(map[string]map[*Client]struct{}),
	}
}

// SetSnapshotProvider wires the state rebuilder after both modules
// exist (the display module needs the hub, and the hub needs it back).
func (h *Hub) SetSnapshotProvider(p SnapshotProvider) {
	h.snapshots = p
}

// Run subscribes to every MPP channel and fans messages out to local
// sockets until ctx is cancelled. One pattern subscription is enough:
// the channel count is small and bounded by the agency/service count.
func (h *Hub) Run(ctx context.Context) {
	sub := h.redis.PSubscribe(ctx, channelPrefix+"*")
	defer func() { _ = sub.Close() }()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub.Channel():
			if !ok {
				return
			}
			h.dispatch(msg.Channel[len(channelPrefix):], []byte(msg.Payload))
		}
	}
}

// Publish sends an event to every subscriber of a channel, on any
// instance. Failure to publish is logged, never fatal: the queue must
// keep working when realtime is degraded (REST stays the source of truth).
func (h *Hub) Publish(ctx context.Context, channel string, event Event) {
	event.Channel = channel

	payload, err := json.Marshal(event)
	if err != nil {
		logger.Error("Failed to encode ws event", logger.Err(err))
		return
	}

	if err := h.redis.Publish(ctx, channelPrefix+channel, payload).Err(); err != nil {
		logger.Warn("Failed to publish ws event",
			logger.String("channel", channel), logger.Err(err))
	}
}

// PublishAll is the common case: one transition that several audiences
// care about (the service stream, the loket, the TV, the agency feed).
func (h *Hub) PublishAll(ctx context.Context, channels []string, event Event) {
	for _, c := range channels {
		h.Publish(ctx, c, event)
	}
}

func (h *Hub) dispatch(channel string, payload []byte) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.channels[channel]))
	for c := range h.channels[channel] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.trySend(payload)
	}
}

func (h *Hub) subscribe(c *Client, channels []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, ch := range channels {
		if h.channels[ch] == nil {
			h.channels[ch] = make(map[*Client]struct{})
		}
		h.channels[ch][c] = struct{}{}
		c.channels[ch] = struct{}{}
	}
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range c.channels {
		if set := h.channels[ch]; set != nil {
			delete(set, c)
			if len(set) == 0 {
				delete(h.channels, ch)
			}
		}
	}
}

// snapshotFor builds the re-sync payload for a channel. An error or a
// missing provider degrades to an empty snapshot rather than dropping
// the connection — a TV with no data still has to stay connected.
func (h *Hub) snapshotFor(ctx context.Context, channel string) Event {
	event := Event{Type: "snapshot", Channel: channel, Data: map[string]any{}}
	if h.snapshots == nil {
		return event
	}

	data, err := h.snapshots.SnapshotForChannel(ctx, channel)
	if err != nil {
		logger.Warn("Failed to build ws snapshot",
			logger.String("channel", channel), logger.Err(err))
		return event
	}
	if data != nil {
		event.Data = data
	}

	return event
}

// Channel name builders — one place so publishers and subscribers can
// never drift apart.

func ChannelLayanan(id string) string  { return "layanan:" + id }
func ChannelLoket(id string) string    { return "loket:" + id }
func ChannelDisplay(id string) string  { return "display:" + id }
func ChannelInstansi(pfx string) string { return "instansi:" + pfx }

const ChannelMonitoring = "monitoring"
