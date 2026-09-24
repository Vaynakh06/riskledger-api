CREATE TABLE IF NOT EXISTS alert_rules (
    id UUID PRIMARY KEY,
    portfolio_id UUID NOT NULL,
    type TEXT NOT NULL,
    threshold NUMERIC(24,10) NOT NULL DEFAULT 0,
    severity TEXT NOT NULL DEFAULT 'warning',
    channels JSONB NOT NULL DEFAULT '[]'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alert_events (
    id UUID PRIMARY KEY,
    portfolio_id UUID NOT NULL,
    rule_id UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT NOT NULL,
    channels JSONB NOT NULL DEFAULT '{}'::jsonb,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_entries (
    id UUID PRIMARY KEY,
    portfolio_id UUID NOT NULL,
    actor TEXT NOT NULL DEFAULT 'system',
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    before_state TEXT NOT NULL DEFAULT '',
    after_state TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_events_portfolio_triggered_at ON alert_events(portfolio_id, triggered_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_entries_portfolio_created_at ON audit_entries(portfolio_id, created_at DESC);