package availability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"barbermanager/internal/models"
)

var testDate = time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC) // a Wednesday

func TestAvailableSlots_ShopClosed(t *testing.T) {
	in := Input{
		Date:      testDate,
		ShopHours: &models.ShopHours{IsClosed: true},
		WorkSchedule: &models.WorkSchedule{
			IsWorking: true, StartTime: "09:00", EndTime: "17:00",
		},
		ServiceMinutes: 30,
	}
	slots, err := AvailableSlots(in)
	require.NoError(t, err)
	assert.Empty(t, slots)
}

func TestAvailableSlots_BarberNotWorking(t *testing.T) {
	in := Input{
		Date:           testDate,
		ShopHours:      &models.ShopHours{OpenTime: "09:00", CloseTime: "18:00"},
		WorkSchedule:   &models.WorkSchedule{IsWorking: false},
		ServiceMinutes: 30,
	}
	slots, err := AvailableSlots(in)
	require.NoError(t, err)
	assert.Empty(t, slots)
}

func TestAvailableSlots_ExceptionDayOffOverridesSchedule(t *testing.T) {
	in := Input{
		Date:      testDate,
		ShopHours: &models.ShopHours{OpenTime: "09:00", CloseTime: "18:00"},
		WorkSchedule: &models.WorkSchedule{
			IsWorking: true, StartTime: "09:00", EndTime: "17:00",
		},
		Exception:      &models.ScheduleException{IsWorking: false, Reason: "sick leave"},
		ServiceMinutes: 30,
	}
	slots, err := AvailableSlots(in)
	require.NoError(t, err)
	assert.Empty(t, slots)
}

func TestAvailableSlots_ExceptionCustomHoursOverridesSchedule(t *testing.T) {
	in := Input{
		Date:      testDate,
		ShopHours: &models.ShopHours{OpenTime: "09:00", CloseTime: "20:00"},
		WorkSchedule: &models.WorkSchedule{
			IsWorking: true, StartTime: "09:00", EndTime: "17:00",
		},
		Exception: &models.ScheduleException{
			IsWorking: true, StartTime: "18:00", EndTime: "19:00",
		},
		ServiceMinutes:         30,
		SlotGranularityMinutes: 30,
	}
	slots, err := AvailableSlots(in)
	require.NoError(t, err)
	require.Len(t, slots, 2)
	assert.Equal(t, "18:00", slots[0].Start.Format("15:04"))
	assert.Equal(t, "18:30", slots[1].Start.Format("15:04"))
}

func TestAvailableSlots_IntersectsShopAndBarberHours(t *testing.T) {
	in := Input{
		Date:      testDate,
		ShopHours: &models.ShopHours{OpenTime: "10:00", CloseTime: "16:00"},
		WorkSchedule: &models.WorkSchedule{
			IsWorking: true, StartTime: "09:00", EndTime: "17:00",
		},
		ServiceMinutes:         60,
		SlotGranularityMinutes: 60,
	}
	slots, err := AvailableSlots(in)
	require.NoError(t, err)
	require.Len(t, slots, 6) // 10-11,11-12,12-13,13-14,14-15,15-16
	assert.Equal(t, "10:00", slots[0].Start.Format("15:04"))
	assert.Equal(t, "15:00", slots[len(slots)-1].Start.Format("15:04"))
}

func TestAvailableSlots_ExcludesOverlappingReservations(t *testing.T) {
	loc := time.UTC
	reservationStart := time.Date(2026, 7, 8, 10, 0, 0, 0, loc)
	reservationEnd := time.Date(2026, 7, 8, 10, 30, 0, 0, loc)

	in := Input{
		Date:      testDate,
		Location:  loc,
		ShopHours: &models.ShopHours{OpenTime: "09:00", CloseTime: "12:00"},
		WorkSchedule: &models.WorkSchedule{
			IsWorking: true, StartTime: "09:00", EndTime: "12:00",
		},
		Reservations: []models.Reservation{
			{StartsAt: reservationStart, EndsAt: reservationEnd},
		},
		ServiceMinutes:         30,
		SlotGranularityMinutes: 30,
	}
	slots, err := AvailableSlots(in)
	require.NoError(t, err)

	for _, s := range slots {
		assert.False(t, s.Start.Before(reservationEnd) && reservationStart.Before(s.End),
			"slot %v-%v should not overlap reservation %v-%v", s.Start, s.End, reservationStart, reservationEnd)
	}
	// 09:00-12:00 in 30-min steps = 6 possible slots, minus the one colliding with 10:00-10:30
	assert.Len(t, slots, 5)
}

func TestAvailableSlots_IgnoresCancelledAndNoShowReservations(t *testing.T) {
	loc := time.UTC
	reservationStart := time.Date(2026, 7, 8, 10, 0, 0, 0, loc)
	reservationEnd := time.Date(2026, 7, 8, 10, 30, 0, 0, loc)

	in := Input{
		Date:      testDate,
		Location:  loc,
		ShopHours: &models.ShopHours{OpenTime: "09:00", CloseTime: "12:00"},
		WorkSchedule: &models.WorkSchedule{
			IsWorking: true, StartTime: "09:00", EndTime: "12:00",
		},
		Reservations: []models.Reservation{
			{StartsAt: reservationStart, EndsAt: reservationEnd, Status: models.ReservationCancelled},
			{StartsAt: reservationStart, EndsAt: reservationEnd, Status: models.ReservationNoShow},
		},
		ServiceMinutes:         30,
		SlotGranularityMinutes: 30,
	}
	slots, err := AvailableSlots(in)
	require.NoError(t, err)
	// 09:00-12:00 in 30-min steps = 6 slots; cancelled/no_show reservations
	// must not remove any of them.
	assert.Len(t, slots, 6)
}

func TestContains_WithinWorkingWindow(t *testing.T) {
	loc := time.UTC
	in := Input{
		Date:         testDate,
		Location:     loc,
		ShopHours:    &models.ShopHours{OpenTime: "09:00", CloseTime: "18:00"},
		WorkSchedule: &models.WorkSchedule{IsWorking: true, StartTime: "09:00", EndTime: "17:00"},
	}

	within, err := Contains(in, time.Date(2026, 7, 8, 9, 30, 0, 0, loc), time.Date(2026, 7, 8, 10, 0, 0, 0, loc))
	require.NoError(t, err)
	assert.True(t, within)

	within, err = Contains(in, time.Date(2026, 7, 8, 17, 30, 0, 0, loc), time.Date(2026, 7, 8, 18, 0, 0, 0, loc))
	require.NoError(t, err)
	assert.False(t, within, "outside barber's working hours even though the shop is still open")

	within, err = Contains(in, time.Date(2026, 7, 8, 3, 0, 0, 0, loc), time.Date(2026, 7, 8, 3, 30, 0, 0, loc))
	require.NoError(t, err)
	assert.False(t, within, "outside shop hours")
}

func TestAvailableSlots_NoWorkScheduleNoException(t *testing.T) {
	in := Input{
		Date:           testDate,
		ShopHours:      &models.ShopHours{OpenTime: "09:00", CloseTime: "18:00"},
		ServiceMinutes: 30,
	}
	slots, err := AvailableSlots(in)
	require.NoError(t, err)
	assert.Empty(t, slots)
}

func TestAvailableSlots_InvalidServiceDuration(t *testing.T) {
	in := Input{
		Date:           testDate,
		ShopHours:      &models.ShopHours{OpenTime: "09:00", CloseTime: "18:00"},
		ServiceMinutes: 0,
	}
	_, err := AvailableSlots(in)
	assert.Error(t, err)
}
