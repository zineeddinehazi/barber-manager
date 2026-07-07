package models

import (
	"encoding/json"
	"time"
)

const (
	ApprovalTargetSchedule = "work_schedule"
	ApprovalTargetService  = "service"

	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"
)

type ApprovalRequest struct {
	ID         string          `json:"id"`
	ShopID     string          `json:"shop_id"`
	BarberID   string          `json:"barber_id"`
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	Payload    json.RawMessage `json:"payload"`
	Status     string          `json:"status"`
	ReviewedBy *string         `json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time      `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
