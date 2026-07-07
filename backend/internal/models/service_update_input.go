package models

// ServiceUpdateInput is a barber's proposed price/duration change; like
// ScheduleUpdateInput it never applies directly, it creates an ApprovalRequest.
type ServiceUpdateInput struct {
	Name            *string  `json:"name"`
	Description     *string  `json:"description"`
	PriceDZD        *float64 `json:"price_dzd"`
	DurationMinutes *int     `json:"duration_minutes"`
}
