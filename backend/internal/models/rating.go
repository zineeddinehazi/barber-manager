package models

import "time"

type Rating struct {
	ID            string    `json:"id"`
	ReservationID string    `json:"reservation_id"`
	BarberID      string    `json:"barber_id"`
	CustomerID    string    `json:"customer_id"`
	Score         int       `json:"score"`
	Comment       string    `json:"comment"`
	CreatedAt     time.Time `json:"created_at"`
}
