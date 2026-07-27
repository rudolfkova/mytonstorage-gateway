package httpServer

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"

	"mytonstorage-gateway/pkg/constants"
	"mytonstorage-gateway/pkg/iframewrap"
	"mytonstorage-gateway/pkg/models"
	"mytonstorage-gateway/pkg/models/private"
)

func okHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
	})
}

// shouldServeLoadingShell is true for browser document navigations without ?direct=1.
// API clients (curl/Bruno with Accept */* and no Sec-Fetch-Dest) skip the shell.
func shouldServeLoadingShell(c *fiber.Ctx) bool {
	if c.Query("direct") == "1" {
		return false
	}

	secFetchDest := c.Get("Sec-Fetch-Dest")
	if secFetchDest == "document" {
		return true
	}
	if secFetchDest != "" {
		return false
	}

	accept := c.Get("Accept")
	return strings.Contains(accept, "text/html")
}

func errorHandler(c *fiber.Ctx, err error) error {
	if e, ok := err.(*fiber.Error); ok {
		return c.Status(e.Code).JSON(fiber.Map{
			"error": e.Message,
		})
	}

	if appErr, ok := err.(*models.AppError); ok {
		return c.Status(appErr.Code).JSON(fiber.Map{
			"error": appErr.Message,
		})
	}

	errorResponse := errorResponse{
		Error: err.Error(),
	}

	return c.Status(fiber.StatusInternalServerError).JSON(errorResponse)
}

func validateBagID(bagid string) bool {
	if len(bagid) != 64 {
		return false
	}

	bagid = strings.ToLower(bagid)
	for i := range 64 {
		c := bagid[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}

	return true
}

func sanitizePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || p == "." || p == "/" {
		return ".", nil
	}

	cleaned := filepath.Clean(p)

	if cleaned == "." {
		return ".", nil
	}

	if len(cleaned) > constants.MaxPathLength {
		return "", models.NewAppError(models.BadRequestErrorCode, "path too long")
	}

	segments := strings.Split(cleaned, string(filepath.Separator))
	for _, seg := range segments {
		if seg == ".." {
			return "", models.NewAppError(models.BadRequestErrorCode, "invalid segment in path")
		}
	}

	if strings.ContainsRune(cleaned, '\x00') {
		return "", models.NewAppError(models.BadRequestErrorCode, "invalid path")
	}

	return cleaned, nil
}

func mapPathInfoError(err error, bagInfo private.FolderInfo, log *slog.Logger) error {
	var appErr *models.AppError
	if errors.As(err, &appErr) {
		if appErr.Code == models.NotFoundErrorCode {
			pc := float64(bagInfo.PeersCount)
			if bagInfo.StreamFile != nil {
				pc = math.Max(float64(bagInfo.StreamFile.PeersCount), pc)
			}

			if pc > 0 {
				return fiber.NewError(fiber.StatusRequestTimeout, fmt.Sprintf("found %d peers, but request timed out", int(pc)))
			}
		}

		return fiber.NewError(appErr.Code, appErr.Message)
	}

	log.Error("failed to get bag path", slog.String("error", err.Error()))
	return fiber.NewError(fiber.StatusInternalServerError, "")
}

func serveHTMLFile(c *fiber.Ctx, bagInfo private.FolderInfo) error {
	var htmlContent string

	if bagInfo.StreamFile != nil {
		buf := make([]byte, bagInfo.StreamFile.Size)
		_, err := bagInfo.StreamFile.FileStream.Read(buf)
		bagInfo.StreamFile.FileStream.Close()
		if err != nil {
			return errorHandler(c, err)
		}

		htmlContent = string(buf)
	} else if bagInfo.SingleFilePath != "" {
		content, err := os.ReadFile(bagInfo.SingleFilePath)
		if err != nil {
			return errorHandler(c, err)
		}

		htmlContent = string(content)
	} else {
		return errorHandler(c, fiber.NewError(fiber.StatusNotFound, "file not found"))
	}

	iframeHTML, err := iframewrap.WrapHTML(htmlContent, iframewrap.Options{
		AllowScripts: true,
		AllowForms:   true,
	})
	if err != nil {
		log.Error("failed to create iframe wrapper", slog.String("error", err.Error()))
		err = models.NewAppError(models.InternalServerErrorCode, "")
		return errorHandler(c, err)
	}

	return c.SendString(iframeHTML)
}

func streamWithDeadline(c *fiber.Ctx, r io.Reader, size int) error {
	deadline := time.Now().Add(time.Second * constants.FileDownloadTimeoutSeconds)

	if size >= 0 {
		c.Response().Header.Set("Content-Length", fmt.Sprintf("%d", size))
	}

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer func() {
			_ = w.Flush()

			if rc, ok := r.(io.ReadCloser); ok {
				_ = rc.Close()
			}
		}()

		buf := make([]byte, 64*1024)
		for {
			if time.Now().After(deadline) {
				return
			}
			n, err := r.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	})
	return nil
}
