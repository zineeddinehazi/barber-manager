CREATE TABLE work_schedules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    barber_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weekday     SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    is_working  BOOLEAN NOT NULL DEFAULT true,
    start_time  TIME,
    end_time    TIME,
    status      approval_status NOT NULL DEFAULT 'approved',
    UNIQUE (barber_id, weekday, status)
);
