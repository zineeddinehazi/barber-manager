package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
)

// CreateReservationHandler re-derives EndsAt from the service's duration
// server-side; it never trusts a client-supplied end time. It also verifies
// the requested barber actually offers the requested service (same shop, and
// same barber if the service is barber-specific) and that the slot isn't in
// the past - neither of which the DB constraints alone catch.
func CreateReservationHandler(reservations repository.ReservationRepository, services repository.ServiceRepository, barbers repository.BarberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in models.ReservationCreateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if !in.StartsAt.After(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "starts_at must be in the future"})
			return
		}

		service, err := services.GetService(c.Request.Context(), in.ServiceID)
		if err != nil {
			respondError(c, err)
			return
		}
		if service.BarberID != nil && *service.BarberID != in.BarberID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "service does not belong to the requested barber"})
			return
		}

		barber, err := barbers.GetBarberProfile(c.Request.Context(), in.BarberID)
		if err != nil {
			respondError(c, err)
			return
		}
		if barber.ShopID != service.ShopID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "barber does not work at this service's shop"})
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

// ListBarberReservationsHandler parses from/to in the shop's own location
// (not UTC) so day boundaries match what GetAvailabilityHandler shows for the
// same dates.
func ListBarberReservationsHandler(reservations repository.ReservationRepository, loc *time.Location) gin.HandlerFunc {
	return func(c *gin.Context) {
		barberID := c.GetString("user_id")

		y, m, d := time.Now().In(loc).Date()
		from := time.Date(y, m, d, 0, 0, 0, 0, loc)
		to := from.AddDate(0, 0, 30)
		if fromStr := c.Query("from"); fromStr != "" {
			if parsed, err := time.ParseInLocation("2006-01-02", fromStr, loc); err == nil {
				from = parsed
			}
		}
		if toStr := c.Query("to"); toStr != "" {
			if parsed, err := time.ParseInLocation("2006-01-02", toStr, loc); err == nil {
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
