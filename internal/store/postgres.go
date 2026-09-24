package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"riskledger/internal/domain"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) CreateUser(user domain.User) error {
	_, err := s.db.ExecContext(context.Background(), `INSERT INTO users (id, email, password_hash, created_at, updated_at, role, two_factor_enabled, two_factor_secret) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, user.ID, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt, user.Role, user.TwoFactorEnabled, user.TwoFactorSecret)
	return err
}

func (s *PostgresStore) UserByEmail(email string) (domain.User, bool) {
	var user domain.User
	err := s.db.QueryRowContext(context.Background(), `SELECT id, email, password_hash, created_at, updated_at, role, two_factor_enabled, two_factor_secret FROM users WHERE email=$1`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt, &user.Role, &user.TwoFactorEnabled, &user.TwoFactorSecret)
	return user, err == nil
}

func (s *PostgresStore) UserByID(id string) (domain.User, bool) {
	var user domain.User
	err := s.db.QueryRowContext(context.Background(), `SELECT id, email, password_hash, created_at, updated_at, role, two_factor_enabled, two_factor_secret FROM users WHERE id=$1`, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt, &user.Role, &user.TwoFactorEnabled, &user.TwoFactorSecret)
	return user, err == nil
}

func (s *PostgresStore) UpdateUser(user domain.User) error {
	_, err := s.db.ExecContext(context.Background(), `UPDATE users SET role=$1, two_factor_enabled=$2, two_factor_secret=$3, updated_at=$4 WHERE id=$5`, user.Role, user.TwoFactorEnabled, user.TwoFactorSecret, user.UpdatedAt, user.ID)
	return err
}

func (s *PostgresStore) CreatePortfolio(portfolio domain.Portfolio) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO portfolios (id, user_id, name, base_currency, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6)`, portfolio.ID, portfolio.UserID, portfolio.Name, portfolio.BaseCurrency, portfolio.CreatedAt, portfolio.UpdatedAt)
}

func (s *PostgresStore) PortfolioByID(id string) (domain.Portfolio, bool) {
	var portfolio domain.Portfolio
	err := s.db.QueryRowContext(context.Background(), `SELECT id, user_id, name, base_currency, created_at, updated_at FROM portfolios WHERE id=$1`, id).Scan(&portfolio.ID, &portfolio.UserID, &portfolio.Name, &portfolio.BaseCurrency, &portfolio.CreatedAt, &portfolio.UpdatedAt)
	return portfolio, err == nil
}

func (s *PostgresStore) PortfoliosForUser(userID string) []domain.Portfolio {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, user_id, name, base_currency, created_at, updated_at FROM portfolios WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	portfolios := make([]domain.Portfolio, 0)
	for rows.Next() {
		var portfolio domain.Portfolio
		if rows.Scan(&portfolio.ID, &portfolio.UserID, &portfolio.Name, &portfolio.BaseCurrency, &portfolio.CreatedAt, &portfolio.UpdatedAt) == nil {
			portfolios = append(portfolios, portfolio)
		}
	}
	return portfolios
}

func (s *PostgresStore) AddTrade(trade domain.Trade) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO trades (id, portfolio_id, instrument, side, quantity, price, fee, strategy, tags, notes, checklist_ok, executed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, trade.ID, trade.PortfolioID, trade.Instrument, trade.Side, trade.Quantity, trade.Price, trade.Fee, trade.Strategy, trade.Tags, trade.Notes, trade.ChecklistOK, trade.ExecutedAt)
}

func (s *PostgresStore) AddLedgerEntry(entry domain.LedgerEntry) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO ledger_entries (id, portfolio_id, type, instrument, side, quantity, price, amount, currency, reason, notes, created_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, entry.ID, entry.PortfolioID, entry.Type, entry.Instrument, entry.Side, entry.Quantity, entry.Price, entry.Amount, entry.Currency, entry.Reason, entry.Notes, entry.CreatedBy, entry.CreatedAt)
}

