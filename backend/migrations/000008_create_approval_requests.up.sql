CREATE TABLE approval_requests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id       UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    barber_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type   approval_target NOT NULL,
    target_id     UUID NOT NULL,
    payload       JSONB NOT NULL,
    status        approval_status NOT NULL DEFAULT 'pending',
    reviewed_by   UUID REFERENCES users(id),
    reviewed_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_approval_requests_shop_status ON approval_requests(shop_id, status);
