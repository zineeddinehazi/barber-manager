# Barber Manager API

Backend for a barber shop management and reservation system. See
[`PLAN.md`](./PLAN.md) for the full design (schema, endpoints, approval
workflow, availability engine).

## Setup

```bash
cp .env.example .env
docker compose up --build
```

This starts Postgres, runs all migrations, and serves the API on
`http://localhost:8080`. Then bootstrap the first shop + owner:

```bash
make seed-owner
```

## Local development (without Docker)

```bash
cp .env.example .env    # point DATABASE_URL at your own Postgres
make migrate_up
air                     # or: make run
```

## Testing

```bash
make test              # unit + pure-logic tests, no Postgres required
make test-integration  # spins up a real Postgres via testcontainers-go
```

On Docker Desktop (including the WSL2 backend), testcontainers-go can resolve
the exposed port against the internal bridge IP instead of `localhost`,
causing a connection timeout. `make test-integration` already sets
`TESTCONTAINERS_HOST_OVERRIDE=127.0.0.1` to avoid this; if you run
`go test -tags=integration ./...` directly, set that env var yourself.

## API overview

Base path: `/api/v1`. Full endpoint table in [`PLAN.md`](./PLAN.md#8-api-endpoints).

- **Public** (no auth): browse shops, barbers, services, availability, ratings.
- **Customer**: book/cancel reservations, rate a barber after a completed visit.
- **Barber** (`/barbers/me/...`): manage own bio, propose schedule/price
  changes (owner approval required), manage own bookings.
- **Owner** (`/shops/:id/...`): manage shop info/hours, onboard barbers,
  create services, approve/reject barber proposals, view all bookings.
