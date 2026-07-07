package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
)

// CreateReservationHandler re-derives EndsAt from the service's duration
// server-side; it never trusts a client-supplied end time.
func CreateReservationHandler(reservations repository.ReservationRepository, services repository.ServiceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in models.ReservationCreateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		service, err := services.GetService(c.Request.Context(), in.ServiceID)
		if err != nil {
			respondError(c, err)
			return
		}

		in.CustomerID = c.GetString("user_id")
		in.ShopID = service.ShopID
		in.EndsAt = in.StartsAt.Add(time.Duration(service.DurationMinutes) * time.Minute)

		res, err := reservations.CreateReservation(c.Request.Context(), in)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, res)
	}
}

func ListOwnReservationsHandler(reservations repository.ReservationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		customerID := c.GetString("user_id")
		list, err := reservations.ListForCustomer(c.Request.Context(), customerID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func CancelReservationHandler(reservations repository.ReservationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		customerID := c.GetString("user_id")
		if err := reservations.Cancel(c.Request.Context(), id, customerID); err != nil {
			respondError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func ListBarberReservationsHandler(reservations repository.ReservationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		barberID := c.GetString("user_id")

		from := time.Now().Truncate(24 * time.Hour)
		to := from.AddDate(0, 0, 30)
		if fromStr := c.Query("from"); fromStr != "" {
			if parsed, err := time.Parse("2006-01-02", fromStr); err == nil {
				from = parsed
			}
		}
		if toStr := c.Query("to"); toStr != "" {
			if parsed, err := time.Parse("2006-01-02", toStr); err == nil {
				to = parsed
			}
		}

		list, err := reservations.ListForBarber(c.Request.Context(), barberID, from, to)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func CompleteReservationHandler(reservations repository.ReservationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := reservations.UpdateStatus(c.Request.Context(), id, models.ReservationCompleted); err != nil {
			respondError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func NoShowReservationHandler(reservations repository.ReservationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := reservations.UpdateStatus(c.Request.Context(), id, models.ReservationNoShow); err != nil {
			respondError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func ListShopReservationsHandler(reservations repository.ReservationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		filter := models.ReservationFilter{
			BarberID: c.Query("barberId"),
			Status:   c.Query("status"),
		}
		list, err := reservations.ListForShop(c.Request.Context(), shopID, filter)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
