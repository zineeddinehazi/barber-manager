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

type ServiceRepository interface {
	CreateService(ctx context.Context, shopID string, in models.ServiceCreateInput) (*models.Service, error)
	ListServices(ctx context.Context, shopID string) ([]models.Service, error)
	ListServicesByBarber(ctx context.Context, barberID string) ([]models.Service, error)
	GetService(ctx context.Context, id string) (*models.Service, error)
	// ProposeUpdate never mutates the live service row - it only records an
	// ApprovalRequest holding the proposed diff.
	ProposeUpdate(ctx context.Context, shopID, barberID, serviceID string, in models.ServiceUpdateInput) (*models.ApprovalRequest, error)
	// ApplyUpdate writes an approved payload onto the live service row;
	// called only from ApprovalRepository.Approve inside its transaction.
	ApplyUpdate(ctx context.Context, tx pgx.Tx, serviceID string, payload json.RawMessage) error
}

type PgServiceRepository struct {
	Pool *pgxpool.Pool
}

func NewServiceRepository(pool *pgxpool.Pool) *PgServiceRepository {
	return &PgServiceRepository{Pool: pool}
}

func (r *PgServiceRepository) CreateService(ctx context.Context, shopID string, in models.ServiceCreateInput) (*models.Service, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var s models.Service
	err := r.Pool.QueryRow(ctx,
		`INSERT INTO services (shop_id, barber_id, name, description, price_dzd, duration_minutes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, shop_id, barber_id, name, description, price_dzd, duration_minutes, status, is_active, created_at`,
		shopID, in.BarberID, in.Name, in.Description, in.PriceDZD, in.DurationMinutes,
	).Scan(&s.ID, &s.ShopID, &s.BarberID, &s.Name, &s.Description, &s.PriceDZD, &s.DurationMinutes, &s.Status, &s.IsActive, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgServiceRepository) ListServices(ctx context.Context, shopID string) ([]models.Service, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT id, shop_id, barber_id, name, description, price_dzd, duration_minutes, status, is_active, created_at
		 FROM services WHERE shop_id = $1 AND status = 'approved' AND is_active = true ORDER BY name`,
		shopID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServices(rows)
}

func (r *PgServiceRepository) ListServicesByBarber(ctx context.Context, barberID string) ([]models.Service, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT id, shop_id, barber_id, name, description, price_dzd, duration_minutes, status, is_active, created_at
		 FROM services WHERE barber_id = $1 ORDER BY name`,
		barberID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServices(rows)
}

func scanServices(rows pgx.Rows) ([]models.Service, error) {
	services := make([]models.Service, 0)
	for rows.Next() {
		var s models.Service
		if err := rows.Scan(&s.ID, &s.ShopID, &s.BarberID, &s.Name, &s.Description, &s.PriceDZD, &s.DurationMinutes, &s.Status, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

func (r *PgServiceRepository) GetService(ctx context.Context, id string) (*models.Service, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var s models.Service
	err := r.Pool.QueryRow(ctx,
		`SELECT id, shop_id, barber_id, name, description, price_dzd, duration_minutes, status, is_active, created_at
		 FROM services WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.ShopID, &s.BarberID, &s.Name, &s.Description, &s.PriceDZD, &s.DurationMinutes, &s.Status, &s.IsActive, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgServiceRepository) ProposeUpdate(ctx context.Context, shopID, barberID, serviceID string, in models.ServiceUpdateInput) (*models.ApprovalRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	var req models.ApprovalRequest
	err = r.Pool.QueryRow(ctx,
		`INSERT INTO approval_requests (shop_id, barber_id, target_type, target_id, payload)
		 VALUES ($1, $2, 'service', $3, $4)
		 RETURNING id, shop_id, barber_id, target_type, target_id, payload, status, reviewed_by, reviewed_at, created_at`,
		shopID, barberID, serviceID, payload,
	).Scan(&req.ID, &req.ShopID, &req.BarberID, &req.TargetType, &req.TargetID, &req.Payload, &req.Status, &req.ReviewedBy, &req.ReviewedAt, &req.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *PgServiceRepository) ApplyUpdate(ctx context.Context, tx pgx.Tx, serviceID string, payload json.RawMessage) error {
	var in models.ServiceUpdateInput
	if err := json.Unmarshal(payload, &in); err != nil {
		return err
	}

	_, err := tx.Exec(ctx,
		`UPDATE services SET
			name = COALESCE($1, name),
			description = COALESCE($2, description),
			price_dzd = COALESCE($3, price_dzd),
			duration_minutes = COALESCE($4, duration_minutes)
		 WHERE id = $5`,
		in.Name, in.Description, in.PriceDZD, in.DurationMinutes, serviceID,
	)
	return err
}
