package config

import (
	"os"
	"strings"
)

const DefaultPublicPath = "/files"

// Config contains the runtime settings for the file CDN.
type Config struct {
	Port                  string
	StorageDir            string
	StaticAssetsDir       string
	PublicBaseURL         string
	InternalAPIKey        string
	AvatarGeneratorScript string
}

// Load reads the CDN configuration from environment variables.
func Load() Config {
	return Config{
		Port:                  envOr("PORT", "8091"),
		StorageDir:            envOr("STORAGE_DIR", "/data/files"),
		StaticAssetsDir:       envOr("STATIC_ASSETS_DIR", "/data/files/uno"),
		PublicBaseURL:         strings.TrimRight(envOr("PUBLIC_BASE_URL", DefaultPublicPath), "/"),
		InternalAPIKey:        os.Getenv("INTERNAL_API_KEY"),
		AvatarGeneratorScript: envOr("AVATAR_GENERATOR_SCRIPT", "scripts/avatar_generator.py"),
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
