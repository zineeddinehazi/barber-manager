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
