CREATE TABLE IF NOT EXISTS ledger_entries (
    id UUID PRIMARY KEY,
    portfolio_id UUID NOT NULL,
    type TEXT NOT NULL,
    instrument TEXT NOT NULL DEFAULT '',
    side TEXT NOT NULL DEFAULT '',
    quantity NUMERIC(24,10) NOT NULL DEFAULT 0,
    price NUMERIC(24,10) NOT NULL DEFAULT 0,
    amount NUMERIC(24,10) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    reason TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_portfolio_created_at ON ledger_entries(portfolio_id, created_at DESC);