func (s *PostgresStore) LedgerEntriesForPortfolio(portfolioID string) []domain.LedgerEntry {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, portfolio_id, type, instrument, side, quantity, price, amount, currency, reason, notes, created_by, created_at FROM ledger_entries WHERE portfolio_id=$1 ORDER BY created_at DESC`, portfolioID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	entries := make([]domain.LedgerEntry, 0)
	for rows.Next() {
		var entry domain.LedgerEntry
		if rows.Scan(&entry.ID, &entry.PortfolioID, &entry.Type, &entry.Instrument, &entry.Side, &entry.Quantity, &entry.Price, &entry.Amount, &entry.Currency, &entry.Reason, &entry.Notes, &entry.CreatedBy, &entry.CreatedAt) == nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (s *PostgresStore) TradesForPortfolio(portfolioID string) []domain.Trade {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, portfolio_id, instrument, side, quantity, price, fee, strategy, tags, notes, checklist_ok, executed_at FROM trades WHERE portfolio_id=$1 ORDER BY executed_at ASC`, portfolioID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	trades := make([]domain.Trade, 0)
	for rows.Next() {
		var trade domain.Trade
		if rows.Scan(&trade.ID, &trade.PortfolioID, &trade.Instrument, &trade.Side, &trade.Quantity, &trade.Price, &trade.Fee, &trade.Strategy, &trade.Tags, &trade.Notes, &trade.ChecklistOK, &trade.ExecutedAt) == nil {
			trades = append(trades, trade)
		}
	}
	return trades
}

func (s *PostgresStore) AddRiskLimit(portfolioID string, limit domain.RiskLimit) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO risk_limits (id, portfolio_id, type, value, enabled) VALUES ($1,$2,$3,$4,$5)`, limit.ID, portfolioID, limit.Type, limit.Value, limit.Enabled)
}

func (s *PostgresStore) RiskLimitsForPortfolio(portfolioID string) []domain.RiskLimit {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, type, value, enabled FROM risk_limits WHERE portfolio_id=$1 ORDER BY created_at DESC`, portfolioID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	limits := make([]domain.RiskLimit, 0)
	for rows.Next() {
		var limit domain.RiskLimit
		if rows.Scan(&limit.ID, &limit.Type, &limit.Value, &limit.Enabled) == nil {
			limits = append(limits, limit)
		}
	}
	return limits
}

func (s *PostgresStore) AddRiskEvent(portfolioID string, event domain.RiskEvent) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO risk_events (id, portfolio_id, type, current_value, allowed_value, status, occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, event.ID, portfolioID, event.Type, event.CurrentValue, event.AllowedValue, event.Status, event.OccurredAt)
}

func (s *PostgresStore) RiskEventsForPortfolio(portfolioID string) []domain.RiskEvent {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, type, current_value, allowed_value, status, occurred_at FROM risk_events WHERE portfolio_id=$1 ORDER BY occurred_at DESC`, portfolioID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	events := make([]domain.RiskEvent, 0)
	for rows.Next() {
		var event domain.RiskEvent
		if rows.Scan(&event.ID, &event.Type, &event.CurrentValue, &event.AllowedValue, &event.Status, &event.OccurredAt) == nil {
			events = append(events, event)
		}
	}
	return events
}

func (s *PostgresStore) AddAlertRule(rule domain.AlertRule) {
	channels, _ := json.Marshal(rule.Channels)
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO alert_rules (id, portfolio_id, type, threshold, severity, channels, enabled, created_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, rule.ID, rule.PortfolioID, rule.Type, rule.Threshold, rule.Severity, channels, rule.Enabled, rule.CreatedBy, rule.CreatedAt)
}

func (s *PostgresStore) AlertRulesForPortfolio(portfolioID string) []domain.AlertRule {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, portfolio_id, type, threshold, severity, channels, enabled, created_by, created_at FROM alert_rules WHERE portfolio_id=$1 ORDER BY created_at DESC`, portfolioID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	rules := make([]domain.AlertRule, 0)
	for rows.Next() {
		var rule domain.AlertRule
		var channels []byte
		if rows.Scan(&rule.ID, &rule.PortfolioID, &rule.Type, &rule.Threshold, &rule.Severity, &channels, &rule.Enabled, &rule.CreatedBy, &rule.CreatedAt) == nil {
			_ = json.Unmarshal(channels, &rule.Channels)
			rules = append(rules, rule)
		}
	}
	return rules
}

