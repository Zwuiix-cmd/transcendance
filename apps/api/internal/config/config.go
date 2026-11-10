package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the complete runtime configuration of the REST API.
type Config struct {
	Environment				 string
	Port                     string
	APIKey                   string
	DatabasePath             string
	SchemaPath               string
	SessionTTL               time.Duration
	CORSAllowedOrigins       []string
	AdminUsernames           []string
	AdminEmails              []string
	CDNURL                   string
	CDNPublicURL             string
	RedisURL                 string
	InternalCDNAPIKey        string
	GameServiceKey           string
	DebugAuthEnabled         bool
	DebugAuthUsername        string
	ValidRequestsPerMinute   int
	InvalidRequestsPerMinute int
	AdminRequestsPerMinute   int
	OTELTracesStdout         bool
}

// Load reads configuration from environment variables. The defaults are safe
// for local development and can be overridden by Docker Compose.
func Load() Config {
	return Config{
		Environment:			  envOr("APP_ENV", "development"),
		Port:                     envOr("PORT", "8080"),
		APIKey:                   envOr("API_KEY", "dev-only-change-me"),
		DatabasePath:             envOr("DATABASE_PATH", "./data/transcendance.db"),
		SchemaPath:               envOr("SCHEMA_PATH", "apps/api/db/schema.sql"),
		SessionTTL:               parseDuration(envOr("SESSION_TTL", "24h"), 24*time.Hour),
		CORSAllowedOrigins:       splitList(envOr("CORS_ALLOWED_ORIGINS", "https://localhost")),
		AdminUsernames:           splitList(os.Getenv("ADMIN_USERNAMES")),
		AdminEmails:              splitList(os.Getenv("ADMIN_EMAILS")),
		CDNURL:                   strings.TrimRight(envOr("CDN_URL", "http://cdn:8091"), "/"),
		CDNPublicURL:             strings.TrimRight(envOr("CDN_PUBLIC_URL", "/files"), "/"),
		RedisURL:                 strings.TrimSpace(os.Getenv("REDIS_URL")),
		InternalCDNAPIKey:        os.Getenv("INTERNAL_CDN_API_KEY"),
		GameServiceKey:           os.Getenv("GAME_SERVICE_KEY"),
		DebugAuthEnabled:         parseBool(os.Getenv("DEBUG_AUTH_ENABLED")),
		DebugAuthUsername:        envOr("DEBUG_AUTH_USERNAME", "Zwuiix"),
		ValidRequestsPerMinute:   parseInt(envOr("VALID_REQUESTS_PER_MINUTE", "120"), 120),
		InvalidRequestsPerMinute: parseInt(envOr("INVALID_REQUESTS_PER_MINUTE", "10"), 10),
		AdminRequestsPerMinute:   parseInt(envOr("ADMIN_REQUESTS_PER_MINUTE", "60"), 60),
		OTELTracesStdout:         parseBool(os.Getenv("OTEL_TRACES_STDOUT")),
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func splitList(value string) []string {
	items := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func parseBool(value string) bool {
	parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
	return parsed
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err == nil && parsed > 0 {
		return parsed
	}
	// Keep compatibility with the existing Compose configuration (PT24H).
	if strings.HasPrefix(value, "PT") && strings.HasSuffix(value, "H") {
		hours, parseErr := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(value, "PT"), "H"))
		if parseErr == nil && hours > 0 {
			return time.Duration(hours) * time.Hour
		}
	}
	return fallback
}
