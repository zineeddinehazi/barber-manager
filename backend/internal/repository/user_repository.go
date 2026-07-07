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

type UserRepository interface {
	CreateUser(ctx context.Context, in models.RegisterInput, passwordHash, role string, shopID *string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)
	UpdatePassword(ctx context.Context, userID, newHash string) error
}

type PgUserRepository struct {
	Pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *PgUserRepository {
	return &PgUserRepository{Pool: pool}
}

func (r *PgUserRepository) CreateUser(ctx context.Context, in models.RegisterInput, passwordHash, role string, shopID *string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var u models.User
	err := r.Pool.QueryRow(ctx,
		`INSERT INTO users (shop_id, role, full_name, email, phone, password_hash)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, shop_id, role, full_name, email, phone, password_hash, created_at`,
		shopID, role, in.FullName, in.Email, in.Phone, passwordHash,
	).Scan(&u.ID, &u.ShopID, &u.Role, &u.FullName, &u.Email, &u.Phone, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}
	return &u, nil
}

func (r *PgUserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var u models.User
	err := r.Pool.QueryRow(ctx,
		`SELECT id, shop_id, role, full_name, email, phone, password_hash, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.ShopID, &u.Role, &u.FullName, &u.Email, &u.Phone, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *PgUserRepository) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var u models.User
	err := r.Pool.QueryRow(ctx,
		`SELECT id, shop_id, role, full_name, email, phone, password_hash, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.ShopID, &u.Role, &u.FullName, &u.Email, &u.Phone, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *PgUserRepository) UpdatePassword(ctx context.Context, userID, newHash string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tag, err := r.Pool.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, newHash, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
