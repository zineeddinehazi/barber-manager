# Barber Manager API — Manual Testing Guide

Base URL once running: `http://localhost:8080/api/v1`. All request/response
bodies are JSON. Protected routes require `Authorization: Bearer <token>`.

## 0. Start it up

```bash
cd backend
cp .env.example .env      # first time only
docker compose up --build -d
make seed-owner           # interactive: creates the first shop + owner account
```

`make seed-owner` will prompt for owner name/email/phone/password and
shop name/address/city/phone, then print the shop ID and owner ID — you'll
need the shop ID for the owner-only endpoints below.

To stop: `docker compose down -v` (the `-v` also wipes the Postgres volume).

## 1. Roles & who can do what

| Role | How created | Can do |
|---|---|---|
| **customer** | self-registers via `POST /auth/register` | browse shops/barbers/services, book/cancel reservations, rate a barber after a completed visit |
| **barber** | created by an owner via `POST /shops/:shopId/barbers` (gets a temp password) | edit own bio, propose schedule/price changes (need owner approval), manage own bookings |
| **owner** | created once via `make seed-owner` | manage shop info/hours, onboard barbers, create services, approve/reject barber proposals, view all shop bookings |

## 2. Functional walkthrough (a full happy path)

This is the exact sequence to hand-test the whole system end to end.

1. **Login as owner** → `POST /auth/login` → save the token.
2. **Set shop hours** → `PUT /shops/:shopId/hours` (owner token).
3. **Create a barber** → `POST /shops/:shopId/barbers` (owner token) → note
   the returned `temp_password`.
4. **Login as the barber** with that temp password.
5. **Barber proposes a weekly schedule** → `PUT /barbers/me/schedule` (barber
   token) → response is an `ApprovalRequest` with `status: "pending"`.
6. **Owner approves it** → `PATCH /shops/:shopId/approval-requests/:requestId/approve`
   (owner token) → 204. The schedule is now live.
7. **Owner creates a service** → `POST /shops/:shopId/services` (owner token).
8. **Check availability (public, no token)** →
   `GET /shops/:shopId/barbers/:barberId/availability?serviceId=...&date=YYYY-MM-DD`
   → should return slots inside the approved schedule ∩ shop hours, in the
   `Africa/Algiers` timezone.
9. **Register a customer** → `POST /auth/register`, then login.
10. **Book a reservation** → `POST /reservations` (customer token) using one
    of the returned slot start times as `starts_at`.
11. **Try booking the same slot again** → should get **409** (the DB rejects
    the overlap even under a race, not just an app-level check).
12. **Barber marks it completed** → `PATCH /barbers/me/reservations/:id/complete`
    (barber token).
13. **Customer rates the barber** → `POST /reservations/:id/rating` (customer
    token) → the barber's `avg_rating`/`rating_count` update immediately,
    visible on `GET /shops/:shopId/barbers/:barberId`.

RBAC sanity checks worth trying by hand:
- Call an owner-only route with a customer token → **403**.
- Call any protected route with no `Authorization` header → **401**.
- Call `PUT /shops/:shopId` using a *different* shop's ID than the owner's
  own (from their JWT) → **403** ("not your shop").
- Try to rate a reservation that isn't `completed` yet → **409**.
- Try to rate the same reservation twice → **409**.

## 3. Full endpoint reference

### Auth
| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/auth/register` | none | customer self-registration |
| POST | `/auth/login` | none | any role, returns `{token, user}` |
| PATCH | `/auth/password` | any | body: `{current_password, new_password}` |

### Public browsing (no token needed)
| Method | Path | Notes |
|---|---|---|
| GET | `/shops?city=` | optional `city` filter |
| GET | `/shops/:shopId` | returns `{shop, hours}` |
| GET | `/shops/:shopId/barbers` | active barbers only |
| GET | `/shops/:shopId/barbers/:barberId` | includes `avg_rating`/`rating_count` |
| GET | `/shops/:shopId/services` | approved + active only |
| GET | `/shops/:shopId/barbers/:barberId/availability?serviceId=&date=YYYY-MM-DD` | returns `{slots: [{start, end}]}` |
| GET | `/barbers/:barberId/ratings?page=&page_size=` | paginated |

### Customer (role=customer)
| Method | Path | Notes |
|---|---|---|
| POST | `/reservations` | body: `{barber_id, service_id, starts_at, notes}` |
| GET | `/reservations/me` | own bookings |
| PATCH | `/reservations/:id/cancel` | only own pending/confirmed bookings |
| POST | `/reservations/:id/rating` | body: `{score (1-5), comment}`, only after `completed` |

### Barber (`/barbers/me/...`, role=barber, always acts on own account)
| Method | Path | Notes |
|---|---|---|
| GET | `/barbers/me` | own profile |
| PATCH | `/barbers/me` | body: `{bio}` |
| GET | `/barbers/me/schedule` | own **approved** weekly schedule |
| PUT | `/barbers/me/schedule` | body: `{days: [{weekday, is_working, start_time, end_time}]}` — creates an approval request, does not apply immediately |
| POST | `/barbers/me/schedule/exceptions` | body: `{date, is_working, start_time, end_time, reason}` — applies immediately, no approval needed |
| GET | `/barbers/me/services` | own services, any status |
| PUT | `/barbers/me/services/:id` | body: partial `{name, description, price_dzd, duration_minutes}` — creates an approval request |
| GET | `/barbers/me/reservations?from=&to=` | own bookings (dates `YYYY-MM-DD`) |
| PATCH | `/barbers/me/reservations/:id/complete` | mark a booking completed |
| PATCH | `/barbers/me/reservations/:id/no-show` | mark a booking no-show |
| GET | `/barbers/me/approval-requests` | own proposals + status |

### Owner (`/shops/:shopId/...`, role=owner, must own that shop)
| Method | Path | Notes |
|---|---|---|
| PUT | `/shops/:shopId` | body: partial `{name, address, city, phone}` |
| PUT | `/shops/:shopId/hours` | body: `{hours: [{weekday, is_closed, open_time, close_time}]}` (7 entries, weekday 0=Sunday) |
| POST | `/shops/:shopId/barbers` | body: `{full_name, email, phone, bio}` → returns barber + `temp_password` |
| PATCH | `/shops/:shopId/barbers/:barberId/status` | body: `{is_active}` |
| POST | `/shops/:shopId/services` | body: `{name, description, price_dzd, duration_minutes, barber_id?}` |
| GET | `/shops/:shopId/approval-requests` | pending proposals from this shop's barbers |
| PATCH | `/shops/:shopId/approval-requests/:requestId/approve` | applies the payload |
| PATCH | `/shops/:shopId/approval-requests/:requestId/reject` | discards the payload |
| GET | `/shops/:shopId/reservations?barberId=&status=` | all bookings shop-wide |

## 4. Example curl session

```bash
BASE=http://localhost:8080/api/v1

