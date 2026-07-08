# Barber Manager API

A multi-tenant REST API for barber shop management and reservations, built
in Go. Each shop runs its own barbers, services, hours, and bookings behind
role-based access control, with an approval workflow so barbers can propose
schedule/price changes without touching live data directly.

## Features

- **Role-based access** — customer, barber, and owner tokens (JWT), each
  scoped to their own account or shop; no cross-tenant access to another
  shop's data.
- **Approval workflow** — a barber's proposed schedule or service-price
  change is staged as an `ApprovalRequest` and only takes effect once the
  shop owner approves it.
- **Availability engine** — computes real bookable slots from shop hours ∩
  the barber's approved weekly schedule ∩ one-off exceptions, minus existing
  reservations; booking re-validates against the same rules server-side.
- **Race-safe booking** — overlapping reservations for the same barber are
  rejected at the database level via a Postgres `EXCLUDE` constraint, not
  just an application-level check.
- **Ratings** — customers rate a barber after a completed visit; the
  barber's average rating updates atomically with the rating insert.

## Tech stack

| Layer | Choice |
|---|---|
| Language | Go 1.26 |
| HTTP | [Gin](https://github.com/gin-gonic/gin) |
| Database | PostgreSQL via [pgx](https://github.com/jackc/pgx) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Auth | JWT ([golang-jwt](https://github.com/golang-jwt/jwt)) |
| Testing | [testify](https://github.com/stretchr/testify), [gomock](https://github.com/uber-go/mock), [testcontainers-go](https://github.com/testcontainers/testcontainers-go) |

## Quickstart

```bash
cp .env.example .env
docker compose up --build
```

This starts Postgres, runs all migrations, and serves the API on
`http://localhost:8080`. Then bootstrap the first shop + owner:

```bash
make seed-owner
```

`make seed-owner` prompts for the owner's and shop's details and prints the
new shop ID and owner ID, which you'll need for the owner-only endpoints.

### Local development (without Docker)

```bash
cp .env.example .env    # point DATABASE_URL at your own Postgres
make migrate_up
air                      # live-reload, or: make run
```

## Testing

```bash
make test              # unit + pure-logic tests, no Postgres required
make test-integration   # spins up a real Postgres via testcontainers-go
```

On Docker Desktop (including the WSL2 backend), testcontainers-go can
resolve the exposed port against the internal bridge IP instead of
`localhost`, causing a connection timeout. `make test-integration` already
sets `TESTCONTAINERS_HOST_OVERRIDE=127.0.0.1` to avoid this; if you run
`go test -tags=integration ./...` directly, set that env var yourself.

## Roles

| Role | How created | Can do |
|---|---|---|
| **customer** | self-registers via `POST /auth/register` | browse shops/barbers/services, book/cancel reservations, rate a barber after a completed visit |
| **barber** | created by an owner via `POST /shops/:shopId/barbers` (gets a temp password) | edit own bio, propose schedule/price changes (need owner approval), manage own bookings |
| **owner** | created once via `make seed-owner` | manage shop info/hours, onboard barbers, create services, approve/reject barber proposals, view all shop bookings |

## API overview

Base path: `/api/v1`. All request/response bodies are JSON; protected routes
require `Authorization: Bearer <token>`.

### Auth
| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/auth/register` | none | customer self-registration |
| POST | `/auth/login` | none | any role, returns `{token, user}` |
| PATCH | `/auth/password` | any | body: `{current_password, new_password}` |

### Public browsing
| Method | Path | Notes |
|---|---|---|
| GET | `/shops?city=` | optional `city` filter |
| GET | `/shops/:shopId` | returns `{shop, hours}` |
| GET | `/shops/:shopId/barbers` | active barbers only |
| GET | `/shops/:shopId/barbers/:barberId` | includes `avg_rating`/`rating_count` |
| GET | `/shops/:shopId/services` | approved + active only |
| GET | `/shops/:shopId/barbers/:barberId/availability?serviceId=&date=YYYY-MM-DD` | returns `{slots: [{start, end}]}` |
| GET | `/barbers/:barberId/ratings?page=&page_size=` | paginated |

### Customer
| Method | Path | Notes |
|---|---|---|
| POST | `/reservations` | body: `{barber_id, service_id, starts_at, notes}` — validated against the barber's real availability, not just the DB overlap constraint |
| GET | `/reservations/me` | own bookings |
| PATCH | `/reservations/:id/cancel` | only own pending/confirmed bookings |
| POST | `/reservations/:id/rating` | body: `{score (1-5), comment}`, only after `completed` |

### Barber (`/barbers/me/...`)
| Method | Path | Notes |
|---|---|---|
| GET / PATCH | `/barbers/me` | own profile / update bio |
| GET | `/barbers/me/schedule` | own **approved** weekly schedule |
| PUT | `/barbers/me/schedule` | proposes a new weekly schedule (owner approval required) |
| POST | `/barbers/me/schedule/exceptions` | one-off day off / custom hours, applies immediately |
| GET | `/barbers/me/services` | own services, any status |
| PUT | `/barbers/me/services/:id` | proposes a price/duration change (owner approval required) |
| GET | `/barbers/me/reservations?from=&to=` | own bookings |
| PATCH | `/barbers/me/reservations/:id/complete` | mark a booking completed |
| PATCH | `/barbers/me/reservations/:id/no-show` | mark a booking no-show |
| GET | `/barbers/me/approval-requests` | own proposals + status |

### Owner (`/shops/:shopId/...`)
| Method | Path | Notes |
|---|---|---|
| PUT | `/shops/:shopId` | update shop info |
| PUT | `/shops/:shopId/hours` | weekly opening hours (7 entries) |
| POST | `/shops/:shopId/barbers` | onboard a barber, returns a temp password |
| PATCH | `/shops/:shopId/barbers/:barberId/status` | activate/deactivate a barber |
| POST | `/shops/:shopId/services` | create a service, optionally scoped to one barber |
| GET | `/shops/:shopId/approval-requests` | pending proposals from this shop's barbers |
| PATCH | `/shops/:shopId/approval-requests/:requestId/approve` \| `/reject` | applies or discards the proposal |
| GET | `/shops/:shopId/reservations?barberId=&status=` | all bookings shop-wide |

## Project layout

```
cmd/api          entrypoint, route wiring
cmd/seed         interactive first-shop/owner bootstrap
internal/handlers    HTTP handlers (gin)
internal/repository  Postgres access, one interface + impl per aggregate
internal/availability the pure slot-computation engine
internal/middleware  auth + RBAC middleware
internal/models      request/response and domain types
migrations           golang-migrate SQL migrations
```
