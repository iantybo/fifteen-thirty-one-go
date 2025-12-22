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

func LoadFromEnv() (Config, error) {
	ttlMinutes := int64(10080) // 7 days
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
		}
	}
	if v := strings.TrimSpace(os.Getenv("DEV_WEBSOCKETS_ALLOW_ALL")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.DevWebSocketsAllowAll = b
		}
	}

	var missing []string
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if cfg.DatabasePath == "" {
		missing = append(missing, "DATABASE_PATH")
	}
	// BACKEND_ADDR is optional if PORT is set by the hosting environment.
	if cfg.Addr == "" {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			// If PORT is a bare port, accept ":<port>". If it already includes host, keep it.
			if strings.Contains(port, ":") {
				cfg.Addr = port
			} else {
				cfg.Addr = ":" + port
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