# owner login
OWNER_TOKEN=$(curl -s -X POST $BASE/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"<owner-email>","password":"<owner-password>"}' | jq -r .token)

SHOP_ID=<from make seed-owner output>

# set hours (Mon-Fri 9-18, weekend closed)
curl -s -X PUT $BASE/shops/$SHOP_ID/hours -H "Authorization: Bearer $OWNER_TOKEN" \
  -H 'Content-Type: application/json' -d '{"hours":[
    {"weekday":0,"is_closed":true},
    {"weekday":1,"is_closed":false,"open_time":"09:00","close_time":"18:00"},
    {"weekday":2,"is_closed":false,"open_time":"09:00","close_time":"18:00"},
    {"weekday":3,"is_closed":false,"open_time":"09:00","close_time":"18:00"},
    {"weekday":4,"is_closed":false,"open_time":"09:00","close_time":"18:00"},
    {"weekday":5,"is_closed":false,"open_time":"09:00","close_time":"18:00"},
    {"weekday":6,"is_closed":true}
  ]}'

# create a barber
curl -s -X POST $BASE/shops/$SHOP_ID/barbers -H "Authorization: Bearer $OWNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"full_name":"Karim","email":"karim@example.com","phone":"0555000003","bio":"10 years experience"}'
# → note the returned barber.id and temp_password

BARBER_TOKEN=$(curl -s -X POST $BASE/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"karim@example.com","password":"<temp_password>"}' | jq -r .token)

# barber proposes a schedule
curl -s -X PUT $BASE/barbers/me/schedule -H "Authorization: Bearer $BARBER_TOKEN" \
  -H 'Content-Type: application/json' -d '{"days":[
    {"weekday":1,"is_working":true,"start_time":"09:00","end_time":"17:00"},
    {"weekday":2,"is_working":true,"start_time":"09:00","end_time":"17:00"}
  ]}'
# → note the returned approval request "id"

# owner approves it
curl -s -X PATCH $BASE/shops/$SHOP_ID/approval-requests/<request_id>/approve \
  -H "Authorization: Bearer $OWNER_TOKEN"

# owner creates a service
curl -s -X POST $BASE/shops/$SHOP_ID/services -H "Authorization: Bearer $OWNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Haircut","description":"Classic cut","price_dzd":500,"duration_minutes":30}'
# → note the returned service "id"

# check availability (public)
curl -s "$BASE/shops/$SHOP_ID/barbers/<barber_id>/availability?serviceId=<service_id>&date=2026-07-13"

# register + login a customer
curl -s -X POST $BASE/auth/register -H 'Content-Type: application/json' \
  -d '{"full_name":"Sofiane","email":"sofiane@example.com","phone":"0555000004","password":"password123"}'
CUST_TOKEN=$(curl -s -X POST $BASE/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"sofiane@example.com","password":"password123"}' | jq -r .token)

# book a slot returned by the availability call above
curl -s -X POST $BASE/reservations -H "Authorization: Bearer $CUST_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"barber_id":"<barber_id>","service_id":"<service_id>","starts_at":"2026-07-13T09:00:00+01:00"}'
# → note the returned reservation "id"

# barber completes it, customer rates it
curl -s -X PATCH $BASE/barbers/me/reservations/<reservation_id>/complete -H "Authorization: Bearer $BARBER_TOKEN"
curl -s -X POST $BASE/reservations/<reservation_id>/rating -H "Authorization: Bearer $CUST_TOKEN" \
  -H 'Content-Type: application/json' -d '{"score":5,"comment":"Great cut"}'
```

## 5. Known gaps (found in review, not yet fixed)

- `POST /reservations` doesn't verify the given `barber_id` actually offers
  `service_id` (they can currently be mismatched across barbers/shops).
- `POST /reservations` doesn't reject a `starts_at` in the past.
- `GET /barbers/me/reservations?from=&to=` parses those dates in UTC while
  the availability endpoint uses `Africa/Algiers` — can shift results near
  midnight.
