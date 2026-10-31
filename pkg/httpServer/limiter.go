package httpServer

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

const (
	MaxRequests     = 100
	RateLimitWindow = 60 * time.Second
)

func ipLimiterConfig(limitReached fiber.Handler) limiter.Config {
	return limiter.Config{
		Max:               MaxRequests,
		Expiration:        RateLimitWindow,
		LimitReached:      limitReached,
		LimiterMiddleware: limiter.SlidingWindow{},
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
	}
}
