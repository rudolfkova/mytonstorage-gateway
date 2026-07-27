package httpServer

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"mytonstorage-gateway/pkg/constants"
	v1 "mytonstorage-gateway/pkg/models/api/v1"
	"mytonstorage-gateway/pkg/models/private"
	htmlTemplates "mytonstorage-gateway/pkg/templates"
)

func (h *handler) limitReached(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("method", "limitReached"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
	)

	log.Warn("rate limit reached for request")
	return fiber.NewError(fiber.StatusTooManyRequests, "too many requests, please try again later")
}

func (h *handler) getBag(c *fiber.Ctx) (err error) {
	bagid := strings.ToLower(c.Params("bagid"))

	log := h.logger.With(
		slog.String("func", "getBag"),
		slog.String("bagid", bagid),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
	)

	return h.getBagInfoResponse(c, bagid, "", log)
}

func (h *handler) getPath(c *fiber.Ctx) (err error) {
	bagid := strings.ToLower(c.Params("bagid"))
	rawPath := c.Params("*")

	log := h.logger.With(
		slog.String("func", "getPath"),
		slog.String("bagid", bagid),
		slog.String("raw_path", rawPath),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
	)

	decodedPath, err := url.QueryUnescape(rawPath)
	if err != nil {
		log.Error("failed to decode path", slog.String("error", err.Error()))
		err = fiber.NewError(fiber.StatusBadRequest, "invalid path encoding")
		return errorHandler(c, err)
	}

	return h.getBagInfoResponse(c, bagid, decodedPath, log)
}

func (h *handler) getReports(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("func", "getAllReports"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
	)

	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)

	reports, err := h.reports.GetReports(c.Context(), limit, offset)
	if err != nil {
		log.Error("failed to get reports", slog.String("error", err.Error()))
		return errorHandler(c, err)
	}

	return c.JSON(fiber.Map{
		"reports": reports,
	})
}

func (h *handler) getAllBans(c *fiber.Ctx) error {
	log := h.logger.With(
		slog.String("func", "getAllBans"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
	)

	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)

	bans, err := h.reports.GetAllBans(c.Context(), limit, offset)
	if err != nil {
		log.Error("failed to get bans", slog.String("error", err.Error()))
		return errorHandler(c, err)
	}

	return c.JSON(fiber.Map{
		"bans": bans,
	})
}

