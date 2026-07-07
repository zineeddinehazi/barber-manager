package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"barbermanager/internal/config"
	"barbermanager/internal/utils"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func signToken(t *testing.T, secret string, claims utils.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func TestAuth(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}

	validToken := signToken(t, cfg.JWTSecret, utils.Claims{
		UserID:           "u1",
		Role:             "customer",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	expiredToken := signToken(t, cfg.JWTSecret, utils.Claims{
		UserID:           "u1",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))},
	})
	wrongSecretToken := signToken(t, "some-other-secret", utils.Claims{
		UserID:           "u1",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{"valid token", "Bearer " + validToken, http.StatusOK},
		{"missing header", "", http.StatusUnauthorized},
		{"malformed header", "NotBearer xyz", http.StatusUnauthorized},
		{"expired token", "Bearer " + expiredToken, http.StatusUnauthorized},
		{"wrong secret", "Bearer " + wrongSecretToken, http.StatusUnauthorized},
		{"garbage token", "Bearer garbage", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/protected", Auth(cfg), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
