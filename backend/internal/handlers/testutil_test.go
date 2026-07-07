package handlers

import (
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// withContext stands in for middleware.Auth in these unit tests: it sets
// user_id/role/shop_id directly in the Gin context.
func withContext(userID, role, shopID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("role", role)
		c.Set("shop_id", shopID)
	}
}

func newRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

func strPtr(s string) *string { return &s }
