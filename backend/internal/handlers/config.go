package handlers

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

	AppEnv          string
	WSAllowedOrigins []string
}

func LoadConfigFromEnv() (Config, error) {
	ttlMinutes := int64(10080) // 7 days
	if v := os.Getenv("JWT_TTL_MINUTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			ttlMinutes = n
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

	var missing []string
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if cfg.DatabasePath == "" {
		missing = append(missing, "DATABASE_PATH")
	}
	if cfg.Addr == "" {
		missing = append(missing, "BACKEND_ADDR")
	}
	if cfg.JWTTTL <= 0 {
		missing = append(missing, "JWT_TTL_MINUTES")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing/invalid env: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}


