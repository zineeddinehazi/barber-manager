CREATE TABLE schedule_exceptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    barber_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date        DATE NOT NULL,
    is_working  BOOLEAN NOT NULL DEFAULT false,
    start_time  TIME,
    end_time    TIME,
    reason      TEXT NOT NULL DEFAULT '',
    UNIQUE (barber_id, date)
);
