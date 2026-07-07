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

    EXCLUDE USING gist (
        barber_id WITH =,
        tstzrange(starts_at, ends_at) WITH &&
    ) WHERE (status NOT IN ('cancelled', 'no_show'))
);

CREATE INDEX idx_reservations_barber_time ON reservations(barber_id, starts_at);
CREATE INDEX idx_reservations_customer ON reservations(customer_id);
CREATE INDEX idx_reservations_shop_time ON reservations(shop_id, starts_at);
