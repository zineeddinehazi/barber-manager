package availability

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"barbermanager/internal/models"
)

const defaultSlotGranularityMinutes = 15

type Slot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Input struct {
	Date                   time.Time
	Location               *time.Location
	ShopHours              *models.ShopHours
	WorkSchedule           *models.WorkSchedule
	Exception              *models.ScheduleException
	Reservations           []models.Reservation
	ServiceMinutes         int
	SlotGranularityMinutes int
}

// AvailableSlots computes bookable start/end times for one barber on one date,
// given the shop's hours, the barber's approved weekly schedule, any one-off
// exception for that date, and the barber's existing reservations that day.
// Pure function: no I/O, so it's fully unit-testable with fixed fixtures.
func AvailableSlots(in Input) ([]Slot, error) {
	if in.ServiceMinutes <= 0 {
		return nil, fmt.Errorf("service duration must be positive")
	}
	granularity := in.SlotGranularityMinutes
	if granularity <= 0 {
		granularity = defaultSlotGranularityMinutes
	}

	windowStart, windowEnd, ok, err := workingWindow(in)
	if err != nil || !ok {
		return nil, err
	}

	serviceDur := time.Duration(in.ServiceMinutes) * time.Minute
	step := time.Duration(granularity) * time.Minute

	slots := make([]Slot, 0)
	for start := windowStart; !start.Add(serviceDur).After(windowEnd); start = start.Add(step) {
		end := start.Add(serviceDur)
		if !overlapsAny(start, end, in.Reservations) {
			slots = append(slots, Slot{Start: start, End: end})
		}
	}
	return slots, nil
}

// Contains reports whether [start, end) falls entirely within the barber's
// working window for the day described by in (shop hours intersected with
// the approved schedule/exception for in.Date) — used to re-validate a
// booking request server-side against the same rules GetAvailabilityHandler
// uses to display slots.
func Contains(in Input, start, end time.Time) (bool, error) {
	windowStart, windowEnd, ok, err := workingWindow(in)
	if err != nil || !ok {
		return false, err
	}
	return !start.Before(windowStart) && !end.After(windowEnd), nil
}

// workingWindow computes the barber's bookable [start, end) window on
// in.Date: the shop's open hours intersected with the barber's approved
// weekly schedule (or that date's one-off exception, which takes priority).
// ok is false when the shop is closed, the barber isn't working, or the
// intersection is empty.
func workingWindow(in Input) (start, end time.Time, ok bool, err error) {
	loc := in.Location
	if loc == nil {
		loc = time.UTC
	}

	if in.ShopHours == nil || in.ShopHours.IsClosed {
		return time.Time{}, time.Time{}, false, nil
	}

	var workStart, workEnd string
	switch {
	case in.Exception != nil:
		if !in.Exception.IsWorking {
			return time.Time{}, time.Time{}, false, nil
		}
		workStart, workEnd = in.Exception.StartTime, in.Exception.EndTime
	case in.WorkSchedule != nil && in.WorkSchedule.IsWorking:
		workStart, workEnd = in.WorkSchedule.StartTime, in.WorkSchedule.EndTime
	default:
		return time.Time{}, time.Time{}, false, nil
	}

	shopOpen, err := timeOfDay(in.Date, loc, in.ShopHours.OpenTime)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("parsing shop open time: %w", err)
	}
	shopClose, err := timeOfDay(in.Date, loc, in.ShopHours.CloseTime)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("parsing shop close time: %w", err)
	}
	barberStart, err := timeOfDay(in.Date, loc, workStart)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("parsing barber start time: %w", err)
	}
	barberEnd, err := timeOfDay(in.Date, loc, workEnd)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("parsing barber end time: %w", err)
	}

	windowStart := shopOpen
	if barberStart.After(windowStart) {
		windowStart = barberStart
	}
	windowEnd := shopClose
	if barberEnd.Before(windowEnd) {
		windowEnd = barberEnd
	}
	if !windowStart.Before(windowEnd) {
		return time.Time{}, time.Time{}, false, nil
	}
	return windowStart, windowEnd, true, nil
}

func overlapsAny(start, end time.Time, reservations []models.Reservation) bool {
	for _, r := range reservations {
		if r.Status == models.ReservationCancelled || r.Status == models.ReservationNoShow {
			continue
		}
		if start.Before(r.EndsAt) && r.StartsAt.Before(end) {
			return true
		}
	}
	return false
}

// timeOfDay combines a "HH:MM" or "HH:MM:SS" string with the date part of
// `date`, in the given location.
func timeOfDay(date time.Time, loc *time.Location, s string) (time.Time, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid time %q", s)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
	}
	sec := 0
	if len(parts) >= 3 {
		sec, err = strconv.Atoi(parts[2])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
		}
	}
	y, m, d := date.In(loc).Date()
	return time.Date(y, m, d, hour, minute, sec, 0, loc), nil
}
