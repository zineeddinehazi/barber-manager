//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testPool is shared by every integration test in this package. One Postgres
// container is started for the whole run (see TestMain), migrated once, and
// each test seeds its own rows via seedFixtures using a unique suffix so
// tests never collide.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(runIntegrationTests(m))
}

func runIntegrationTests(m *testing.M) int {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("barbershop_test"),
		postgres.WithUsername("barber"),
		postgres.WithPassword("barber"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		fmt.Println("failed to start postgres container:", err)
		return 1
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Println("failed to get connection string:", err)
		return 1
	}

	if err := runMigrations(connStr); err != nil {
		fmt.Println("failed to run migrations:", err)
		return 1
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Println("failed to connect pool:", err)
		return 1
	}
	defer pool.Close()
	testPool = pool

	return m.Run()
}

func runMigrations(connStr string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return err
	}

	mig, err := migrate.NewWithDatabaseInstance("file://../../migrations", "pgx5", driver)
	if err != nil {
		return err
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
