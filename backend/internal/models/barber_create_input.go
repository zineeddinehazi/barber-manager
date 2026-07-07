package models

type BarberCreateInput struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone" binding:"required"`
	Bio      string `json:"bio"`
}

// BarberCreateResponse includes the generated temporary password so the owner
// can hand it to the barber; the barber changes it via PATCH /auth/password.
type BarberCreateResponse struct {
	Barber       BarberWithProfile `json:"barber"`
	TempPassword string            `json:"temp_password"`
}
