package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"barbermanager/internal/models"
)

type BarberRepository interface {
	CreateBarberProfile(ctx context.Context, userID, bio string) error
	ListActiveBarbers(ctx context.Context, shopID string) ([]models.BarberWithProfile, error)
	GetBarberProfile(ctx context.Context, barberID string) (*models.BarberWithProfile, error)
	UpdateBio(ctx context.Context, barberID, bio string) error
	SetActive(ctx context.Context, barberID, shopID string, active bool) error
	// RecalculateAvgRating must run inside the same transaction as a rating
	// INSERT (see RatingRepository.CreateRating) so the aggregate never drifts.
	RecalculateAvgRating(ctx context.Context, tx pgx.Tx, barberID string) error
}

type PgBarberRepository struct {
	Pool *pgxpool.Pool
}

func NewBarberRepository(pool *pgxpool.Pool) *PgBarberRepository {
	return &PgBarberRepository{Pool: pool}
}

func (r *PgBarberRepository) CreateBarberProfile(ctx context.Context, userID, bio string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO barber_profiles (user_id, bio) VALUES ($1, $2)`,
		userID, bio,
	)
	return err
}

func (r *PgBarberRepository) ListActiveBarbers(ctx context.Context, shopID string) ([]models.BarberWithProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT u.id, u.shop_id, u.full_name, u.phone, p.bio, p.is_active, p.avg_rating, p.rating_count
		 FROM users u
		 JOIN barber_profiles p ON p.user_id = u.id
		 WHERE u.shop_id = $1 AND u.role = 'barber' AND p.is_active = true
		 ORDER BY u.full_name`,
		shopID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	barbers := make([]models.BarberWithProfile, 0)
	for rows.Next() {
		var b models.BarberWithProfile
		if err := rows.Scan(&b.ID, &b.ShopID, &b.FullName, &b.Phone, &b.Bio, &b.IsActive, &b.AvgRating, &b.RatingCount); err != nil {
			return nil, err
		}
		barbers = append(barbers, b)
	}
	return barbers, rows.Err()
}

func (r *PgBarberRepository) GetBarberProfile(ctx context.Context, barberID string) (*models.BarberWithProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var b models.BarberWithProfile
	err := r.Pool.QueryRow(ctx,
		`SELECT u.id, u.shop_id, u.full_name, u.phone, p.bio, p.is_active, p.avg_rating, p.rating_count
		 FROM users u
		 JOIN barber_profiles p ON p.user_id = u.id
		 WHERE u.id = $1 AND u.role = 'barber'`,
		barberID,
	).Scan(&b.ID, &b.ShopID, &b.FullName, &b.Phone, &b.Bio, &b.IsActive, &b.AvgRating, &b.RatingCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *PgBarberRepository) UpdateBio(ctx context.Context, barberID, bio string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tag, err := r.Pool.Exec(ctx, `UPDATE barber_profiles SET bio = $1 WHERE user_id = $2`, bio, barberID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PgBarberRepository) SetActive(ctx context.Context, barberID, shopID string, active bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tag, err := r.Pool.Exec(ctx,
		`UPDATE barber_profiles p SET is_active = $1
		 FROM users u
		 WHERE p.user_id = u.id AND u.id = $2 AND u.shop_id = $3`,
		active, barberID, shopID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PgBarberRepository) RecalculateAvgRating(ctx context.Context, tx pgx.Tx, barberID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE barber_profiles SET
			avg_rating = COALESCE((SELECT AVG(score) FROM ratings WHERE barber_id = $1), 0),
			rating_count = (SELECT COUNT(*) FROM ratings WHERE barber_id = $1)
		 WHERE user_id = $1`,
		barberID,
	)
	return err
}
