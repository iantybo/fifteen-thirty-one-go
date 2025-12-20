package handlers

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr         string
	DatabasePath string

	JWTSecret string
	JWTIssuer string
	JWTTTL    time.Duration
}

func LoadConfigFromEnv() Config {
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

	return Config{
		Addr:         os.Getenv("BACKEND_ADDR"),
		DatabasePath: os.Getenv("DATABASE_PATH"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		JWTIssuer:    issuer,
		JWTTTL:       time.Duration(ttlMinutes) * time.Minute,
	}
}


