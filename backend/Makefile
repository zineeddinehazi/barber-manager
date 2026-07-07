-include .env
export

run:
	go run ./cmd/api

build:
	go build -o ./tmp/main ./cmd/api

test:
	go test ./...

test-integration:
	TESTCONTAINERS_HOST_OVERRIDE=127.0.0.1 go test -tags=integration ./...

seed-owner:
	go run ./cmd/seed

create_migration:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

migrate_up:
	migrate -path migrations -database "$(DATABASE_URL)" -verbose up

migrate_down:
	migrate -path migrations -database "$(DATABASE_URL)" -verbose down 1

migrate_down_all:
	migrate -path migrations -database "$(DATABASE_URL)" -verbose down -all

migrate_version:
	migrate -path migrations -database "$(DATABASE_URL)" version

migrate_force:
	@read -p "Version to force: " v; \
	migrate -path migrations -database "$(DATABASE_URL)" force $$v

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v

.PHONY: run build test test-integration seed-owner create_migration migrate_up migrate_down migrate_down_all migrate_version migrate_force docker-up docker-down