func (h *handler) updateBanStatus(c *fiber.Ctx) (err error) {
	body := c.Body()
	log := h.logger.With(
		slog.String("func", "updateBanStatus"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
		slog.Int("body_length", len(body)),
	)

	if len(body) == 0 || body[0] != '[' {
		err = fiber.NewError(fiber.StatusBadRequest, "invalid gzip body")
		return errorHandler(c, err)
	}

	var statuses []v1.BanStatus
	err = json.Unmarshal(body, &statuses)
	if err != nil {
		log.Error("failed to parse request body", slog.String("error", err.Error()))
		return errorHandler(c, fiber.NewError(fiber.StatusBadRequest, "invalid request body"))
	}

	if err := h.reports.UpdateBanStatus(c.Context(), statuses); err != nil {
		log.Error("failed to update ban status", slog.String("error", err.Error()))
		return errorHandler(c, err)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *handler) getReportsByBagID(c *fiber.Ctx) (err error) {
	bagID := strings.ToLower(c.Params("bagid"))
	log := h.logger.With(
		slog.String("func", "getReportsByBagID"),
		slog.String("bagID", bagID),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
	)

	if !validateBagID(bagID) {
		log.Error("invalid bagid format")
		err = fiber.NewError(fiber.StatusBadRequest, "invalid bagid")
		return errorHandler(c, err)
	}

	reports, err := h.reports.GetReportsByBagID(c.Context(), bagID)
	if err != nil {
		log.Error("failed to get report", slog.String("error", err.Error()))
		return errorHandler(c, err)
	}

	return c.JSON(fiber.Map{
		"reports": reports,
	})
}

func (h *handler) getBan(c *fiber.Ctx) (err error) {
	bagID := strings.ToLower(c.Params("bagid"))
	log := h.logger.With(
		slog.String("func", "getBan"),
		slog.String("bagID", bagID),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
	)

	if !validateBagID(bagID) {
		log.Error("invalid bagid format")
		err = fiber.NewError(fiber.StatusBadRequest, "invalid bagid")
		return errorHandler(c, err)
	}

	info, err := h.reports.GetBan(c.Context(), bagID)
	if err != nil {
		log.Error("failed to get ban", slog.String("error", err.Error()))
		return errorHandler(c, err)
	}

	return c.JSON(fiber.Map{
		"ban": info,
	})
}

func (h *handler) addReport(c *fiber.Ctx) (err error) {
	body := c.Body()
	log := h.logger.With(
		slog.String("func", "addReport"),
		slog.String("method", c.Method()),
		slog.String("url", c.OriginalURL()),
		slog.Any("headers", c.GetReqHeaders()),
		slog.Int("body_length", len(body)),
		slog.String("content_type", c.Get("Content-Type")),
	)

	if len(body) == 0 || body[0] != '{' {
		err = fiber.NewError(fiber.StatusBadRequest, "invalid gzip body")
		return errorHandler(c, err)
	}

	var report v1.Report
	if err := c.BodyParser(&report); err != nil {
		log.Error("failed to parse request body", slog.String("error", err.Error()))
		return errorHandler(c, fiber.NewError(fiber.StatusBadRequest, "invalid request body"))
	}

	report.BagID = strings.ToLower(report.BagID)

	if !validateBagID(report.BagID) {
		log.Error("invalid bagid format")
		err = fiber.NewError(fiber.StatusBadRequest, "invalid bagid")
		return errorHandler(c, err)
	}

	if err := h.reports.AddReport(c.Context(), report); err != nil {
		log.Error("failed to add report", slog.String("error", err.Error()))
		return errorHandler(c, err)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *handler) getBagInfoResponse(c *fiber.Ctx, bagid, path string, log *slog.Logger) (err error) {
	if !validateBagID(bagid) {
		log.Error("invalid bagid format")
		return errorHandler(c, fiber.NewError(fiber.StatusBadRequest, "invalid bagid"))
	}

	path, err = sanitizePath(path)
	if err != nil {
		log.Error("invalid path", "path", path)
		return errorHandler(c, err)
	}

	if shouldServeLoadingShell(c) {
		html, rerr := h.templates.HtmlLoadingPage()
		if rerr != nil {
			log.Error("failed to render loading template", slog.String("error", rerr.Error()))
			return errorHandler(c, fiber.NewError(fiber.StatusInternalServerError, ""))
		}
		return c.Type("html").SendString(html)
	}

	bagInfo, err := h.files.GetPathInfo(c.Context(), bagid, path)
	if err != nil {
		mapped := mapPathInfoError(err, bagInfo, log)
		return errorHandler(c, mapped)
	}

	// File response branch
	if bagInfo.StreamFile != nil || bagInfo.SingleFilePath != "" {
		if bagInfo.SingleFilePath != "" {
			sanitized, sErr := sanitizePath(bagInfo.SingleFilePath)
			if sErr != nil {
				log.Error("invalid single file path", slog.String("path", bagInfo.SingleFilePath))
				return errorHandler(c, sErr)
			}

			bagInfo.SingleFilePath = sanitized
			_, file := filepath.Split(bagInfo.SingleFilePath)
			return h.serveFile(c, bagInfo, h.templates.ContentType(file))
		}

		return h.serveFile(c, bagInfo, h.templates.ContentType(path))
	}

	// Directory listing
	html, rerr := h.templates.HtmlFilesListWithTemplate(bagInfo, path)
	if rerr != nil {
		log.Error("failed to render directory template", slog.String("error", rerr.Error()))
		return errorHandler(c, fiber.NewError(fiber.StatusInternalServerError, ""))
	}

	return c.Type("html").SendString(html)
}

func (h *handler) serveFile(c *fiber.Ctx, bagInfo private.FolderInfo, ct htmlTemplates.ContentType) error {
	// Force download for large HTML files
	if ct.IsHtml && bagInfo.SingleFilePath != "" {
		f, sErr := os.Lstat(bagInfo.SingleFilePath)
		if sErr != nil {
			return errorHandler(c, fiber.NewError(fiber.StatusNotFound, "file not found"))
		}

		if f.Size() >= constants.MaxHTMLFileSize {
			ct.Header = "Content-Disposition"
			ct.Value = `attachment; filename="` + bagInfo.SingleFilePath + `"`
			ct.IsDownload = true
			ct.IsHtml = false
		}
	}

	if ct.IsDownload {
		c.Response().Header.Set(ct.Header, ct.Value)
	} else {
		c.Response().Header.SetContentType(ct.Value)
	}

	if ct.IsHtml {
		return serveHTMLFile(c, bagInfo)
	}

	if bagInfo.StreamFile != nil {
		// Size check is done before on service layer
		return streamWithDeadline(c, bagInfo.StreamFile.FileStream, int(bagInfo.StreamFile.Size))
	} else if bagInfo.SingleFilePath != "" {
		f, err := os.Stat(bagInfo.SingleFilePath)
		if errors.Is(err, os.ErrNotExist) {
			return errorHandler(c, fiber.NewError(fiber.StatusNotFound, "file not found"))
		}

		if int(f.Size()) > constants.MaxFileServeSize {
			return errorHandler(c, fiber.NewError(fiber.StatusRequestEntityTooLarge, "file too large, use https://github.com/xssnick/TON-Torrent"))
		}
		file, err := os.Open(bagInfo.SingleFilePath)
		if err != nil {
			return errorHandler(c, fiber.NewError(fiber.StatusInternalServerError, ""))
		}
		return streamWithDeadline(c, file, int(f.Size()))
	}

	return errorHandler(c, fiber.NewError(fiber.StatusNotFound, "file not found"))
}

func (h *handler) health(c *fiber.Ctx) error {
	return c.JSON(okHandler(c))
}

func (h *handler) metrics(c *fiber.Ctx) error {
	m := promhttp.Handler()

	return adaptor.HTTPHandler(m)(c)
}
