package auth

import (
	"fmt"
	"time"

	"fifteen-thirty-one-go/backend/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(userID int64, username string, cfg config.Config) (string, error) {
	if cfg.JWTSecret == "" {
		return "", fmt.Errorf("JWT_SECRET is required")
	}
	now := time.Now().UTC()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.JWTTTL)),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(cfg.JWTSecret))
}

func ParseAndValidateToken(tokenString string, cfg config.Config) (*Claims, error) {
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	tok, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.JWTSecret), nil
	},
		jwt.WithIssuer(cfg.JWTIssuer),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
