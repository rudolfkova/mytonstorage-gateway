package main

import (
	"log"
	"log/slog"

	"github.com/caarlos0/env/v11"
)

var logLevels = map[uint8]slog.Level{
	0: slog.LevelDebug,
	1: slog.LevelInfo,
	2: slog.LevelWarn,
	3: slog.LevelError,
}

type System struct {
	Port string `env:"SYSTEM_PORT" envDefault:"9093"`

	// AccessTokens format: "hash1:bans,reports,metrics;hash2:metrics;hash3"
	// Tokens separated by semicolon (;), permissions by comma (,)
	// Permissions: bans, reports, metrics, all
	// If no permissions specified - all permissions granted.
	AccessTokens string `env:"SYSTEM_ACCESS_TOKENS" envDefault:""`
	LogLevel     uint8  `env:"SYSTEM_LOG_LEVEL" envDefault:"1"` // 0 - debug, 1 - info, 2 - warn, 3 - error
}

type TONStorage struct {
	BaseURL  string `env:"TON_STORAGE_BASE_URL" required:"true"`
	Login    string `env:"TON_STORAGE_LOGIN" required:"true"`
	Password string `env:"TON_STORAGE_PASSWORD" required:"true"`
}

type RemoteTONStorageCache struct {
	MaxCacheEntries int `env:"REMOTE_TON_STORAGE_CACHE_MAX_ENTRIES" envDefault:"1000"`
}

type Metrics struct {
	Namespace        string `env:"NAMESPACE" envDefault:"ton-storage"`
	ServerSubsystem  string `env:"SERVER_SUBSYSTEM" envDefault:"mtpo-server"`
	WorkersSubsystem string `env:"WORKERS_SUBSYSTEM" envDefault:"mtpo-workers"`
	DbSubsystem      string `env:"DB_SUBSYSTEM" envDefault:"mtpo-db"`
}

type Postgress struct {
	Host     string `env:"DB_HOST" required:"true"`
	Port     string `env:"DB_PORT" required:"true"`
	User     string `env:"DB_USER" required:"true"`
	Password string `env:"DB_PASSWORD" required:"true"`
	Name     string `env:"DB_NAME" required:"true"`
}

type Templates struct {
	Path string `env:"TEMPLATES_PATH" envDefault:"../templates"`
}

type Config struct {
	System                System
	TONStorage            TONStorage
	RemoteTONStorageCache RemoteTONStorageCache
	Metrics               Metrics
	DB                    Postgress
	Templates             Templates
}

func loadConfig() *Config {
	cfg := &Config{}
	if err := env.Parse(&cfg.System); err != nil {
		log.Fatalf("Failed to parse system config: %v", err)
	}
	if err := env.Parse(&cfg.TONStorage); err != nil {
		log.Fatalf("Failed to parse TONStorage config: %v", err)
	}
	if err := env.Parse(&cfg.Metrics); err != nil {
		log.Fatalf("Failed to parse metrics config: %v", err)
	}
	if err := env.Parse(&cfg.RemoteTONStorageCache); err != nil {
		log.Fatalf("Failed to parse remote TON Storage cache config: %v", err)
	}
	if err := env.Parse(&cfg.DB); err != nil {
		log.Fatalf("Failed to parse db config: %v", err)
	}
	if err := env.Parse(&cfg.Templates); err != nil {
		log.Fatalf("Failed to parse templates config: %v", err)
	}

	return cfg
}
