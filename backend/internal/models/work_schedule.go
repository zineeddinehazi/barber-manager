package models

// WorkSchedule is a barber's recurring weekly working hours for one weekday.
// StartTime/EndTime are "HH:MM:SS" strings, empty when IsWorking is false.
type WorkSchedule struct {
	ID        string `json:"id"`
	BarberID  string `json:"barber_id"`
	Weekday   int    `json:"weekday"`
	IsWorking bool   `json:"is_working"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	Status    string `json:"status"`
}
