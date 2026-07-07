package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
)

func ListShopsHandler(shops repository.ShopRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		city := c.Query("city")
		list, err := shops.ListShops(c.Request.Context(), city)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func GetShopHandler(shops repository.ShopRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("shopId")
		shop, err := shops.GetShop(c.Request.Context(), id)
		if err != nil {
			respondError(c, err)
			return
		}
		hours, err := shops.GetShopHours(c.Request.Context(), id)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"shop": shop, "hours": hours})
	}
}

func UpdateShopHandler(shops repository.ShopRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("shopId")
		var in models.ShopUpdateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		shop, err := shops.UpdateShop(c.Request.Context(), id, in)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, shop)
	}
}

type updateShopHoursInput struct {
	Hours []models.ShopHours `json:"hours" binding:"required,dive"`
}

func UpdateShopHoursHandler(shops repository.ShopRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("shopId")
		var in updateShopHoursInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		for i := range in.Hours {
			in.Hours[i].ShopID = id
		}
		if err := shops.SetShopHours(c.Request.Context(), id, in.Hours); err != nil {
			respondError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
