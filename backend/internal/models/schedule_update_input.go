package models

type ScheduleDayInput struct {
	Weekday   int    `json:"weekday" binding:"min=0,max=6"`
	IsWorking bool   `json:"is_working"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// ScheduleUpdateInput is a barber's proposed weekly schedule; it never applies
// directly, it always creates an ApprovalRequest (see repository.ScheduleRepository).
type ScheduleUpdateInput struct {
	Days []ScheduleDayInput `json:"days" binding:"required,dive"`
}

type ScheduleExceptionInput struct {
	Date      string `json:"date" binding:"required"`
	IsWorking bool   `json:"is_working"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Reason    string `json:"reason"`
}
