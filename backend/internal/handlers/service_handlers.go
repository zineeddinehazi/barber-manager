package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
)

func CreateServiceHandler(services repository.ServiceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		var in models.ServiceCreateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		service, err := services.CreateService(c.Request.Context(), shopID, in)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, service)
	}
}

func ListShopServicesHandler(services repository.ServiceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		list, err := services.ListServices(c.Request.Context(), shopID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func ListOwnServicesHandler(services repository.ServiceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		barberID := c.GetString("user_id")
		list, err := services.ListServicesByBarber(c.Request.Context(), barberID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

// ProposeServiceUpdateHandler never mutates the live service row - it creates
// an ApprovalRequest the shop owner must approve.
func ProposeServiceUpdateHandler(services repository.ServiceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceID := c.Param("id")
		var in models.ServiceUpdateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		barberID := c.GetString("user_id")
		shopID := c.GetString("shop_id")

		svc, err := services.GetService(c.Request.Context(), serviceID)
		if err != nil {
			respondError(c, err)
			return
		}
		if svc.BarberID == nil || *svc.BarberID != barberID {
			c.JSON(http.StatusForbidden, gin.H{"error": "not your service"})
			return
		}

		req, err := services.ProposeUpdate(c.Request.Context(), shopID, barberID, serviceID, in)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, req)
	}
}
