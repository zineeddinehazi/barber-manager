package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"barbermanager/internal/models"
)

type ReservationRepository interface {
	CreateReservation(ctx context.Context, in models.ReservationCreateInput) (*models.Reservation, error)
	GetReservation(ctx context.Context, id string) (*models.Reservation, error)
	ListForCustomer(ctx context.Context, customerID string) ([]models.Reservation, error)
	ListForBarber(ctx context.Context, barberID string, from, to time.Time) ([]models.Reservation, error)
	ListForShop(ctx context.Context, shopID string, filter models.ReservationFilter) ([]models.Reservation, error)
	UpdateStatus(ctx context.Context, id, status string) error
	Cancel(ctx context.Context, id, customerID string) error
}

type PgReservationRepository struct {
	Pool *pgxpool.Pool
}

func NewReservationRepository(pool *pgxpool.Pool) *PgReservationRepository {
	return &PgReservationRepository{Pool: pool}
}

const reservationColumns = `id, shop_id, barber_id, customer_id, service_id, starts_at, ends_at, status, notes, created_at`

func (r *PgReservationRepository) CreateReservation(ctx context.Context, in models.ReservationCreateInput) (*models.Reservation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var res models.Reservation
	err := r.Pool.QueryRow(ctx,
		`INSERT INTO reservations (shop_id, barber_id, customer_id, service_id, starts_at, ends_at, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+reservationColumns,
		in.ShopID, in.BarberID, in.CustomerID, in.ServiceID, in.StartsAt, in.EndsAt, in.Notes,
	).Scan(&res.ID, &res.ShopID, &res.BarberID, &res.CustomerID, &res.ServiceID, &res.StartsAt, &res.EndsAt, &res.Status, &res.Notes, &res.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			return nil, ErrSlotUnavailable
		}
		return nil, err
	}
	return &res, nil
}

func (r *PgReservationRepository) GetReservation(ctx context.Context, id string) (*models.Reservation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var res models.Reservation
	err := r.Pool.QueryRow(ctx,
		`SELECT `+reservationColumns+` FROM reservations WHERE id = $1`,
		id,
	).Scan(&res.ID, &res.ShopID, &res.BarberID, &res.CustomerID, &res.ServiceID, &res.StartsAt, &res.EndsAt, &res.Status, &res.Notes, &res.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *PgReservationRepository) ListForCustomer(ctx context.Context, customerID string) ([]models.Reservation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT `+reservationColumns+` FROM reservations WHERE customer_id = $1 ORDER BY starts_at DESC`,
		customerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReservations(rows)
}

func (r *PgReservationRepository) ListForBarber(ctx context.Context, barberID string, from, to time.Time) ([]models.Reservation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT `+reservationColumns+` FROM reservations
		 WHERE barber_id = $1 AND starts_at >= $2 AND starts_at < $3
		 ORDER BY starts_at`,
		barberID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReservations(rows)
}

func (r *PgReservationRepository) ListForShop(ctx context.Context, shopID string, filter models.ReservationFilter) ([]models.Reservation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `SELECT ` + reservationColumns + ` FROM reservations WHERE shop_id = $1`
	args := []any{shopID}

	if filter.BarberID != "" {
		args = append(args, filter.BarberID)
		query += fmt.Sprintf(" AND barber_id = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		query += fmt.Sprintf(" AND starts_at >= $%d", len(args))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		query += fmt.Sprintf(" AND starts_at < $%d", len(args))
	}
	query += " ORDER BY starts_at"

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReservations(rows)
}

func scanReservations(rows pgx.Rows) ([]models.Reservation, error) {
	reservations := make([]models.Reservation, 0)
	for rows.Next() {
		var res models.Reservation
		if err := rows.Scan(&res.ID, &res.ShopID, &res.BarberID, &res.CustomerID, &res.ServiceID, &res.StartsAt, &res.EndsAt, &res.Status, &res.Notes, &res.CreatedAt); err != nil {
			return nil, err
		}
		reservations = append(reservations, res)
	}
	return reservations, rows.Err()
}

func (r *PgReservationRepository) UpdateStatus(ctx context.Context, id, status string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tag, err := r.Pool.Exec(ctx, `UPDATE reservations SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PgReservationRepository) Cancel(ctx context.Context, id, customerID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tag, err := r.Pool.Exec(ctx,
		`UPDATE reservations SET status = 'cancelled'
		 WHERE id = $1 AND customer_id = $2 AND status IN ('pending', 'confirmed')`,
		id, customerID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
