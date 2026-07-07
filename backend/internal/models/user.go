package models

import "time"

type User struct {
	ID           string    `json:"id"`
	ShopID       *string   `json:"shop_id,omitempty"`
	Role         string    `json:"role"`
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

const (
	RoleCustomer = "customer"
	RoleBarber   = "barber"
	RoleOwner    = "owner"
)
