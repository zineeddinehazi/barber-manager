package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole 403s unless the authenticated user's role is in the allowed set.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		if !allowed[c.GetString("role")] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

// RequireOwnShop 403s unless the caller's JWT shop_id matches the :shopId
// path parameter. Only meaningful after Auth and RequireRole(models.RoleOwner)
// have already run.
func RequireOwnShop() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("shop_id") != c.Param("shopId") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not your shop"})
			return
		}
		c.Next()
	}
}
