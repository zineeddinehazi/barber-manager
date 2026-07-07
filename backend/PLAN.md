# Barber Shop Management & Reservation API — Design Plan

Status: **planning only** — no code has been written yet. This document is the complete
spec to execute from. Everything under `backend/` described here does not exist yet
except this file.

---

## 1. Overview & Assumptions

Product: a backend API for barber shops in Algeria. Two customer-facing personas:

- **Customers** — browse shops/barbers, view schedules & prices, book/cancel
  reservations, rate barbers after a completed visit.
- **Shop staff** — split into two roles:
  - **Owner/Manager** — owns a shop, manages barbers, approves schedule/price
    changes, manages shop-level hours and services.
  - **Barber** — works at a shop, proposes their own working hours and prices
    (subject to owner approval), manages their own bookings.

Assumptions made to keep the plan unambiguous (revisit if wrong):

1. **Multi-tenant from day one.** Since the product will be *sold to multiple
   barber shops* in Algeria, the schema is tenant-aware via a `shops` table and a
   `shop_id` foreign key on every shop-scoped resource. A single-shop deployment
   is just a tenant with one row in `shops` — no schema change needed later.
2. Auth is email + password + JWT, matching the reference scaffold. Phone number
   is stored as a required contact field (common for SMS/WhatsApp reminders and
   for a barber-shop audience that may not check email) but is **not** the login
   credential in v1. OTP/phone-login is called out as a future extension point.
3. Payments are **out of scope** for v1 (cash-on-visit is the norm). The schema
   leaves room for a `payment_status` field on reservations for later.
4. One `owner` per shop for v1 (simplifies approval routing: barber → shop's
   owner). Multiple managers per shop is a future extension (would just widen
   the `shops.owner_id` single-FK into a `shop_staff` join table).
5. Timezone: all shops operate in `Africa/Algiers` (single timezone). Store all
   timestamps in `TIMESTAMPTZ` regardless, so this is a non-issue if it changes.

---

## 2. Domain Model & Roles

### Roles (enum `user_role`)
- `customer`
- `barber`
- `owner`

(No platform super-admin role in v1 — add later as `platform_admin` if a support
back-office is needed.)

### Core entities
- **Shop** — the tenant. Has an owner, opening hours, address, services.
- **User** — one account, one role, optionally linked to a shop (barbers/owners
  are linked; customers are not).
- **BarberProfile** — 1:1 extension of a `barber`-role user, holds bio, average
  rating (denormalized), active/inactive status.
- **ShopHours** — the shop's weekly opening/closing hours (when the shop itself
  is open at all).
- **WorkSchedule** — a barber's recurring weekly working hours *within* shop
  hours. Editable by the barber but only effective once approved.
- **ScheduleException** — one-off overrides (day off, holiday, custom hours for
  a specific date) for a barber.
- **Service** — a bookable service (e.g. "haircut", "beard trim") with a price
  and duration, offered shop-wide or per-barber. Price/duration edits by a
  barber go through the same approval flow as schedule edits.
- **Reservation** — a booking: customer + barber + service + time range +
  status.
- **Rating** — a 1–5 review of a barber, tied 1:1 to a completed reservation.
- **ApprovalRequest** — generic pending-change record used for both schedule
  and service/price edits proposed by a barber.

### Entity relationship summary
```
shops 1───* users (owner_id on shops, shop_id on users for barbers/owners)
shops 1───* shop_hours
shops 1───* services
users(barber) 1───1 barber_profiles
users(barber) 1───* work_schedules
users(barber) 1───* schedule_exceptions
users(barber) 1───* approval_requests (proposer)
shops 1───* reservations
users(barber) 1───* reservations
users(customer) 1───* reservations
services 1───* reservations
reservations 1───1 ratings (nullable, created after completion)
users(barber) 1───* ratings (denormalized avg on barber_profiles)
```

---

## 3. Database Schema (Postgres)

All tables use `UUID PRIMARY KEY DEFAULT gen_random_uuid()` unless noted. All
timestamps `TIMESTAMPTZ NOT NULL DEFAULT now()`. Requires the `pgcrypto`
extension (for `gen_random_uuid()`) and `btree_gist` (for the overlap-prevention
exclusion constraint on reservations).

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TYPE user_role AS ENUM ('customer', 'barber', 'owner');
CREATE TYPE approval_status AS ENUM ('pending', 'approved', 'rejected');
CREATE TYPE reservation_status AS ENUM ('pending', 'confirmed', 'cancelled', 'completed', 'no_show');
CREATE TYPE approval_target AS ENUM ('work_schedule', 'service');

