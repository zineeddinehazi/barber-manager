package utils

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"barbermanager/internal/config"
	"barbermanager/internal/models"
)

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	ShopID string `json:"shop_id"`
	jwt.RegisteredClaims
}

func GenerateToken(user *models.User, cfg *config.Config) (string, error) {
	shopID := ""
	if user.ShopID != nil {
		shopID = *user.ShopID
	}

	claims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		ShopID: shopID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(cfg.JWTExpiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}
