CREATE TABLE IF NOT EXISTS import_batches (
    id UUID PRIMARY KEY,
    portfolio_id UUID NOT NULL,
    source TEXT NOT NULL,
    filename TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    total INTEGER NOT NULL DEFAULT 0,
    matched INTEGER NOT NULL DEFAULT 0,
    unmatched INTEGER NOT NULL DEFAULT 0,
    needs_review INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS import_records (
    id UUID PRIMARY KEY,
    batch_id UUID NOT NULL REFERENCES import_batches(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    instrument TEXT NOT NULL DEFAULT '',
    side TEXT NOT NULL DEFAULT '',
    quantity NUMERIC(24,10) NOT NULL DEFAULT 0,
    price NUMERIC(24,10) NOT NULL DEFAULT 0,
    fee NUMERIC(24,10) NOT NULL DEFAULT 0,
    amount NUMERIC(24,10) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    executed_at TIMESTAMPTZ,
    reason TEXT NOT NULL DEFAULT '',
    raw TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_import_batches_portfolio_created_at ON import_batches(portfolio_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_import_records_batch_status ON import_records(batch_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_import_records_external_id ON import_records(source, external_id) WHERE external_id <> '';