-- shops -----------------------------------------------------------------
CREATE TABLE shops (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    address     TEXT NOT NULL,
    city        TEXT NOT NULL,
    phone       TEXT NOT NULL,
    owner_id    UUID, -- FK added after users table exists (circular dep)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- users -------------------------------------------------------------------
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id       UUID REFERENCES shops(id) ON DELETE SET NULL, -- null for customers
    role          user_role NOT NULL,
    full_name     TEXT NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    phone         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE shops
    ADD CONSTRAINT fk_shops_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE RESTRICT;

CREATE INDEX idx_users_shop_id ON users(shop_id);

-- barber_profiles ---------------------------------------------------------
CREATE TABLE barber_profiles (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    bio           TEXT NOT NULL DEFAULT '',
    is_active     BOOLEAN NOT NULL DEFAULT true,
    avg_rating    NUMERIC(3,2) NOT NULL DEFAULT 0,
    rating_count  INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- shop_hours (shop-level opening/closing per weekday) ---------------------
CREATE TABLE shop_hours (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id    UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    weekday    SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6), -- 0=Sunday
    is_closed  BOOLEAN NOT NULL DEFAULT false,
    open_time  TIME, -- null when is_closed
    close_time TIME,
    UNIQUE (shop_id, weekday)
);

-- work_schedules (barber recurring weekly hours) --------------------------
CREATE TABLE work_schedules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    barber_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weekday     SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    is_working  BOOLEAN NOT NULL DEFAULT true,
    start_time  TIME,
    end_time    TIME,
    status      approval_status NOT NULL DEFAULT 'approved', -- 'approved' = currently live version
    UNIQUE (barber_id, weekday, status) -- at most one pending + one approved row per weekday
);

-- schedule_exceptions (one-off day overrides) ------------------------------
CREATE TABLE schedule_exceptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    barber_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date        DATE NOT NULL,
    is_working  BOOLEAN NOT NULL DEFAULT false, -- false = day off/holiday
    start_time  TIME,
    end_time    TIME,
    reason      TEXT NOT NULL DEFAULT '',
    UNIQUE (barber_id, date)
);

-- services ------------------------------------------------------------------
CREATE TABLE services (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id          UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    barber_id        UUID REFERENCES users(id) ON DELETE CASCADE, -- null = shop-wide default
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    price_dzd        NUMERIC(10,2) NOT NULL,
    duration_minutes INT NOT NULL CHECK (duration_minutes > 0),
    status           approval_status NOT NULL DEFAULT 'approved',
    is_active        BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_services_shop_id ON services(shop_id);
CREATE INDEX idx_services_barber_id ON services(barber_id);

-- approval_requests (barber-proposed changes awaiting owner approval) ------
CREATE TABLE approval_requests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id       UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    barber_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type   approval_target NOT NULL,
    target_id     UUID NOT NULL, -- points at work_schedules.id or services.id (status='pending' row)
    payload       JSONB NOT NULL, -- proposed field values, for audit/history
    status        approval_status NOT NULL DEFAULT 'pending',
    reviewed_by   UUID REFERENCES users(id),
    reviewed_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_approval_requests_shop_status ON approval_requests(shop_id, status);

-- reservations ----------------------------------------------------------
CREATE TABLE reservations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id      UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    barber_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    customer_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    service_id   UUID NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    starts_at    TIMESTAMPTZ NOT NULL,
    ends_at      TIMESTAMPTZ NOT NULL CHECK (ends_at > starts_at),
    status       reservation_status NOT NULL DEFAULT 'pending',
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- prevents any overlapping reservation for the same barber at the DB level,
    -- regardless of race conditions at the application layer
    EXCLUDE USING gist (
        barber_id WITH =,
        tstzrange(starts_at, ends_at) WITH &&
    ) WHERE (status NOT IN ('cancelled', 'no_show'))
);

CREATE INDEX idx_reservations_barber_time ON reservations(barber_id, starts_at);
CREATE INDEX idx_reservations_customer ON reservations(customer_id);
CREATE INDEX idx_reservations_shop_time ON reservations(shop_id, starts_at);

