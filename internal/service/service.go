package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"riskledger/internal/domain"
)

type Service struct {
	mu    sync.RWMutex
	store Store
}

type Store interface {
	CreateUser(domain.User) error
	UserByEmail(string) (domain.User, bool)
	UserByID(string) (domain.User, bool)
	CreatePortfolio(domain.Portfolio)
	PortfolioByID(string) (domain.Portfolio, bool)
	PortfoliosForUser(string) []domain.Portfolio
	AddTrade(domain.Trade)
	TradesForPortfolio(string) []domain.Trade
	AddRiskLimit(string, domain.RiskLimit)
	RiskLimitsForPortfolio(string) []domain.RiskLimit
	AddRiskEvent(string, domain.RiskEvent)
	RiskEventsForPortfolio(string) []domain.RiskEvent
}

func NewService(s Store) *Service {
	return &Service{store: s}
}

func (s *Service) Register(email, password string) (domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || len(password) < 6 {
		return domain.User{}, errors.New("invalid email or password")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, errors.New("could not secure password")
	}
	user := domain.NewUser(email, string(hash))
	if err := s.store.CreateUser(user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Service) Login(email, password string) (domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	u, ok := s.store.UserByEmail(email)
	if !ok {
		return domain.User{}, errors.New("user not found")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return domain.User{}, errors.New("invalid credentials")
	}
	return u, nil
}

func (s *Service) CreatePortfolio(userID, name, currency string) (domain.Portfolio, error) {
	if strings.TrimSpace(name) == "" {
		return domain.Portfolio{}, errors.New("portfolio name is required")
	}
	p := domain.NewPortfolio(userID, name, currency)
	s.store.CreatePortfolio(p)
	return p, nil
}

func (s *Service) PortfoliosForUser(userID string) []domain.Portfolio {
	return s.store.PortfoliosForUser(userID)
}

func (s *Service) PortfolioByID(id string) (domain.Portfolio, bool) {
	return s.store.PortfolioByID(id)
}

func (s *Service) TradesForPortfolio(portfolioID string) []domain.Trade {
	return s.store.TradesForPortfolio(portfolioID)
}

func (s *Service) AddTrade(portfolioID, instrument, side string, quantity, price, fee float64, strategy, notes string, tags []string, checklistOK bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.store.PortfolioByID(portfolioID); !ok {
		return fmt.Errorf("portfolio not found")
	}
	if instrument == "" || !validSide(side) {
		return fmt.Errorf("invalid trade data")
	}
	if quantity <= 0 || price <= 0 || fee < 0 {
		return fmt.Errorf("invalid trade data")
	}

	trades := s.store.TradesForPortfolio(portfolioID)
	if side == "sell" {
		pos := calculateOpenPosition(trades, instrument)
		if pos < quantity {
			return fmt.Errorf("cannot sell more than open position")
		}
	}

	s.store.AddTrade(domain.NewTrade(portfolioID, instrument, side, quantity, price, fee, strategy, notes, tags, checklistOK))
	return nil
}

func (s *Service) Analytics(portfolioID string) map[string]any {
	trades := s.store.TradesForPortfolio(portfolioID)
	strategies := map[string]int{}
	tags := map[string]int{}
	checked := 0
	for _, trade := range trades {
		strategy := trade.Strategy
		if strategy == "" {
			strategy = "Без стратегии"
		}
		strategies[strategy]++
		if trade.ChecklistOK {
			checked++
		}
		for _, tag := range trade.Tags {
			tags[tag]++
		}
	}
	rate := 0.0
	if len(trades) > 0 {
		rate = float64(checked) / float64(len(trades)) * 100
	}
	return map[string]any{"portfolio_id": portfolioID, "trade_count": len(trades), "strategies": strategies, "tags": tags, "checklist_rate": rate}
}

func (s *Service) Positions(portfolioID string) []domain.Position {
	trades := s.store.TradesForPortfolio(portfolioID)
	byInstrument := map[string][]domain.Trade{}
	for _, trade := range trades {
		byInstrument[trade.Instrument] = append(byInstrument[trade.Instrument], trade)
	}

	positions := make([]domain.Position, 0, len(byInstrument))
	for symbol, items := range byInstrument {
		positions = append(positions, buildPosition(symbol, items))
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i].Instrument < positions[j].Instrument })
	return positions
}

