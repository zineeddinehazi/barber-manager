package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"barbermanager/internal/models"
)

type ScheduleRepository interface {
	GetApprovedSchedule(ctx context.Context, barberID string) ([]models.WorkSchedule, error)
	GetExceptions(ctx context.Context, barberID string, from, to time.Time) ([]models.ScheduleException, error)
	GetException(ctx context.Context, barberID string, date time.Time) (*models.ScheduleException, error)
	// ProposeSchedule never mutates work_schedules directly - it only records
	// an ApprovalRequest holding the proposed days as its payload.
	ProposeSchedule(ctx context.Context, shopID, barberID string, in models.ScheduleUpdateInput) (*models.ApprovalRequest, error)
	AddException(ctx context.Context, barberID string, in models.ScheduleExceptionInput) (*models.ScheduleException, error)
	// ApplySchedule writes an approved payload onto the live work_schedules
	// rows; called only from ApprovalRepository.Approve inside its transaction.
	ApplySchedule(ctx context.Context, tx pgx.Tx, barberID string, payload json.RawMessage) error
}

type PgScheduleRepository struct {
	Pool *pgxpool.Pool
}

func NewScheduleRepository(pool *pgxpool.Pool) *PgScheduleRepository {
	return &PgScheduleRepository{Pool: pool}
}

func (r *PgScheduleRepository) GetApprovedSchedule(ctx context.Context, barberID string) ([]models.WorkSchedule, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT id, barber_id, weekday, is_working, COALESCE(start_time::text, ''), COALESCE(end_time::text, ''), status
		 FROM work_schedules WHERE barber_id = $1 AND status = 'approved' ORDER BY weekday`,
		barberID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedule := make([]models.WorkSchedule, 0)
	for rows.Next() {
		var s models.WorkSchedule
		if err := rows.Scan(&s.ID, &s.BarberID, &s.Weekday, &s.IsWorking, &s.StartTime, &s.EndTime, &s.Status); err != nil {
			return nil, err
		}
		schedule = append(schedule, s)
	}
	return schedule, rows.Err()
}

func (r *PgScheduleRepository) GetExceptions(ctx context.Context, barberID string, from, to time.Time) ([]models.ScheduleException, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT id, barber_id, date::text, is_working, COALESCE(start_time::text, ''), COALESCE(end_time::text, ''), reason
		 FROM schedule_exceptions WHERE barber_id = $1 AND date BETWEEN $2 AND $3 ORDER BY date`,
		barberID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exceptions := make([]models.ScheduleException, 0)
	for rows.Next() {
		var e models.ScheduleException
		if err := rows.Scan(&e.ID, &e.BarberID, &e.Date, &e.IsWorking, &e.StartTime, &e.EndTime, &e.Reason); err != nil {
			return nil, err
		}
		exceptions = append(exceptions, e)
	}
	return exceptions, rows.Err()
}

func (r *PgScheduleRepository) GetException(ctx context.Context, barberID string, date time.Time) (*models.ScheduleException, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var e models.ScheduleException
	err := r.Pool.QueryRow(ctx,
		`SELECT id, barber_id, date::text, is_working, COALESCE(start_time::text, ''), COALESCE(end_time::text, ''), reason
		 FROM schedule_exceptions WHERE barber_id = $1 AND date = $2`,
		barberID, date,
	).Scan(&e.ID, &e.BarberID, &e.Date, &e.IsWorking, &e.StartTime, &e.EndTime, &e.Reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *PgScheduleRepository) ProposeSchedule(ctx context.Context, shopID, barberID string, in models.ScheduleUpdateInput) (*models.ApprovalRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	payload, err := json.Marshal(in.Days)
	if err != nil {
		return nil, err
	}

	var req models.ApprovalRequest
	err = r.Pool.QueryRow(ctx,
		`INSERT INTO approval_requests (shop_id, barber_id, target_type, target_id, payload)
		 VALUES ($1, $2, 'work_schedule', $2, $3)
		 RETURNING id, shop_id, barber_id, target_type, target_id, payload, status, reviewed_by, reviewed_at, created_at`,
		shopID, barberID, payload,
	).Scan(&req.ID, &req.ShopID, &req.BarberID, &req.TargetType, &req.TargetID, &req.Payload, &req.Status, &req.ReviewedBy, &req.ReviewedAt, &req.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *PgScheduleRepository) AddException(ctx context.Context, barberID string, in models.ScheduleExceptionInput) (*models.ScheduleException, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var startTime, endTime any
	if in.IsWorking {
		startTime, endTime = in.StartTime, in.EndTime
	}

	var e models.ScheduleException
	err := r.Pool.QueryRow(ctx,
		`INSERT INTO schedule_exceptions (barber_id, date, is_working, start_time, end_time, reason)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (barber_id, date) DO UPDATE SET
			is_working = EXCLUDED.is_working,
			start_time = EXCLUDED.start_time,
			end_time = EXCLUDED.end_time,
			reason = EXCLUDED.reason
		 RETURNING id, barber_id, date::text, is_working, COALESCE(start_time::text, ''), COALESCE(end_time::text, ''), reason`,
		barberID, in.Date, in.IsWorking, startTime, endTime, in.Reason,
	).Scan(&e.ID, &e.BarberID, &e.Date, &e.IsWorking, &e.StartTime, &e.EndTime, &e.Reason)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *PgScheduleRepository) ApplySchedule(ctx context.Context, tx pgx.Tx, barberID string, payload json.RawMessage) error {
	var days []models.ScheduleDayInput
	if err := json.Unmarshal(payload, &days); err != nil {
		return err
	}

	for _, d := range days {
		var startTime, endTime any
		if d.IsWorking {
			startTime, endTime = d.StartTime, d.EndTime
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO work_schedules (barber_id, weekday, is_working, start_time, end_time, status)
			 VALUES ($1, $2, $3, $4, $5, 'approved')
			 ON CONFLICT (barber_id, weekday, status) DO UPDATE SET
				is_working = EXCLUDED.is_working,
				start_time = EXCLUDED.start_time,
				end_time = EXCLUDED.end_time`,
			barberID, d.Weekday, d.IsWorking, startTime, endTime,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
