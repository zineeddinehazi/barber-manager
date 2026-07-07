package repository

import "errors"

var (
	ErrNotFound        = errors.New("resource not found")
	ErrSlotUnavailable = errors.New("requested time slot is no longer available")
	ErrAlreadyRated    = errors.New("reservation already rated")
	ErrNotCompleted    = errors.New("reservation is not completed yet")
	ErrDuplicateEmail  = errors.New("email already registered")
	ErrNotPending      = errors.New("approval request is not pending")
)
