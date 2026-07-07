package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		wantStatus int
	}{
		{"allowed role", "owner", http.StatusOK},
		{"disallowed role", "customer", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := tt.role
			router := gin.New()
			router.GET("/owner-only", func(c *gin.Context) {
				c.Set("role", role)
			}, RequireRole("owner"), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/owner-only", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestRequireOwnShop(t *testing.T) {
	tests := []struct {
		name       string
		jwtShopID  string
		pathShopID string
		wantStatus int
	}{
		{"matching shop", "shop1", "shop1", http.StatusOK},
		{"mismatched shop", "shop1", "shop2", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwtShopID := tt.jwtShopID
			router := gin.New()
			router.GET("/shops/:shopId/secret", func(c *gin.Context) {
				c.Set("shop_id", jwtShopID)
			}, RequireOwnShop(), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/shops/"+tt.pathShopID+"/secret", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
