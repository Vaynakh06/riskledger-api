package store

import (
	"context"
	"database/sql"

	"riskledger/internal/domain"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) CreateUser(user domain.User) error {
	_, err := s.db.ExecContext(context.Background(), `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ($1,$2,$3,$4,$5)`, user.ID, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt)
	return err
}

func (s *PostgresStore) UserByEmail(email string) (domain.User, bool) {
	var user domain.User
	err := s.db.QueryRowContext(context.Background(), `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email=$1`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	return user, err == nil
}

func (s *PostgresStore) UserByID(id string) (domain.User, bool) {
	var user domain.User
	err := s.db.QueryRowContext(context.Background(), `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id=$1`, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	return user, err == nil
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
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO trades (id, portfolio_id, instrument, side, quantity, price, fee, executed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, trade.ID, trade.PortfolioID, trade.Instrument, trade.Side, trade.Quantity, trade.Price, trade.Fee, trade.ExecutedAt)
}

func (s *PostgresStore) TradesForPortfolio(portfolioID string) []domain.Trade {
	rows, err := s.db.QueryContext(context.Background(), `SELECT id, portfolio_id, instrument, side, quantity, price, fee, executed_at FROM trades WHERE portfolio_id=$1 ORDER BY executed_at ASC`, portfolioID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	trades := make([]domain.Trade, 0)
	for rows.Next() {
		var trade domain.Trade
		if rows.Scan(&trade.ID, &trade.PortfolioID, &trade.Instrument, &trade.Side, &trade.Quantity, &trade.Price, &trade.Fee, &trade.ExecutedAt) == nil {
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

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}
