package httpServer

import (
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func TestLimiterSeparatesClientsByXRealIP(t *testing.T) {
	app := fiber.New(fiber.Config{ProxyHeader: "X-Real-IP"})
	app.Use(limiter.New(limiter.Config{
		Max:               1,
		Expiration:        time.Minute,
		LimiterMiddleware: limiter.SlidingWindow{},
		LimitReached: func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusTooManyRequests)
		},
		KeyGenerator: ipLimiterConfig(nil).KeyGenerator,
	}))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	get := func(ip string) int {
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Set("X-Real-IP", ip)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}

	if code := get("1.1.1.1"); code != fiber.StatusOK {
		t.Fatalf("first 1.1.1.1 = %d, want 200", code)
	}
	if code := get("1.1.1.1"); code != fiber.StatusTooManyRequests {
		t.Fatalf("second 1.1.1.1 = %d, want 429", code)
	}
	if code := get("2.2.2.2"); code != fiber.StatusOK {
		t.Fatalf("first 2.2.2.2 = %d, want 200", code)
	}
}