func (s *PostgresStore) AddAlertEvent(event domain.AlertEvent) {
	channels, _ := json.Marshal(event.Channels)
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO alert_events (id, portfolio_id, rule_id, type, severity, status, message, channels, triggered_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, event.ID, event.PortfolioID, event.RuleID, event.Type, event.Severity, event.Status, event.Message, channels, event.TriggeredAt)
}
func (s *PostgresStore) AlertEventByID(id string) (domain.AlertEvent, bool) {
	var event domain.AlertEvent
	var channels []byte
	err := s.db.QueryRowContext(context.Background(), `SELECT id, portfolio_id, rule_id, type, severity, status, message, channels, triggered_at FROM alert_events WHERE id=$1`, id).Scan(&event.ID, &event.PortfolioID, &event.RuleID, &event.Type, &event.Severity, &event.Status, &event.Message, &channels, &event.TriggeredAt)
	if err != nil {
		return event, false
	}
	_ = json.Unmarshal(channels, &event.Channels)
	return event, true
}

func (s *PostgresStore) AlertEventsForPortfolio(portfolioID string) []domain.AlertEvent {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, portfolio_id, rule_id, type, severity, status, message, channels, triggered_at FROM alert_events WHERE portfolio_id=$1 ORDER BY triggered_at DESC`, portfolioID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	events := make([]domain.AlertEvent, 0)
	for rows.Next() {
		var event domain.AlertEvent
		var channels []byte
		if rows.Scan(&event.ID, &event.PortfolioID, &event.RuleID, &event.Type, &event.Severity, &event.Status, &event.Message, &channels, &event.TriggeredAt) == nil {
			_ = json.Unmarshal(channels, &event.Channels)
			events = append(events, event)
		}
	}
	return events
}

func (s *PostgresStore) AddAuditEntry(entry domain.AuditEntry) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO audit_entries (id, portfolio_id, actor, action, entity_type, entity_id, reason, before_state, after_state, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, entry.ID, entry.PortfolioID, entry.Actor, entry.Action, entry.EntityType, entry.EntityID, entry.Reason, entry.Before, entry.After, entry.CreatedAt)
}

func (s *PostgresStore) AuditEntriesForPortfolio(portfolioID string) []domain.AuditEntry {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, portfolio_id, actor, action, entity_type, entity_id, reason, before_state, after_state, created_at FROM audit_entries WHERE portfolio_id=$1 ORDER BY created_at DESC`, portfolioID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	entries := make([]domain.AuditEntry, 0)
	for rows.Next() {
		var entry domain.AuditEntry
		if rows.Scan(&entry.ID, &entry.PortfolioID, &entry.Actor, &entry.Action, &entry.EntityType, &entry.EntityID, &entry.Reason, &entry.Before, &entry.After, &entry.CreatedAt) == nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (s *PostgresStore) AddCashEntry(entry domain.CashEntry) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO cash_entries (id, portfolio_id, type, amount, currency, reason, occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, entry.ID, entry.PortfolioID, entry.Type, entry.Amount, entry.Currency, entry.Reason, entry.OccurredAt)
}
func (s *PostgresStore) CashEntriesForPortfolio(id string) []domain.CashEntry {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, portfolio_id, type, amount, currency, reason, occurred_at FROM cash_entries WHERE portfolio_id=$1 ORDER BY occurred_at`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.CashEntry{}
	for rows.Next() {
		var item domain.CashEntry
		if rows.Scan(&item.ID, &item.PortfolioID, &item.Type, &item.Amount, &item.Currency, &item.Reason, &item.OccurredAt) == nil {
			out = append(out, item)
		}
	}
	return out
}
func (s *PostgresStore) AddFXRate(rate domain.FXRate) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO fx_rates (base_currency, quote_currency, rate, as_of, source) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (base_currency, quote_currency) DO UPDATE SET rate=EXCLUDED.rate, as_of=EXCLUDED.as_of, source=EXCLUDED.source`, rate.Base, rate.Quote, rate.Rate, rate.AsOf, rate.Source)
}
func (s *PostgresStore) FXRateForPair(base, quote string) (domain.FXRate, bool) {
	var item domain.FXRate
	err := s.db.QueryRowContext(context.Background(), `SELECT base_currency, quote_currency, rate, as_of, source FROM fx_rates WHERE base_currency=$1 AND quote_currency=$2`, base, quote).Scan(&item.Base, &item.Quote, &item.Rate, &item.AsOf, &item.Source)
	return item, err == nil
}
func (s *PostgresStore) AddBenchmark(item domain.Benchmark) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO benchmarks (id, portfolio_id, symbol, name, enabled) VALUES ($1,$2,$3,$4,$5)`, item.ID, item.PortfolioID, item.Symbol, item.Name, item.Enabled)
}
func (s *PostgresStore) BenchmarksForPortfolio(id string) []domain.Benchmark {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,portfolio_id,symbol,name,enabled FROM benchmarks WHERE portfolio_id=$1`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.Benchmark{}
	for rows.Next() {
		var item domain.Benchmark
		if rows.Scan(&item.ID, &item.PortfolioID, &item.Symbol, &item.Name, &item.Enabled) == nil {
			out = append(out, item)
		}
	}
	return out
}
func (s *PostgresStore) AddAllocation(item domain.Allocation) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO allocations (id, portfolio_id, instrument, category, target_weight, current_weight) VALUES ($1,$2,$3,$4,$5,$6)`, item.ID, item.PortfolioID, item.Instrument, item.Category, item.TargetWeight, item.CurrentWeight)
}
func (s *PostgresStore) AllocationsForPortfolio(id string) []domain.Allocation {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,portfolio_id,instrument,category,target_weight,current_weight FROM allocations WHERE portfolio_id=$1`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.Allocation{}
	for rows.Next() {
		var item domain.Allocation
		if rows.Scan(&item.ID, &item.PortfolioID, &item.Instrument, &item.Category, &item.TargetWeight, &item.CurrentWeight) == nil {
			out = append(out, item)
		}
	}
	return out
}
func (s *PostgresStore) AddBrokerConnection(item domain.BrokerConnection) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO broker_connections (id, portfolio_id, provider, status, last_sync_at, last_error) VALUES ($1,$2,$3,$4,$5,$6)`, item.ID, item.PortfolioID, item.Provider, item.Status, item.LastSyncAt, item.LastError)
}
func (s *PostgresStore) BrokerConnectionsForPortfolio(id string) []domain.BrokerConnection {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,portfolio_id,provider,status,last_sync_at,last_error FROM broker_connections WHERE portfolio_id=$1`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.BrokerConnection{}
	for rows.Next() {
		var item domain.BrokerConnection
		if rows.Scan(&item.ID, &item.PortfolioID, &item.Provider, &item.Status, &item.LastSyncAt, &item.LastError) == nil {
			out = append(out, item)
		}
	}
	return out
}
func (s *PostgresStore) AddDeliveryJob(item domain.DeliveryJob) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO delivery_jobs (id, alert_event_id, channel, status, attempts, next_attempt_at, last_error) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (id) DO UPDATE SET status=EXCLUDED.status, attempts=EXCLUDED.attempts, next_attempt_at=EXCLUDED.next_attempt_at, last_error=EXCLUDED.last_error`, item.ID, item.AlertEventID, item.Channel, item.Status, item.Attempts, item.NextAttemptAt, item.LastError)
}
func (s *PostgresStore) UpdateDeliveryJob(item domain.DeliveryJob) {
	s.AddDeliveryJob(item)
}
func (s *PostgresStore) DeliveryJobs() []domain.DeliveryJob {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id,alert_event_id,channel,status,attempts,next_attempt_at,last_error FROM delivery_jobs WHERE status IN ('pending','failed') ORDER BY next_attempt_at`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []domain.DeliveryJob{}
	for rows.Next() {
		var item domain.DeliveryJob
		if rows.Scan(&item.ID, &item.AlertEventID, &item.Channel, &item.Status, &item.Attempts, &item.NextAttemptAt, &item.LastError) == nil {
			out = append(out, item)
		}
	}
	return out
}

