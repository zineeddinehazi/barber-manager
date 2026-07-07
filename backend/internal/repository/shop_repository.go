package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"barbermanager/internal/models"
)

type ShopRepository interface {
	CreateShop(ctx context.Context, in models.ShopCreateInput, ownerID string) (*models.Shop, error)
	GetShop(ctx context.Context, id string) (*models.Shop, error)
	ListShops(ctx context.Context, city string) ([]models.Shop, error)
	UpdateShop(ctx context.Context, id string, in models.ShopUpdateInput) (*models.Shop, error)
	GetShopHours(ctx context.Context, shopID string) ([]models.ShopHours, error)
	SetShopHours(ctx context.Context, shopID string, hours []models.ShopHours) error
}

type PgShopRepository struct {
	Pool *pgxpool.Pool
}

func NewShopRepository(pool *pgxpool.Pool) *PgShopRepository {
	return &PgShopRepository{Pool: pool}
}

func (r *PgShopRepository) CreateShop(ctx context.Context, in models.ShopCreateInput, ownerID string) (*models.Shop, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var s models.Shop
	err = tx.QueryRow(ctx,
		`INSERT INTO shops (name, address, city, phone, owner_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, name, address, city, phone, owner_id, created_at`,
		in.Name, in.Address, in.City, in.Phone, ownerID,
	).Scan(&s.ID, &s.Name, &s.Address, &s.City, &s.Phone, &s.OwnerID, &s.CreatedAt)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `UPDATE users SET shop_id = $1 WHERE id = $2`, s.ID, ownerID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgShopRepository) GetShop(ctx context.Context, id string) (*models.Shop, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var s models.Shop
	err := r.Pool.QueryRow(ctx,
		`SELECT id, name, address, city, phone, owner_id, created_at FROM shops WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.Name, &s.Address, &s.City, &s.Phone, &s.OwnerID, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgShopRepository) ListShops(ctx context.Context, city string) ([]models.Shop, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `SELECT id, name, address, city, phone, owner_id, created_at FROM shops`
	args := []any{}
	if city != "" {
		query += ` WHERE city ILIKE $1`
		args = append(args, city)
	}
	query += ` ORDER BY name`

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shops := make([]models.Shop, 0)
	for rows.Next() {
		var s models.Shop
		if err := rows.Scan(&s.ID, &s.Name, &s.Address, &s.City, &s.Phone, &s.OwnerID, &s.CreatedAt); err != nil {
			return nil, err
		}
		shops = append(shops, s)
	}
	return shops, rows.Err()
}

func (r *PgShopRepository) UpdateShop(ctx context.Context, id string, in models.ShopUpdateInput) (*models.Shop, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var s models.Shop
	err := r.Pool.QueryRow(ctx,
		`UPDATE shops SET
			name = COALESCE($1, name),
			address = COALESCE($2, address),
			city = COALESCE($3, city),
			phone = COALESCE($4, phone)
		 WHERE id = $5
		 RETURNING id, name, address, city, phone, owner_id, created_at`,
		in.Name, in.Address, in.City, in.Phone, id,
	).Scan(&s.ID, &s.Name, &s.Address, &s.City, &s.Phone, &s.OwnerID, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgShopRepository) GetShopHours(ctx context.Context, shopID string) ([]models.ShopHours, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.Pool.Query(ctx,
		`SELECT id, shop_id, weekday, is_closed, COALESCE(open_time::text, ''), COALESCE(close_time::text, '')
		 FROM shop_hours WHERE shop_id = $1 ORDER BY weekday`,
		shopID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hours := make([]models.ShopHours, 0)
	for rows.Next() {
		var h models.ShopHours
		if err := rows.Scan(&h.ID, &h.ShopID, &h.Weekday, &h.IsClosed, &h.OpenTime, &h.CloseTime); err != nil {
			return nil, err
		}
		hours = append(hours, h)
	}
	return hours, rows.Err()
}

func (r *PgShopRepository) SetShopHours(ctx context.Context, shopID string, hours []models.ShopHours) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, h := range hours {
		var openTime, closeTime any
		if !h.IsClosed {
			openTime, closeTime = h.OpenTime, h.CloseTime
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO shop_hours (shop_id, weekday, is_closed, open_time, close_time)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (shop_id, weekday) DO UPDATE SET
				is_closed = EXCLUDED.is_closed,
				open_time = EXCLUDED.open_time,
				close_time = EXCLUDED.close_time`,
			shopID, h.Weekday, h.IsClosed, openTime, closeTime,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
