package models

import "time"

type Service struct {
	ID              string    `json:"id"`
	ShopID          string    `json:"shop_id"`
	BarberID        *string   `json:"barber_id,omitempty"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	PriceDZD        float64   `json:"price_dzd"`
	DurationMinutes int       `json:"duration_minutes"`
	Status          string    `json:"status"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}
