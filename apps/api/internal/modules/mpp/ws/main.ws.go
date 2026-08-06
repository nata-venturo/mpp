package ws

import (
	"context"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"

	"github.com/ndollem/mpp/apps/api/internal/middleware"
)

type Module struct {
	Hub *Hub
}

// Initialize builds the hub and starts its Redis subscription. The
// returned module is wired into other MPP modules so they can publish.
func Initialize(ctx context.Context, redis *goredis.Client) *Module {
	hub := NewHub(redis)
	go hub.Run(ctx)

	return &Module{Hub: hub}
}

// SetupRoutes registers the upgrade endpoint. JWTAuth covers both staff
// JWTs and device API keys; the socket carries no data a subscriber
// could not already read over REST with the same credentials.
func (m *Module) SetupRoutes(rg *gin.RouterGroup) {
	rg.GET("/ws", credentialsFromQuery(), middleware.JWTAuth(), func(c *gin.Context) {
		Serve(m.Hub, c)
	})
}

// credentialsFromQuery lets a browser authenticate a WebSocket.
//
// The browser WebSocket API cannot set request headers, so the token has
// to arrive as a query parameter; this promotes it to the header JWTAuth
// reads, leaving exactly one auth implementation. Header credentials
// always win, so a non-browser client is unaffected.
//
// ponytail: query strings can end up in access logs. If that matters,
// swap to a short-lived single-use ticket minted over REST and redeemed
// here — the change stays inside this function.
func credentialsFromQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-API-Key") == "" {
			if key := c.Query("api_key"); key != "" {
				c.Request.Header.Set("X-API-Key", key)
			}
		}
		if c.GetHeader("Authorization") == "" {
			if token := c.Query("token"); token != "" {
				c.Request.Header.Set("Authorization", "Bearer "+token)
			}
		}

		c.Next()
	}
}
