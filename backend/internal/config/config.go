package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr         string
	DatabasePath string

	JWTSecret string
	JWTIssuer string
	JWTTTL    time.Duration

	AppEnv           string
	WSAllowedOrigins []string
	WSAllowQueryTokens bool
	DevWebSocketsAllowAll bool
}

func isJWTSecretPlaceholder(secret string) bool {
	s := strings.ToUpper(strings.TrimSpace(secret))
	if s == "" {
		return true
	}
	// Common placeholder values (case-insensitive).
	switch s {
	case "REPLACE_ME",
		"CHANGE_ME",
		"CHANGEME",
		"CHANGE_ME_OR_APP_WILL_NOT_START":
		return true
	}
	// Clearly unsafe sentinels (prefix match).
	if strings.HasPrefix(s, "DO_NOT_USE_IN_PRODUCTION") {
		return true
	}
	return false
}

func LoadFromEnv() (Config, error) {
	ttlMinutes := int64(1440) // 24 hours
	if v := os.Getenv("JWT_TTL_MINUTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			ttlMinutes = n
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: invalid JWT_TTL_MINUTES=%q, using default %d\n", v, ttlMinutes)
		}
	}

	issuer := os.Getenv("JWT_ISSUER")
	if issuer == "" {
		issuer = "fifteen-thirty-one"
	}

	cfg := Config{
		Addr:         os.Getenv("BACKEND_ADDR"),
		DatabasePath: os.Getenv("DATABASE_PATH"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		JWTIssuer:    issuer,
		JWTTTL:       time.Duration(ttlMinutes) * time.Minute,
		AppEnv:       strings.TrimSpace(os.Getenv("APP_ENV")),
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = "development"
	}

	if v := os.Getenv("WS_ALLOWED_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.WSAllowedOrigins = append(cfg.WSAllowedOrigins, p)
			}
		}
	}

	if v := strings.TrimSpace(os.Getenv("WS_ALLOW_QUERY_TOKENS")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.WSAllowQueryTokens = b
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: invalid WS_ALLOW_QUERY_TOKENS=%q, using default false\n", v)
		}
	}
	if v := strings.TrimSpace(os.Getenv("DEV_WEBSOCKETS_ALLOW_ALL")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.DevWebSocketsAllowAll = b
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: invalid DEV_WEBSOCKETS_ALLOW_ALL=%q, using default false\n", v)
		}
	}

	// JWT secret validation:
	// - must be present (and not a placeholder)
	// - must be at least 32 bytes for HS256
	// NOTE: use raw byte length (len(secret)) as requested.
	cfg.JWTSecret = strings.TrimSpace(cfg.JWTSecret)
	if isJWTSecretPlaceholder(cfg.JWTSecret) {
		return Config{}, fmt.Errorf("JWT_SECRET is required; generate and set a strong secret (e.g., `openssl rand -hex 32`)")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 bytes (got %d)", len(cfg.JWTSecret))
	}

	var missing []string
	if cfg.DatabasePath == "" {
		missing = append(missing, "DATABASE_PATH")
	}
	// BACKEND_ADDR is optional if PORT is set by the hosting environment.
	if cfg.Addr == "" {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			// If PORT is a bare numeric port, treat it as ":<port>".
			// Otherwise treat it as already containing host / host:port (or ":<port>").
			onlyDigits := true
			for i := 0; i < len(port); i++ {
				if port[i] < '0' || port[i] > '9' {
					onlyDigits = false
					break
				}
			}
			if onlyDigits {
				cfg.Addr = ":" + port
			} else {
				cfg.Addr = port
			}
		}
	}
	if cfg.Addr == "" {
		missing = append(missing, "BACKEND_ADDR (or PORT)")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing/invalid env: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}


