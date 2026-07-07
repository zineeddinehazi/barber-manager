package models

// ScheduleException is a one-off override (day off, holiday, or custom hours)
// for a barber on a specific date. Takes precedence over WorkSchedule.
type ScheduleException struct {
	ID        string `json:"id"`
	BarberID  string `json:"barber_id"`
	Date      string `json:"date"` // "YYYY-MM-DD"
	IsWorking bool   `json:"is_working"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	Reason    string `json:"reason"`
}
