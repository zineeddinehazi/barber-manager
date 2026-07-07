package models

// RatingCreateInput's ReservationID/CustomerID are set by the handler from
// the route param and JWT claims, never bound directly from client JSON.
type RatingCreateInput struct {
	ReservationID string `json:"-"`
	CustomerID    string `json:"-"`
	Score         int    `json:"score" binding:"required,min=1,max=5"`
	Comment       string `json:"comment"`
}
