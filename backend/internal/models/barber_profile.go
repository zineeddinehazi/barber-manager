package models

import "time"

type BarberProfile struct {
	UserID      string    `json:"user_id"`
	Bio         string    `json:"bio"`
	IsActive    bool      `json:"is_active"`
	AvgRating   float64   `json:"avg_rating"`
	RatingCount int       `json:"rating_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// BarberWithProfile is the flattened shape returned by barber listing/detail
// endpoints: the user's public info joined with their barber profile.
type BarberWithProfile struct {
	ID          string  `json:"id"`
	ShopID      string  `json:"shop_id"`
	FullName    string  `json:"full_name"`
	Phone       string  `json:"phone"`
	Bio         string  `json:"bio"`
	IsActive    bool    `json:"is_active"`
	AvgRating   float64 `json:"avg_rating"`
	RatingCount int     `json:"rating_count"`
}