func (s *PostgresStore) AddImportBatch(batch domain.ImportBatch) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO import_batches (id, portfolio_id, source, filename, created_at, total, matched, unmatched, needs_review) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (id) DO UPDATE SET total=EXCLUDED.total, matched=EXCLUDED.matched, unmatched=EXCLUDED.unmatched, needs_review=EXCLUDED.needs_review`, batch.ID, batch.PortfolioID, batch.Source, batch.Filename, batch.CreatedAt, batch.Total, batch.Matched, batch.Unmatched, batch.Review)
}

func (s *PostgresStore) AddImportRecord(record domain.ImportRecord) {
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO import_records (id, batch_id, source, external_id, kind, status, instrument, side, quantity, price, fee, amount, currency, executed_at, reason, raw, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, record.ID, record.BatchID, record.Source, record.ExternalID, record.Kind, record.Status, record.Instrument, record.Side, record.Quantity, record.Price, record.Fee, record.Amount, record.Currency, nullableTime(record.ExecutedAt), record.Reason, record.Raw, record.CreatedAt)
}

func (s *PostgresStore) ImportBatchesForPortfolio(portfolioID string) []domain.ImportBatch {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, portfolio_id, source, filename, created_at, total, matched, unmatched, needs_review FROM import_batches WHERE portfolio_id=$1 ORDER BY created_at DESC`, portfolioID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]domain.ImportBatch, 0)
	for rows.Next() {
		var batch domain.ImportBatch
		if rows.Scan(&batch.ID, &batch.PortfolioID, &batch.Source, &batch.Filename, &batch.CreatedAt, &batch.Total, &batch.Matched, &batch.Unmatched, &batch.Review) == nil {
			result = append(result, batch)
		}
	}
	return result
}

