package models

// ShopHours is the shop's opening window for one weekday (0=Sunday..6=Saturday).
// OpenTime/CloseTime are "HH:MM:SS" strings, empty when IsClosed is true.
type ShopHours struct {
	ID        string `json:"id"`
	ShopID    string `json:"shop_id"`
	Weekday   int    `json:"weekday"`
	IsClosed  bool   `json:"is_closed"`
	OpenTime  string `json:"open_time,omitempty"`
	CloseTime string `json:"close_time,omitempty"`
}
