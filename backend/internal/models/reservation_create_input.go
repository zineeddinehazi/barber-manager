package models

import "time"

// ReservationCreateInput's ShopID/CustomerID/EndsAt are set by the handler
// from context/service lookup, never bound directly from client JSON.
type ReservationCreateInput struct {
	ShopID     string    `json:"-"`
	BarberID   string    `json:"barber_id" binding:"required"`
	CustomerID string    `json:"-"`
	ServiceID  string    `json:"service_id" binding:"required"`
	StartsAt   time.Time `json:"starts_at" binding:"required"`
	EndsAt     time.Time `json:"-"`
	Notes      string    `json:"notes"`
}
