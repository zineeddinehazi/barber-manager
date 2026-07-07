package models

import "time"

const (
	ReservationPending   = "pending"
	ReservationConfirmed = "confirmed"
	ReservationCancelled = "cancelled"
	ReservationCompleted = "completed"
	ReservationNoShow    = "no_show"
)

type Reservation struct {
	ID         string    `json:"id"`
	ShopID     string    `json:"shop_id"`
	BarberID   string    `json:"barber_id"`
	CustomerID string    `json:"customer_id"`
	ServiceID  string    `json:"service_id"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	Status     string    `json:"status"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
}

// ReservationFilter narrows a shop-wide reservation listing (owner view).
type ReservationFilter struct {
	BarberID string
	Status   string
	From     *time.Time
	To       *time.Time
}
