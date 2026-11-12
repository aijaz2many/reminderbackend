package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	JWTSecret          []byte
	JWTTTL             time.Duration
	Environment        string
	CORSAllowedOrigins []string
}

func Load() (Config, error) {
	if err := LoadEnv(); err != nil {
		return Config{}, err
	}

	port := envOrDefault("PORT", "8080")
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 || strings.TrimSpace(port) != port {
		return Config{}, errors.New("PORT must be a valid port number")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be at least 32 bytes")
	}

	jwtTTL := 24 * time.Hour
	if value := os.Getenv("JWT_TTL"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return Config{}, errors.New("JWT_TTL must be a positive duration such as 24h")
		}
		jwtTTL = parsed
	}

	return Config{
		Port:               port,
		DatabaseURL:        databaseURL,
		JWTSecret:          []byte(secret),
		JWTTTL:             jwtTTL,
		Environment:        envOrDefault("APP_ENV", "development"),
		CORSAllowedOrigins: splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS")),
	}, nil
}

// LoadEnv loads values from .env without overriding variables already set by the process.
func LoadEnv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
