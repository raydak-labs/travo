package ws

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"github.com/openwrt-travel-gui/backend/internal/auth"
)

// Handler returns a Fiber handler for WebSocket connections.
func Handler(hub *Hub, authSvc *auth.AuthService) fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		hub.Register(c)
		defer hub.Unregister(c)

		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
		}
	})
}

// UpgradeMiddleware checks if the request is a WebSocket upgrade and validates JWT token.
func UpgradeMiddleware(authSvc *auth.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !websocket.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}

		// Validate JWT from query parameter
		token := c.Query("token")
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing token",
			})
		}

		if err := authSvc.ValidateToken(token); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid token",
			})
		}

		// Check blocklist
		bl := authSvc.Blocklist()
		if bl != nil && bl.IsBlocked(token) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "token has been revoked",
			})
		}

		return c.Next()
	}
}
