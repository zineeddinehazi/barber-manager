package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"barbermanager/internal/models"
)

type ApprovalRepository interface {
	ListPending(ctx context.Context, shopID string) ([]models.ApprovalRequest, error)
	ListByBarber(ctx context.Context, barberID string) ([]models.ApprovalRequest, error)
	// Approve applies the request's payload onto the live schedule/service row
	// and marks the request approved, all inside a single transaction. shopID
	// scopes the lookup so an owner can only approve/reject their own shop's
	// requests - a mismatch is reported as ErrNotFound (existence-hiding).
	Approve(ctx context.Context, shopID, requestID, reviewerID string) error
	Reject(ctx context.Context, shopID, requestID, reviewerID string) error
}

// PgApprovalRepository depends directly on the schedule/service repositories'
// concrete Apply* methods so approval and application stay in one transaction.
type PgApprovalRepository struct {
	Pool     *pgxpool.Pool
	Schedule *PgScheduleRepository
	Service  *PgServiceRepository
}

func NewApprovalRepository(pool *pgxpool.Pool, schedule *PgScheduleRepository, service *PgServiceRepository) *PgApprovalRepository {
	return &PgApprovalRepository{Pool: pool, Schedule: schedule, Service: service}
}

func (r *PgApprovalRepository) ListPending(ctx context.Context, shopID string) ([]models.ApprovalRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT id, shop_id, barber_id, target_type, target_id, payload, status, reviewed_by, reviewed_at, created_at
		 FROM approval_requests WHERE shop_id = $1 AND status = 'pending' ORDER BY created_at`,
		shopID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanApprovalRequests(rows)
}

func (r *PgApprovalRepository) ListByBarber(ctx context.Context, barberID string) ([]models.ApprovalRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT id, shop_id, barber_id, target_type, target_id, payload, status, reviewed_by, reviewed_at, created_at
		 FROM approval_requests WHERE barber_id = $1 ORDER BY created_at DESC`,
		barberID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanApprovalRequests(rows)
}

func scanApprovalRequests(rows pgx.Rows) ([]models.ApprovalRequest, error) {
	requests := make([]models.ApprovalRequest, 0)
	for rows.Next() {
		var req models.ApprovalRequest
		if err := rows.Scan(&req.ID, &req.ShopID, &req.BarberID, &req.TargetType, &req.TargetID, &req.Payload, &req.Status, &req.ReviewedBy, &req.ReviewedAt, &req.CreatedAt); err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, rows.Err()
}

func (r *PgApprovalRepository) Approve(ctx context.Context, shopID, requestID, reviewerID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var req models.ApprovalRequest
	err = tx.QueryRow(ctx,
		`SELECT id, barber_id, target_type, target_id, payload, status
		 FROM approval_requests WHERE id = $1 AND shop_id = $2 FOR UPDATE`,
		requestID, shopID,
	).Scan(&req.ID, &req.BarberID, &req.TargetType, &req.TargetID, &req.Payload, &req.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if req.Status != models.ApprovalStatusPending {
		return ErrNotPending
	}

	switch req.TargetType {
	case models.ApprovalTargetSchedule:
		if err := r.Schedule.ApplySchedule(ctx, tx, req.BarberID, req.Payload); err != nil {
			return err
		}
	case models.ApprovalTargetService:
		if err := r.Service.ApplyUpdate(ctx, tx, req.TargetID, req.Payload); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown approval target type %q", req.TargetType)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE approval_requests SET status = 'approved', reviewed_by = $1, reviewed_at = now() WHERE id = $2`,
		reviewerID, requestID,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PgApprovalRepository) Reject(ctx context.Context, shopID, requestID, reviewerID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tag, err := r.Pool.Exec(ctx,
		`UPDATE approval_requests SET status = 'rejected', reviewed_by = $1, reviewed_at = now()
		 WHERE id = $2 AND shop_id = $3 AND status = 'pending'`,
		reviewerID, requestID, shopID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