func (s *Service) Summary(portfolioID string) map[string]any {
	positions := s.Positions(portfolioID)
	totalValue := 0.0
	unrealized := 0.0
	realized := 0.0
	for _, p := range positions {
		totalValue += p.MarketValue
		unrealized += p.UnrealizedPnL
		realized += p.RealizedPnL
	}

	riskEvents := s.store.RiskEventsForPortfolio(portfolioID)
	violations := 0
	for _, event := range riskEvents {
		if event.Status == "open" {
			violations++
		}
	}

	return map[string]any{
		"portfolio_id":       portfolioID,
		"total_value":        totalValue,
		"realized_pnl":       realized,
		"unrealized_pnl":     unrealized,
		"open_positions":     len(positions),
		"active_risk_events": violations,
	}
}

func (s *Service) AddRiskLimit(portfolioID, limitType string, value float64) (domain.RiskLimit, error) {
	if limitType == "" || value <= 0 {
		return domain.RiskLimit{}, errors.New("invalid risk limit")
	}
	limit := domain.NewRiskLimit(limitType, value)
	s.store.AddRiskLimit(portfolioID, limit)
	return limit, nil
}

func (s *Service) RiskLimits(portfolioID string) []domain.RiskLimit {
	return s.store.RiskLimitsForPortfolio(portfolioID)
}

func (s *Service) EvaluateRisk(portfolioID string) []domain.RiskEvent {
	summary := s.Summary(portfolioID)
	totalValue := summary["total_value"].(float64)
	limits := s.RiskLimits(portfolioID)
	events := make([]domain.RiskEvent, 0)
	for _, limit := range limits {
		if limit.Enabled && limit.Type == "max_portfolio_value" && totalValue > limit.Value {
			event := domain.NewRiskEvent(limit.Type, totalValue, limit.Value)
			s.store.AddRiskEvent(portfolioID, event)
			events = append(events, event)
		}
	}
	return events
}

func validSide(side string) bool {
	return side == "buy" || side == "sell"
}

func calculateOpenPosition(trades []domain.Trade, instrument string) float64 {
	q := 0.0
	for _, t := range trades {
		if t.Instrument != instrument {
			continue
		}
		if t.Side == "buy" {
			q += t.Quantity
		}
		if t.Side == "sell" {
			q -= t.Quantity
		}
	}
	if q < 0 {
		return 0
	}
	return q
}

func buildPosition(symbol string, trades []domain.Trade) domain.Position {
	var quantity, totalCost, realized, currentPrice float64
	currentPrice = 100
	sort.SliceStable(trades, func(i, j int) bool {
		return trades[i].ExecutedAt.Before(trades[j].ExecutedAt)
	})
	for _, t := range trades {
		if t.Side == "buy" {
			quantity += t.Quantity
			totalCost += t.Quantity*t.Price + t.Fee
			currentPrice = t.Price
		}
		if t.Side == "sell" {
			if quantity < t.Quantity {
				continue
			}
			avg := 0.0
			if quantity > 0 {
				avg = totalCost / quantity
			}
			realized += (t.Price-avg)*t.Quantity - t.Fee
			totalCost -= avg * t.Quantity
			quantity -= t.Quantity
			currentPrice = t.Price
		}
	}
	avg := 0.0
	if quantity > 0 {
		avg = totalCost / quantity
	}
	marketValue := quantity * currentPrice
	unrealized := (currentPrice - avg) * quantity
	return domain.Position{
		Instrument:        symbol,
		Quantity:          quantity,
		AverageEntryPrice: avg,
		MarketPrice:       currentPrice,
		MarketValue:       marketValue,
		RealizedPnL:       realized,
		UnrealizedPnL:     unrealized,
	}
}