-- ratings -----------------------------------------------------------------
CREATE TABLE ratings (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id UUID NOT NULL UNIQUE REFERENCES reservations(id) ON DELETE CASCADE,
    barber_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    customer_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    score          SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    comment        TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ratings_barber_id ON ratings(barber_id);
```

**Denormalized rating aggregate**: update `barber_profiles.avg_rating` /
`rating_count` inside the same DB transaction as the `INSERT INTO ratings`
(application-level, in the repository method — no DB trigger, to keep logic in
Go and testable/mockable). This matches the "raw SQL, no hidden magic" style of
the reference scaffold.

**Overlap prevention**: the `EXCLUDE` constraint above is the source of truth;
the application must still catch the resulting unique-violation error
(Postgres error code `23P01`) and translate it to a `409 Conflict` /
`ErrSlotUnavailable` at the repository layer — do not rely solely on an
application-level "check then insert" query, which races under concurrent
booking attempts.

---

## 4. Approval Workflow Design

Barbers can freely **propose** changes to their working hours or service
prices/durations, but changes only take effect once the shop owner approves.

Mechanics:
1. Barber calls `PUT /barbers/me/schedule` or `PUT /barbers/me/services/:id`.
2. Handler does **not** mutate the live (`status='approved'`) row directly.
   Instead it:
   - Inserts a new `work_schedules`/`services` row with `status='pending'`
     holding the proposed values (for schedules: one pending row per
     changed weekday; for services: a pending row is not a new service row —
     instead store proposed diffs directly in `approval_requests.payload` and
     patch the existing service row's `status` to `'pending'` so its edit is
     visibly in-flight while the old approved values stay readable until
     approval).
   - Inserts a matching `approval_requests` row (`target_type`, `target_id`,
     `payload` = proposed fields) with `status='pending'`.
3. Owner lists pending requests: `GET /shops/:id/approval-requests`.
4. Owner approves (`PATCH /approval-requests/:id/approve`):
   - Repository method, in one DB transaction:
     - Sets the approval request `status='approved'`, `reviewed_by`, `reviewed_at`.
     - Applies the `payload` onto the live row (for schedules: replace the
       approved row's `start_time`/`end_time`/`is_working` for that weekday;
       for services: update `price_dzd`/`duration_minutes` and flip
       `status` back to `'approved'`).
5. Owner rejects (`PATCH /approval-requests/:id/reject`): sets
   `status='rejected'` on the request and discards the pending row/payload
   without touching the live values.

This keeps a full audit trail (every proposal, whether approved or rejected,
stays in `approval_requests`) while customer-facing reads (available slots,
service prices) only ever show `status='approved'` data.

---

## 5. Booking & Availability Engine

To compute a barber's bookable slots for a given date (`GET
/shops/:shopId/barbers/:barberId/availability?date=YYYY-MM-DD&serviceId=...`):

1. Resolve `shop_hours` for that weekday → shop open/close window (if closed,
   return empty).
2. Resolve `work_schedules` (`status='approved'`) for that barber/weekday →
   intersect with shop hours → barber's working window for the day.
3. Check `schedule_exceptions` for that specific date → if present, it
   **overrides** step 2 entirely (day off, or custom hours for that date).
4. Fetch the service's `duration_minutes`.
5. Fetch existing `reservations` for that barber on that date where
   `status NOT IN ('cancelled','no_show')`.
6. Walk the working window in `duration_minutes` increments (or a configurable
   slot granularity, e.g. 15 min, snapping the service duration onto the grid),
   emitting a slot as available only if `[slot_start, slot_end)` doesn't
   overlap any existing reservation and fits fully inside the working window.
7. Return the list of available `{start, end}` slots.

This logic lives in a plain Go package (`internal/availability`), pure
functions over already-fetched data (no DB calls inside the algorithm itself)
— this is what makes it unit-testable without a database or mocks: feed it
fixed working-window/exception/reservation fixtures and assert the returned
slots.

Booking creation (`POST /reservations`) re-validates the requested slot
against the same rules server-side before insert (never trust a slot the
client claims is free), and relies on the `EXCLUDE` constraint as the final
race-condition backstop.

---

## 6. Rating System

- Only a `customer` who has a `reservations` row with that `barber_id`,
  `status='completed'`, and no existing rating for that reservation may rate.
- `POST /reservations/:id/rating` — the reservation ID (not the barber ID) is
  the route key, so the handler can directly verify ownership + completion
  status from the reservation row before allowing the insert.
- One rating per reservation (`UNIQUE` on `ratings.reservation_id`), so a
  customer can't spam ratings for repeat visits — each visit is rated once.
- Reading: `GET /barbers/:barberId/ratings` (paginated list + the
  denormalized `avg_rating`/`rating_count` from `barber_profiles`, so a
  simple shop/barber listing page doesn't need to aggregate on every request).
- Customers are **never** rated — no schema or endpoint exists for that
  direction, by design (`Detail 6` of the spec).

---

## 7. Auth & Authorization

Reuse the reference scaffold's JWT approach (HS256, `golang-jwt/jwt/v5`,
`AuthMiddleware` validating `Authorization: Bearer <token>`), extended with
two extra claims:

```go
type Claims struct {
    UserID string `json:"user_id"`
    Role   string `json:"role"`    // "customer" | "barber" | "owner"
    ShopID string `json:"shop_id"` // "" for customers
    jwt.RegisteredClaims
}
```

Middleware stack:
- `middleware.Auth(cfg)` — validates the JWT, sets `user_id`, `role`,
  `shop_id` in the Gin context. Unchanged in spirit from the scaffold.
- `middleware.RequireRole(roles ...string)` — 403s if `c.GetString("role")`
  isn't in the allowed set. Used to gate owner-only and barber-only routes.
- `middleware.RequireOwnShop()` — for owner routes that take a `:shopId` path
  param, 403s unless it matches the JWT's `shop_id`. Prevents an owner of shop
  A from managing shop B by guessing IDs.
- `middleware.RequireSelfOrOwner()` — for barber routes acting on
  `:barberId`, allow if the caller *is* that barber, or is the owner of that
  barber's shop.

Registration is split by role:
- `POST /auth/register` — customer self-registration (public).
- Barber/owner accounts are **not** self-registered through a public endpoint.
  An owner account is created via a (v1: manual/admin-seeded, documented as a
  one-off `make seed-owner` step) bootstrap; the owner then creates barber
  accounts via `POST /shops/:id/barbers` (owner-only), which generates a
  temporary password the barber changes on first login (`PATCH /auth/password`).
  This models the real-world flow: the shop owner buying the product onboards
  their own barbers.

---

## 8. API Endpoints

Base path: `/api/v1`. All non-public routes require `Authorization: Bearer`.

### Public / Auth
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/auth/register` | none | customer self-registration |
| POST | `/auth/login` | none | returns JWT for any role |
| PATCH | `/auth/password` | any | change own password |

### Public browsing (customer-facing, read-only, no auth required so it can double as a public storefront)
| Method | Path | Description |
|---|---|---|
| GET | `/shops` | list shops (filter by `city`) |
| GET | `/shops/:shopId` | shop details + hours |
| GET | `/shops/:shopId/barbers` | active barbers at a shop, with avg rating |
| GET | `/shops/:shopId/barbers/:barberId` | barber profile + bio + rating |
| GET | `/shops/:shopId/services` | approved, active services + prices |
| GET | `/shops/:shopId/barbers/:barberId/availability` | available slots for a date + service |
| GET | `/barbers/:barberId/ratings` | paginated ratings list |

### Customer (role=customer)
| Method | Path | Description |
|---|---|---|
| POST | `/reservations` | book a slot |
| GET | `/reservations/me` | own bookings (upcoming + history) |
| PATCH | `/reservations/:id/cancel` | cancel own pending/confirmed booking |
| POST | `/reservations/:id/rating` | rate after completion |

### Barber (role=barber)
| Method | Path | Description |
|---|---|---|
| GET | `/barbers/me` | own profile |
| PATCH | `/barbers/me` | edit bio (no approval needed) |
| GET | `/barbers/me/schedule` | own approved + pending schedule |
| PUT | `/barbers/me/schedule` | propose weekly schedule change → approval request |
| POST | `/barbers/me/schedule/exceptions` | add a day-off/custom-hours exception (no approval needed — barber's own one-off availability) |
| GET | `/barbers/me/services` | own services |
| PUT | `/barbers/me/services/:id` | propose price/duration change → approval request |
| GET | `/barbers/me/reservations` | own upcoming/past bookings |
| PATCH | `/barbers/me/reservations/:id/complete` | mark completed |
| PATCH | `/barbers/me/reservations/:id/no-show` | mark no-show |
| GET | `/barbers/me/approval-requests` | own proposals + status |

### Owner (role=owner)
| Method | Path | Description |
|---|---|---|
| PUT | `/shops/:id` | edit shop info |
| PUT | `/shops/:id/hours` | edit shop weekly opening hours |
| POST | `/shops/:id/barbers` | create a barber account for this shop |
| PATCH | `/shops/:id/barbers/:barberId/status` | activate/deactivate a barber |
| POST | `/shops/:id/services` | create a shop-wide service |
| GET | `/shops/:id/approval-requests` | list pending (+ filter by status) |
| PATCH | `/shops/:id/approval-requests/:requestId/approve` | approve → apply payload |
| PATCH | `/shops/:id/approval-requests/:requestId/reject` | reject → discard payload |
| GET | `/shops/:id/reservations` | all bookings shop-wide, filter by date/barber/status |

Note: approval-request routes are nested under `/shops/:id/...` (not flat
`/approval-requests/:id/...` as originally sketched here) so `RequireOwnShop`
can scope them the same way as every other owner route, and the repository's
`Approve`/`Reject` additionally filter by `shop_id` — otherwise an owner could
approve/reject another shop's request by guessing its UUID.

Every handler resolves the effective `shopId`/`barberId`/`customerId` filters
from the JWT context (`user_id`, `role`, `shop_id`), never from a client-
supplied "act as" parameter.

---

## 9. Project Structure

Following the reference scaffold's layered pattern (`cmd` → `internal/{config,
database, handlers, repository, models, middleware, utils}`), extended with an
`availability` package for the pure slot-computation logic and multiplied
across the resources above:

```
backend/
├── .air.toml
├── .env.example
├── .gitignore
├── Makefile
├── README.md
├── go.mod
├── go.sum
├── Dockerfile
├── docker-compose.yml
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── seed/
│       └── main.go                     # one-off: create first owner account
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── database/
│   │   └── postgres.go
│   ├── availability/
│   │   ├── availability.go             # pure slot-computation logic (Step 5)
│   │   └── availability_test.go
│   ├── models/
│   │   ├── shop.go
│   │   ├── shop_hours.go
│   │   ├── user.go
│   │   ├── barber_profile.go
│   │   ├── work_schedule.go
│   │   ├── schedule_exception.go
│   │   ├── service.go
│   │   ├── approval_request.go
│   │   ├── reservation.go
│   │   ├── rating.go
│   │   ├── request_register.go
│   │   ├── request_login.go
│   │   ├── response_login.go
│   │   ├── shop_create_input.go
│   │   ├── barber_create_input.go
│   │   ├── schedule_update_input.go
│   │   ├── service_create_input.go
│   │   ├── service_update_input.go
│   │   ├── reservation_create_input.go
│   │   └── rating_create_input.go
│   ├── repository/
│   │   ├── errors.go                   # ErrNotFound, ErrSlotUnavailable, ErrForbidden
│   │   ├── shop_repository.go
│   │   ├── user_repository.go
│   │   ├── barber_repository.go
│   │   ├── schedule_repository.go
│   │   ├── service_repository.go
│   │   ├── approval_repository.go
│   │   ├── reservation_repository.go
│   │   ├── rating_repository.go
│   │   └── mocks/                      # generated by mockgen, one file per interface above
│   ├── handlers/
│   │   ├── respond.go
│   │   ├── auth_handlers.go
│   │   ├── shop_handlers.go
│   │   ├── barber_handlers.go
│   │   ├── schedule_handlers.go
│   │   ├── service_handlers.go
│   │   ├── approval_handlers.go
│   │   ├── reservation_handlers.go
│   │   ├── rating_handlers.go
│   │   ├── *_handlers_test.go           # one test file per handler file above
│   ├── middleware/
│   │   ├── auth_middleware.go
│   │   └── rbac_middleware.go           # RequireRole / RequireOwnShop / RequireSelfOrOwner
│   └── utils/
│       ├── generate_token.go
│       └── hash_password.go
└── migrations/
    ├── 000001_create_extensions.up.sql
    ├── 000001_create_extensions.down.sql
    ├── 000002_create_shops_and_users.up.sql
    ├── 000002_create_shops_and_users.down.sql
    ├── 000003_create_barber_profiles.up.sql
    ├── 000003_create_barber_profiles.down.sql
    ├── 000004_create_shop_hours.up.sql
    ├── 000004_create_shop_hours.down.sql
    ├── 000005_create_work_schedules.up.sql
    ├── 000005_create_work_schedules.down.sql
    ├── 000006_create_schedule_exceptions.up.sql
    ├── 000006_create_schedule_exceptions.down.sql
    ├── 000007_create_services.up.sql
    ├── 000007_create_services.down.sql
    ├── 000008_create_approval_requests.up.sql
    ├── 000008_create_approval_requests.down.sql
    ├── 000009_create_reservations.up.sql
    ├── 000009_create_reservations.down.sql
    ├── 000010_create_ratings.up.sql
    └── 000010_create_ratings.down.sql
```

Each migration pair contains exactly the corresponding `CREATE TABLE`
statement(s) from Section 3, split so every table is independently
revertible; `000002` bundles `shops`+`users` together only because of their
circular FK (`shops.owner_id` → `users.id`, `users.shop_id` → `shops.id`) —
create both tables first without the `shops.owner_id` FK, then `ALTER TABLE
shops ADD CONSTRAINT ...` at the end of the same migration file.

---

## 10. Tech Stack & Dependencies

Matching the reference scaffold's choices (raw SQL via pgx, no ORM, explicit
DI, no globals), plus the additions this domain needs:

| Concern | Choice | Notes |
|---|---|---|
| HTTP framework | `github.com/gin-gonic/gin` | same as scaffold |
| DB driver/pool | `github.com/jackc/pgx/v5/pgxpool` | raw SQL, manual `Scan` |
| Migrations | `github.com/golang-migrate/migrate/v4` CLI | sequential, never edit existing files |
| Auth | `github.com/golang-jwt/jwt/v5` | HS256, `Claims` extended per Section 7 |
| Password hashing | `golang.org/x/crypto/bcrypt` | default cost |
| Config | `github.com/joho/godotenv` + `os.Getenv` | optional `.env`, real env vars always win (scaffold fix) |
| Logging | stdlib `log/slog` | structured JSON logs; Gin's default logger replaced with a `slog`-backed middleware — newer stdlib-first practice, avoids an extra dependency (zerolog/zap) for a project this size |
| Testing (unit) | stdlib `testing` + `net/http/httptest` + `go.uber.org/mock/gomock` | mirrors scaffold's handler test pattern |
| Testing (assertions) | `github.com/stretchr/testify` (`assert`/`require`) | not in the original scaffold, but standard/expected in "latest best practices" Go test code — cuts boilerplate vs. manual `if got != want` |
| Testing (repository/integration) | `github.com/testcontainers/testcontainers-go` + its Postgres module | spins up a real ephemeral Postgres for repository-layer tests (overlap-exclusion constraint, transactions) that mocks can't meaningfully exercise |
| Containerization | Docker + Docker Compose v2 | Section 13 |
| Hot reload (dev) | `air` (`.air.toml`, copied from scaffold unchanged) | |

`go.mod` dependency list to fetch at scaffolding time:
```
go get github.com/gin-gonic/gin \
       github.com/jackc/pgx/v5/pgxpool \
       github.com/golang-jwt/jwt/v5 \
       golang.org/x/crypto/bcrypt \
       github.com/joho/godotenv \
       github.com/stretchr/testify \
       go.uber.org/mock/gomock \
       github.com/testcontainers/testcontainers-go \
       github.com/testcontainers/testcontainers-go/modules/postgres
```

---

## 11. Repository Interfaces (mocking boundary)

Same principle as the scaffold: handlers depend on interfaces, never on
`*pgxpool.Pool` directly, so `go.uber.org/mock` can generate mocks per
interface. One interface per resource file in `internal/repository/`:

```go
type ShopRepository interface {
    CreateShop(ctx context.Context, in models.ShopCreateInput, ownerID string) (*models.Shop, error)
    GetShop(ctx context.Context, id string) (*models.Shop, error)
    ListShops(ctx context.Context, city string) ([]models.Shop, error)
    UpdateShop(ctx context.Context, id string, in models.ShopUpdateInput) (*models.Shop, error)
    SetShopHours(ctx context.Context, shopID string, hours []models.ShopHours) error
}

type UserRepository interface {
    CreateUser(ctx context.Context, in models.RegisterInput, role string, shopID *string) (*models.User, error)
    GetUserByEmail(ctx context.Context, email string) (*models.User, error)
    GetUserByID(ctx context.Context, id string) (*models.User, error)
    UpdatePassword(ctx context.Context, userID, newHash string) error
}

type BarberRepository interface {
    ListActiveBarbers(ctx context.Context, shopID string) ([]models.BarberWithProfile, error)
    GetBarberProfile(ctx context.Context, barberID string) (*models.BarberWithProfile, error)
    UpdateBio(ctx context.Context, barberID, bio string) error
    SetActive(ctx context.Context, barberID string, active bool) error
    // called inside the same tx as rating insert:
    RecalculateAvgRating(ctx context.Context, tx pgx.Tx, barberID string) error
}

type ScheduleRepository interface {
    GetApprovedSchedule(ctx context.Context, barberID string) ([]models.WorkSchedule, error)
    GetShopHours(ctx context.Context, shopID string) ([]models.ShopHours, error)
    GetExceptions(ctx context.Context, barberID string, from, to time.Time) ([]models.ScheduleException, error)
    ProposeSchedule(ctx context.Context, barberID string, in models.ScheduleUpdateInput) (*models.ApprovalRequest, error)
    AddException(ctx context.Context, barberID string, in models.ScheduleException) error
}

type ServiceRepository interface {
    CreateService(ctx context.Context, shopID string, in models.ServiceCreateInput) (*models.Service, error)
    ListServices(ctx context.Context, shopID string) ([]models.Service, error)
    GetService(ctx context.Context, id string) (*models.Service, error)
    ProposeUpdate(ctx context.Context, barberID, serviceID string, in models.ServiceUpdateInput) (*models.ApprovalRequest, error)
}

type ApprovalRepository interface {
    ListPending(ctx context.Context, shopID string) ([]models.ApprovalRequest, error)
    ListByBarber(ctx context.Context, barberID string) ([]models.ApprovalRequest, error)
    Approve(ctx context.Context, requestID, reviewerID string) error // applies payload transactionally
    Reject(ctx context.Context, requestID, reviewerID string) error
}

type ReservationRepository interface {
    CreateReservation(ctx context.Context, in models.ReservationCreateInput) (*models.Reservation, error)
    GetReservation(ctx context.Context, id string) (*models.Reservation, error)
    ListForCustomer(ctx context.Context, customerID string) ([]models.Reservation, error)
    ListForBarber(ctx context.Context, barberID string, from, to time.Time) ([]models.Reservation, error)
    ListForShop(ctx context.Context, shopID string, filter models.ReservationFilter) ([]models.Reservation, error)
    UpdateStatus(ctx context.Context, id, status string) error
    Cancel(ctx context.Context, id, customerID string) error
}

type RatingRepository interface {
    CreateRating(ctx context.Context, in models.RatingCreateInput) (*models.Rating, error) // wraps RecalculateAvgRating in same tx
    ListForBarber(ctx context.Context, barberID string, page, pageSize int) ([]models.Rating, int, error)
}
```

`internal/repository/errors.go` sentinels (checked with `errors.Is`, mirroring
the scaffold's `ErrNotFound` fix):
```go
var (
    ErrNotFound         = errors.New("resource not found")
    ErrSlotUnavailable  = errors.New("requested time slot is no longer available")
    ErrAlreadyRated     = errors.New("reservation already rated")
    ErrNotCompleted     = errors.New("reservation is not completed yet")
    ErrDuplicateEmail   = errors.New("email already registered")
)
```

`ErrSlotUnavailable` is returned when the `EXCLUDE` constraint rejects an
insert (Postgres code `23P01`) — translate that specific pgx error code inside
`ReservationRepository.CreateReservation`, everything else falls through to a
generic 500.

---

## 12. Testing Strategy

Three layers, matching the "write tests for the whole API" requirement:

1. **Handler tests** (`internal/handlers/*_handlers_test.go`) — table-driven,
   `httptest` + `gomock`-generated repository mocks, one file per handler
   file. Minimum coverage per resource: success (2xx), not-found (404),
   forbidden/wrong-role (403), validation error (400), and — for
   reservations specifically — a `409` test for `ErrSlotUnavailable`.
   No real DB involved; fast, run on every `go test ./...`.

2. **Pure logic tests** (`internal/availability/availability_test.go`) — feed
   fixed shop-hours/work-schedule/exception/existing-reservation fixtures,
   assert exact returned slot list. This is the highest-value test file in
   the project since it's the trickiest logic (four data sources merged into
   one availability window) and needs zero mocking.

3. **Repository/integration tests** (`internal/repository/*_repository_test.go`,
   build-tagged `integration` so `go test ./...` skips them by default and CI
   runs them separately with `go test -tags=integration ./...`) — spin up a
   real Postgres via `testcontainers-go`, run migrations against it, and
   verify things a mock can't: the `EXCLUDE` constraint actually rejects
   overlapping bookings, the approval-apply transaction actually mutates the
   live row, `RecalculateAvgRating` produces the right average.

Makefile target `test` runs layer 1+2 (fast, no Docker needed);
`test-integration` runs layer 3.

---

## 13. Docker & Docker Compose

`backend/Dockerfile` (multi-stage, matching current Go best practice — small
final image, no build toolchain shipped):
```dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
```

`backend/docker-compose.yml` — api + postgres, healthchecked so the api
container waits for Postgres to actually be ready (not just "started"):
```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: barber
      POSTGRES_PASSWORD: barber
      POSTGRES_DB: barbershop
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U barber -d barbershop"]
      interval: 5s
      timeout: 3s
      retries: 5

  migrate:
    image: migrate/migrate:v4.17.1
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - ./migrations:/migrations
    command: [
      "-path", "/migrations",
      "-database", "postgres://barber:barber@postgres:5432/barbershop?sslmode=disable",
      "up"
    ]

  api:
    build: .
    depends_on:
      migrate:
        condition: service_completed_successfully
    environment:
      DATABASE_URL: postgres://barber:barber@postgres:5432/barbershop?sslmode=disable
      PORT: "8080"
      JWT_SECRET: change-me-in-prod
    ports:
      - "8080:8080"

volumes:
  pgdata:
```

Running `docker compose up --build` should be sufficient to get a working API
against a fresh, migrated database with no manual steps.

---

## 14. Environment Variables (`.env.example`)

```
DATABASE_URL=postgres://barber:barber@localhost:5432/barbershop?sslmode=disable
PORT=8080
JWT_SECRET=change-me
JWT_EXPIRY_HOURS=24
```

---

## 15. Makefile Targets

Extends the scaffold's migration targets with test/docker targets:
```makefile
run:                 # go run ./cmd/api
build:                # go build -o ./tmp/main ./cmd/api
test:                 # go test ./...  (unit + pure-logic, no Docker required)
test-integration:     # go test -tags=integration ./...  (spins up testcontainers)
seed-owner:           # go run ./cmd/seed  (interactive: creates first shop+owner)
create_migration:     # migrate create -ext sql -dir migrations -seq <name>
migrate_up / migrate_down / migrate_down_all / migrate_version / migrate_force:
docker-up:            # docker compose up --build
docker-down:          # docker compose down -v
```

---

## 16. Implementation Roadmap (execute in this order)

1. Scaffold `go.mod`, folder tree (Section 9), `.env.example`, `.gitignore`,
   `.air.toml`.
2. `internal/config`, `internal/database` (unchanged from reference scaffold).
3. Migrations `000001`–`000010` (Section 3), verify `make migrate_up` against
   the compose Postgres.
4. `internal/models` — all structs (Section 9 file list).
5. `internal/repository` — interfaces + Pg implementations + `errors.go`, in
   this dependency order: `user_repository` → `shop_repository` →
   `barber_repository` → `schedule_repository` → `service_repository` →
   `approval_repository` → `reservation_repository` → `rating_repository`.
   Generate `mocks/` via `mockgen` for each interface as it's written.
6. `internal/availability` — pure slot logic + its unit tests (can be built
   and fully tested before any handler exists, since it has no DB/HTTP
   dependency).
7. `internal/middleware` — `auth_middleware.go` (JWT parsing, copied from
   scaffold + extended claims), `rbac_middleware.go` (`RequireRole`,
   `RequireOwnShop`, `RequireSelfOrOwner`).
8. `internal/handlers` — one resource at a time, each with its
   `*_handlers_test.go` written alongside (not deferred): auth → shops →
   barbers/schedule/services → approval-requests → reservations → ratings.
9. `cmd/api/main.go` — wire config → pool → all repositories → router groups
   per Section 8's endpoint table → graceful shutdown (`http.Server` +
   `signal.NotifyContext`, per scaffold Step 10).
10. `cmd/seed/main.go` — one-shot CLI to create the first shop + owner account
    (since owners aren't self-registered, Section 7).
11. `Dockerfile` + `docker-compose.yml` (Section 13); verify
    `docker compose up --build` serves a working, migrated API end to end.
12. Repository/integration tests (`testcontainers-go`), tagged `integration`,
    for: reservation overlap exclusion, approval-apply transaction, rating
    average recalculation.
13. Final pass: `go mod tidy && go build ./... && go vet ./... && make test &&
    make test-integration` all green before calling it done.

---

## 17. Explicitly Out of Scope for v1 (documented so it isn't accidentally re-derived mid-build)

- Online payments (cash-on-visit assumed).
- SMS/WhatsApp reminder delivery (schema has the contact fields; the actual
  sending integration is a separate follow-up task).
- Multiple owners/managers per shop.
- Platform super-admin role / back-office.
- Phone-number/OTP login (email+password only).
