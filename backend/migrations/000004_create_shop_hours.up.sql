CREATE TABLE shop_hours (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id    UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    weekday    SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    is_closed  BOOLEAN NOT NULL DEFAULT false,
    open_time  TIME,
    close_time TIME,
    UNIQUE (shop_id, weekday)
);