func (s *PostgresStore) ImportBatchByID(batchID string) (domain.ImportBatch, bool) {
	var batch domain.ImportBatch
	err := s.db.QueryRowContext(context.Background(), `SELECT id, portfolio_id, source, filename, created_at, total, matched, unmatched, needs_review FROM import_batches WHERE id=$1`, batchID).Scan(&batch.ID, &batch.PortfolioID, &batch.Source, &batch.Filename, &batch.CreatedAt, &batch.Total, &batch.Matched, &batch.Unmatched, &batch.Review)
	return batch, err == nil
}

func (s *PostgresStore) ImportRecordExists(portfolioID, source, externalID string) bool {
	var exists bool
	err := s.db.QueryRowContext(context.Background(), `SELECT EXISTS (SELECT 1 FROM import_records r JOIN import_batches b ON b.id=r.batch_id WHERE b.portfolio_id=$1 AND r.source=$2 AND r.external_id=$3)`, portfolioID, source, externalID).Scan(&exists)
	return err == nil && exists
}

func (s *PostgresStore) ImportRecordsForBatch(batchID string) []domain.ImportRecord {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, batch_id, source, external_id, kind, status, instrument, side, quantity, price, fee, amount, currency, executed_at, reason, raw, created_at FROM import_records WHERE batch_id=$1 ORDER BY created_at ASC`, batchID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]domain.ImportRecord, 0)
	for rows.Next() {
		var record domain.ImportRecord
		if rows.Scan(&record.ID, &record.BatchID, &record.Source, &record.ExternalID, &record.Kind, &record.Status, &record.Instrument, &record.Side, &record.Quantity, &record.Price, &record.Fee, &record.Amount, &record.Currency, &record.ExecutedAt, &record.Reason, &record.Raw, &record.CreatedAt) == nil {
			result = append(result, record)
		}
	}
	return result
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
