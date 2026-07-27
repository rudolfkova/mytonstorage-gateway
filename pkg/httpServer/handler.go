package httpServer

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"

	v1 "mytonstorage-gateway/pkg/models/api/v1"
	"mytonstorage-gateway/pkg/models/private"
	htmlTemplates "mytonstorage-gateway/pkg/templates"
)

type TokenPermissions struct {
	Bans    bool
	Reports bool
	Metrics bool
}

type files interface {
	GetPathInfo(ctx context.Context, bagID, path string) (private.FolderInfo, error)
}

type reports interface {
	GetReports(ctx context.Context, limit int, offset int) ([]v1.Report, error)
	GetReportsByBagID(ctx context.Context, bagID string) ([]v1.Report, error)
	GetBan(ctx context.Context, bagID string) (*v1.BanInfo, error)
	GetAllBans(ctx context.Context, limit int, offset int) ([]v1.BanInfo, error)
	AddReport(ctx context.Context, report v1.Report) error
	UpdateBanStatus(ctx context.Context, statuses []v1.BanStatus) error
}

type templatesSvc interface {
	ContentType(filename string) htmlTemplates.ContentType
	HtmlFilesListWithTemplate(f private.FolderInfo, path string) (string, error)
	HtmlLoadingPage() (string, error)
}

type errorResponse struct {
	Error string `json:"error"`
}

type handler struct {
	server       *fiber.App
	logger       *slog.Logger
	files        files
	reports      reports
	templates    templatesSvc
	namespace    string
	subsystem    string
	accessTokens map[string]TokenPermissions
}

func New(
	server *fiber.App,
	files files,
	reports reports,
	templates templatesSvc,
	accessTokens []string,
	namespace string,
	subsystem string,
	logger *slog.Logger,
) *handler {
	accessTokensMap := make(map[string]TokenPermissions)

	for _, token := range accessTokens {
		parts := strings.Split(token, ":")
		tokenHash := strings.TrimSpace(parts[0])

		if tokenHash == "" {
			continue
		}

		// default
		permissions := TokenPermissions{
			Bans:    true,
			Reports: true,
			Metrics: true,
		}

		if len(parts) > 1 {
			permissionsList := strings.Split(parts[1], ",")
			permissions = TokenPermissions{}

			for _, perm := range permissionsList {
				switch strings.TrimSpace(strings.ToLower(perm)) {
				case "bans":
					permissions.Bans = true
					permissions.Reports = true // bans permission includes reports
				case "reports":
					permissions.Reports = true
				case "metrics":
					permissions.Metrics = true
				case "all":
					permissions.Bans = true
					permissions.Reports = true
					permissions.Metrics = true
				}
			}
		}

		accessTokensMap[tokenHash] = permissions
	}

	h := &handler{
		server:       server,
		files:        files,
		reports:      reports,
		templates:    templates,
		namespace:    namespace,
		subsystem:    subsystem,
		accessTokens: accessTokensMap,
		logger:       logger,
	}

	return h
}
