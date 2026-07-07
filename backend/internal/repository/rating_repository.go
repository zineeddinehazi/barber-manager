package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"barbermanager/internal/models"
)

type RatingRepository interface {
	CreateRating(ctx context.Context, in models.RatingCreateInput) (*models.Rating, error)
	ListForBarber(ctx context.Context, barberID string, page, pageSize int) ([]models.Rating, int, error)
}

// PgRatingRepository depends on PgBarberRepository so a rating insert and the
// barber's avg_rating recalculation happen atomically in one transaction.
type PgRatingRepository struct {
	Pool   *pgxpool.Pool
	Barber *PgBarberRepository
}

func NewRatingRepository(pool *pgxpool.Pool, barber *PgBarberRepository) *PgRatingRepository {
	return &PgRatingRepository{Pool: pool, Barber: barber}
}

func (r *PgRatingRepository) CreateRating(ctx context.Context, in models.RatingCreateInput) (*models.Rating, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var resv models.Reservation
	err = tx.QueryRow(ctx,
		`SELECT id, barber_id, customer_id, status FROM reservations WHERE id = $1 FOR UPDATE`,
		in.ReservationID,
	).Scan(&resv.ID, &resv.BarberID, &resv.CustomerID, &resv.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if resv.CustomerID != in.CustomerID {
		// don't leak existence of another customer's reservation
		return nil, ErrNotFound
	}
	if resv.Status != models.ReservationCompleted {
		return nil, ErrNotCompleted
	}

	var rating models.Rating
	err = tx.QueryRow(ctx,
		`INSERT INTO ratings (reservation_id, barber_id, customer_id, score, comment)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, reservation_id, barber_id, customer_id, score, comment, created_at`,
		in.ReservationID, resv.BarberID, in.CustomerID, in.Score, in.Comment,
	).Scan(&rating.ID, &rating.ReservationID, &rating.BarberID, &rating.CustomerID, &rating.Score, &rating.Comment, &rating.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyRated
		}
		return nil, err
	}

	if err := r.Barber.RecalculateAvgRating(ctx, tx, resv.BarberID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &rating, nil
}

func (r *PgRatingRepository) ListForBarber(ctx context.Context, barberID string, page, pageSize int) ([]models.Rating, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var total int
	if err := r.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM ratings WHERE barber_id = $1`, barberID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.Pool.Query(ctx,
		`SELECT id, reservation_id, barber_id, customer_id, score, comment, created_at
		 FROM ratings WHERE barber_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		barberID, pageSize, (page-1)*pageSize,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	ratings := make([]models.Rating, 0)
	for rows.Next() {
		var rt models.Rating
		if err := rows.Scan(&rt.ID, &rt.ReservationID, &rt.BarberID, &rt.CustomerID, &rt.Score, &rt.Comment, &rt.CreatedAt); err != nil {
			return nil, 0, err
		}
		ratings = append(ratings, rt)
	}
	return ratings, total, rows.Err()
}
