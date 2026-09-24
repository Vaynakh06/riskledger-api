ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'owner';
ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_secret TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS cash_entries (id UUID PRIMARY KEY, portfolio_id UUID NOT NULL, type TEXT NOT NULL, amount NUMERIC(24,10) NOT NULL, currency TEXT NOT NULL, reason TEXT NOT NULL DEFAULT '', occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS fx_rates (base_currency TEXT NOT NULL, quote_currency TEXT NOT NULL, rate NUMERIC(24,12) NOT NULL, as_of TIMESTAMPTZ NOT NULL, source TEXT NOT NULL, PRIMARY KEY (base_currency, quote_currency));
CREATE TABLE IF NOT EXISTS benchmarks (id UUID PRIMARY KEY, portfolio_id UUID NOT NULL, symbol TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', enabled BOOLEAN NOT NULL DEFAULT TRUE);
CREATE TABLE IF NOT EXISTS allocations (id UUID PRIMARY KEY, portfolio_id UUID NOT NULL, instrument TEXT NOT NULL, category TEXT NOT NULL DEFAULT '', target_weight NUMERIC(8,5) NOT NULL DEFAULT 0, current_weight NUMERIC(8,5) NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS broker_connections (id UUID PRIMARY KEY, portfolio_id UUID NOT NULL, provider TEXT NOT NULL, status TEXT NOT NULL, last_sync_at TIMESTAMPTZ, last_error TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS delivery_jobs (id UUID PRIMARY KEY, alert_event_id UUID NOT NULL, channel TEXT NOT NULL, status TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), last_error TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS idx_cash_entries_portfolio_occurred ON cash_entries(portfolio_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_delivery_jobs_due ON delivery_jobs(status, next_attempt_at);