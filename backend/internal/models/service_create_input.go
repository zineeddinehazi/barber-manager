package models

type ServiceCreateInput struct {
	Name            string  `json:"name" binding:"required"`
	Description     string  `json:"description"`
	PriceDZD        float64 `json:"price_dzd" binding:"required,gt=0"`
	DurationMinutes int     `json:"duration_minutes" binding:"required,gt=0"`
	BarberID        *string `json:"barber_id,omitempty"`
}
