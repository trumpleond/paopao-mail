package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration for the server.
type Config struct {
	Addr               string
	APIKey             string
	DBPath             string
	UpstreamBase       string
	UpstreamTimeoutSec int
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	return Config{
		Addr:               getEnv("ADDR", ":8080"),
		APIKey:             getEnv("API_KEY", ""),
		DBPath:             getEnv("DB_PATH", "./data/paopao.db"),
		UpstreamBase:       strings.TrimRight(getEnv("UPSTREAM_BASE", "https://query.paopaodw.com"), "/"),
		UpstreamTimeoutSec: getEnvInt("UPSTREAM_TIMEOUT_SEC", 30),
	}
}

func getEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
