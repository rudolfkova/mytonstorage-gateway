package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"

	remotetonstorage "mytonstorage-gateway/pkg/clients/remote-ton-storage"
	tonstorage "mytonstorage-gateway/pkg/clients/ton-storage"
	"mytonstorage-gateway/pkg/httpServer"
	filesRepository "mytonstorage-gateway/pkg/repositories/files"
	filesService "mytonstorage-gateway/pkg/services/files"
	reportsService "mytonstorage-gateway/pkg/services/reports"
	htmlTemplates "mytonstorage-gateway/pkg/templates"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() (err error) {
	// Tools
	config := loadConfig()
	if config == nil {
		fmt.Println("failed to load configuration")
		return
	}

	logLevel := slog.LevelInfo
	if level, ok := logLevels[config.System.LogLevel]; ok {
		logLevel = level
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Metrics
	dbRequestsCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Metrics.Namespace,
			Subsystem: config.Metrics.DbSubsystem,
			Name:      "db_requests_count",
			Help:      "Db requests count",
		},
		[]string{"method", "error"},
	)

	dbRequestsDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Metrics.Namespace,
			Subsystem: config.Metrics.DbSubsystem,
			Name:      "db_requests_duration",
			Help:      "Db requests duration",
		},
		[]string{"method", "error"},
	)

	dbRequestsInFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: config.Metrics.Namespace,
		Subsystem: config.Metrics.DbSubsystem,
		Name:      "db_requests_in_flight",
		Help:      "Current number of in-flight DB requests",
	})

	prometheus.MustRegister(
		dbRequestsCount,
		dbRequestsDuration,
		dbRequestsInFlight,
	)

	// Postgres
	connPool, err := connectPostgres(context.Background(), config, logger)
	if err != nil {
		logger.Error("failed to connect to Postgres", slog.String("error", err.Error()))
		return
	}

	// Database
	filesRepo := filesRepository.NewRepository(connPool)
	filesRepo = filesRepository.NewCache(filesRepo)
	filesRepo = filesRepository.NewMetrics(dbRequestsCount, dbRequestsDuration, dbRequestsInFlight, filesRepo)

	// Clients
	creds := tonstorage.Credentials{
		Login:    config.TONStorage.Login,
		Password: config.TONStorage.Password,
	}
	storage := tonstorage.NewClient(
		config.TONStorage.BaseURL,
		&creds)

	rBagsCache := remotetonstorage.NewBagsCache(config.RemoteTONStorageCache.MaxCacheEntries)
	rRemoteMetrics := remotetonstorage.NewRemoteTONStorageMetrics(config.Metrics.Namespace, "remote-ton-storage")
	rstorage, err := remotetonstorage.NewClient(context.Background(), "", rBagsCache, rRemoteMetrics)
	if err != nil {
		logger.Error("failed to create remote TON Storage client", slog.String("error", err.Error()))
		return
	}
	defer func() {
		if rstorage != nil {
			rstorage.Close()
		}
	}()

	// Services
	filesSvc := filesService.NewService(filesRepo, storage, rstorage, logger)
	filesSvc = filesService.NewCacheMiddleware(filesSvc)

	reportsSvc := reportsService.NewService(filesRepo, logger)
	// TODO:
	// reportsSvc = reportsService.NewCacheMiddleware(reportsSvc)

	templatesSvc, err := htmlTemplates.New(config.Templates.Path)
	if err != nil {
		logger.Error("failed to initialize templates", slog.String("error", err.Error()))
		return
	}

	// HTTP Server
	accessTokens := strings.Split(config.System.AccessTokens, ";")
	app := fiber.New()
	server := httpServer.New(
		app,
		filesSvc,
		reportsSvc,
		templatesSvc,
		accessTokens,
		config.Metrics.Namespace,
		config.Metrics.ServerSubsystem,
		logger,
	)
	if server == nil {
		return fmt.Errorf("failed to create HTTP server handler")
	}

	server.RegisterRoutes()

	go func() {
		if err := app.Listen(":" + config.System.Port); err != nil {
			logger.Error("error starting server", slog.String("err", err.Error()))
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan

	if rstorage != nil {
		rstorage.Close()
		logger.Info("remote TON Storage client closed")
	}

	err = app.ShutdownWithTimeout(time.Second * 5)
	if err != nil {
		logger.Error("server shut down error", slog.String("err", err.Error()))
		return err
	}

	return err
}
