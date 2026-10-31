//go:build !debug
// +build !debug

package httpServer

import (
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func (h *handler) RegisterRoutes() {
	h.logger.Info("Registering routes")

	m := newMetrics(h.namespace, h.subsystem)

	h.server.Use(m.metricsMiddleware)

	h.server.Use(limiter.New(ipLimiterConfig(h.limitReached)))

	apiv1 := h.server.Group("/api/v1", h.loggerMiddleware)

	apiv1.Get("/health", h.health)
	apiv1.Get("/metrics", h.requireMetrics(), h.metrics)

	{
		gateway := apiv1.Group("/gateway", h.securityHeadersMiddleware)

		gateway.Get("/:bagid", h.getBag)
		gateway.Get("/:bagid/*", h.getPath)
	}

	{
		reports := apiv1.Group("/reports")

		reports.Get("", h.requireReports(), h.getReports)
		reports.Post("", h.requireReports(), h.addReport)
		reports.Get("/:bagid", h.requireReports(), h.getReportsByBagID)
	}

	{
		bans := apiv1.Group("/bans")

		bans.Get("", h.requireBans(), h.getAllBans)
		bans.Put("", h.requireBans(), h.updateBanStatus)
		bans.Get("/:bagid", h.requireBans(), h.getBan)
	}
}
