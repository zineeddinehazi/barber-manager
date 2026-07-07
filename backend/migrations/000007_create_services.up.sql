CREATE TABLE services (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id          UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    barber_id        UUID REFERENCES users(id) ON DELETE CASCADE,
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
