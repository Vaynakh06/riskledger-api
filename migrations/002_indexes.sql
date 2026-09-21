CREATE INDEX IF NOT EXISTS idx_portfolios_user_id ON portfolios(user_id);
CREATE INDEX IF NOT EXISTS idx_trades_portfolio_executed_at ON trades(portfolio_id, executed_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_limits_portfolio_enabled ON risk_limits(portfolio_id, enabled);
CREATE INDEX IF NOT EXISTS idx_risk_events_portfolio_status ON risk_events(portfolio_id, status);
