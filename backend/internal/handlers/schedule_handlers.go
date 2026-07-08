package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/availability"
	"barbermanager/internal/models"
	"barbermanager/internal/repository"
)

func GetOwnScheduleHandler(schedules repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		barberID := c.GetString("user_id")
		schedule, err := schedules.GetApprovedSchedule(c.Request.Context(), barberID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, schedule)
	}
}

// ProposeScheduleHandler never mutates the live schedule - it creates an
// ApprovalRequest the shop owner must approve (see repository.ScheduleRepository).
func ProposeScheduleHandler(schedules repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in models.ScheduleUpdateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		barberID := c.GetString("user_id")
		shopID := c.GetString("shop_id")

		req, err := schedules.ProposeSchedule(c.Request.Context(), shopID, barberID, in)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, req)
	}
}

// AddExceptionHandler applies immediately - a barber's own one-off day
// off/custom hours doesn't need owner approval.
func AddExceptionHandler(schedules repository.ScheduleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in models.ScheduleExceptionInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		barberID := c.GetString("user_id")
		exc, err := schedules.AddException(c.Request.Context(), barberID, in)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, exc)
	}
}

// GetAvailabilityHandler composes shop hours, the barber's approved schedule,
// any exception for the date, and existing reservations into the pure
// availability.AvailableSlots computation.
func GetAvailabilityHandler(
	shops repository.ShopRepository,
	schedules repository.ScheduleRepository,
	services repository.ServiceRepository,
	barbers repository.BarberRepository,
	reservations repository.ReservationRepository,
	loc *time.Location,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		barberID := c.Param("barberId")
		serviceID := c.Query("serviceId")
		dateStr := c.Query("date")

		if serviceID == "" || dateStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "serviceId and date query params are required"})
			return
		}

		date, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date, expected YYYY-MM-DD"})
			return
		}

		barber, err := barbers.GetBarberProfile(c.Request.Context(), barberID)
		if err != nil {
			respondError(c, err)
			return
		}
		if barber.ShopID != shopID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "barber does not work at this shop"})
			return
		}

		service, err := services.GetService(c.Request.Context(), serviceID)
		if err != nil {
			respondError(c, err)
			return
		}
		if service.ShopID != shopID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "service does not belong to this shop"})
			return
		}
		if service.BarberID != nil && *service.BarberID != barberID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "service does not belong to the requested barber"})
			return
		}

		shopHours, workSchedule, exception, err := loadDaySchedule(c.Request.Context(), shops, schedules, shopID, barberID, date)
		if err != nil {
			respondError(c, err)
			return
		}

		dayStart := date
		dayEnd := date.AddDate(0, 0, 1)
		existing, err := reservations.ListForBarber(c.Request.Context(), barberID, dayStart, dayEnd)
		if err != nil {
			respondError(c, err)
			return
		}

		slots, err := availability.AvailableSlots(availability.Input{
			Date:           date,
			Location:       loc,
			ShopHours:      shopHours,
			WorkSchedule:   workSchedule,
			Exception:      exception,
			Reservations:   existing,
			ServiceMinutes: service.DurationMinutes,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"shop_id":   shopID,
			"barber_id": barberID,
			"date":      dateStr,
			"slots":     slots,
		})
	}
}

// loadDaySchedule fetches the shop's hours, the barber's approved weekly
// schedule, and any one-off exception, narrowed to the single weekday of
// date - shared by GetAvailabilityHandler and CreateReservationHandler so
// booking is validated against exactly the same rules availability displays.
func loadDaySchedule(
	ctx context.Context,
	shops repository.ShopRepository,
	schedules repository.ScheduleRepository,
	shopID, barberID string,
	date time.Time,
) (*models.ShopHours, *models.WorkSchedule, *models.ScheduleException, error) {
	hoursList, err := shops.GetShopHours(ctx, shopID)
	if err != nil {
		return nil, nil, nil, err
	}
	weekday := int(date.Weekday())
	var shopHours *models.ShopHours
	for i := range hoursList {
		if hoursList[i].Weekday == weekday {
			shopHours = &hoursList[i]
			break
		}
	}

	schedule, err := schedules.GetApprovedSchedule(ctx, barberID)
	if err != nil {
		return nil, nil, nil, err
	}
	var workSchedule *models.WorkSchedule
	for i := range schedule {
		if schedule[i].Weekday == weekday {
			workSchedule = &schedule[i]
			break
		}
	}

	exception, err := schedules.GetException(ctx, barberID, date)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, nil, nil, err
	}
	if errors.Is(err, repository.ErrNotFound) {
		exception = nil
	}

	return shopHours, workSchedule, exception, nil
}
