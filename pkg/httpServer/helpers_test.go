package httpServer

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestShouldServeLoadingShell(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		headers map[string]string
		want    bool
	}{
		{
			name: "browser document navigation",
			url:  "/api/v1/gateway/abcd",
			headers: map[string]string{
				"Sec-Fetch-Dest": "document",
				"Accept":         "text/html,application/xhtml+xml",
			},
			want: true,
		},
		{
			name: "browser with direct=1",
			url:  "/api/v1/gateway/abcd?direct=1",
			headers: map[string]string{
				"Sec-Fetch-Dest": "document",
				"Accept":         "text/html",
			},
			want: false,
		},
		{
			name: "curl default accept",
			url:  "/api/v1/gateway/abcd",
			headers: map[string]string{
				"Accept": "*/*",
			},
			want: false,
		},
		{
			name: "legacy browser accept html no sec-fetch",
			url:  "/api/v1/gateway/abcd",
			headers: map[string]string{
				"Accept": "text/html,application/xhtml+xml,*/*",
			},
			want: true,
		},
		{
			name: "fetch empty dest",
			url:  "/api/v1/gateway/abcd",
			headers: map[string]string{
				"Sec-Fetch-Dest": "empty",
				"Accept":         "*/*",
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			var got bool
			app.Get("/api/v1/gateway/:bagid", func(c *fiber.Ctx) error {
				got = shouldServeLoadingShell(c)
				return c.SendStatus(fiber.StatusNoContent)
			})

			req := httptest.NewRequest(fiber.MethodGet, tc.url, nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)

			if got != tc.want {
				t.Fatalf("shouldServeLoadingShell = %v, want %v", got, tc.want)
			}
		})
	}
}
