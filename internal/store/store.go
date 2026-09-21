package store

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"riskledger/internal/domain"
)

type InMemoryStore struct {
	mu          sync.RWMutex
	users       map[string]domain.User
	portfolios  map[string]domain.Portfolio
	trades      map[string][]domain.Trade
	riskLimits  map[string][]domain.RiskLimit
	riskEvents  map[string][]domain.RiskEvent
	userByEmail map[string]string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		users:       make(map[string]domain.User),
		portfolios:  make(map[string]domain.Portfolio),
		trades:      make(map[string][]domain.Trade),
		riskLimits:  make(map[string][]domain.RiskLimit),
		riskEvents:  make(map[string][]domain.RiskEvent),
		userByEmail: make(map[string]string),
	}
}

func (s *InMemoryStore) CreateUser(u domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.userByEmail[u.Email]; exists {
		return fmt.Errorf("email already registered")
	}
	s.users[u.ID] = u
	s.userByEmail[u.Email] = u.ID
	return nil
}

func (s *InMemoryStore) UserByEmail(email string) (domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.userByEmail[email]
	if !ok {
		return domain.User{}, false
	}
	u, ok := s.users[id]
	return u, ok
}

func (s *InMemoryStore) UserByID(id string) (domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	return u, ok
}

func (s *InMemoryStore) CreatePortfolio(p domain.Portfolio) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.portfolios[p.ID] = p
}

func (s *InMemoryStore) PortfolioByID(id string) (domain.Portfolio, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.portfolios[id]
	return p, ok
}

func (s *InMemoryStore) PortfoliosForUser(userID string) []domain.Portfolio {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Portfolio, 0)
	for _, p := range s.portfolios {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	return out
}

func (s *InMemoryStore) AddTrade(t domain.Trade) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trades[t.PortfolioID] = append(s.trades[t.PortfolioID], t)
}

func (s *InMemoryStore) TradesForPortfolio(portfolioID string) []domain.Trade {
	s.mu.RLock()
	defer s.mu.RUnlock()
	trades := make([]domain.Trade, len(s.trades[portfolioID]))
	copy(trades, s.trades[portfolioID])
	return trades
}

func (s *InMemoryStore) AddRiskLimit(portfolioID string, limit domain.RiskLimit) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.riskLimits[portfolioID] = append(s.riskLimits[portfolioID], limit)
}

func (s *InMemoryStore) RiskLimitsForPortfolio(portfolioID string) []domain.RiskLimit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.RiskLimit, len(s.riskLimits[portfolioID]))
	copy(out, s.riskLimits[portfolioID])
	return out
}

func (s *InMemoryStore) AddRiskEvent(portfolioID string, event domain.RiskEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.riskEvents[portfolioID] = append(s.riskEvents[portfolioID], event)
}

func (s *InMemoryStore) RiskEventsForPortfolio(portfolioID string) []domain.RiskEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.RiskEvent, len(s.riskEvents[portfolioID]))
	copy(out, s.riskEvents[portfolioID])
	return out
}

func (s *InMemoryStore) SeedDemoData() {
	userID := "user-demo"
	hash, _ := bcrypt.GenerateFromPassword([]byte("demo-password"), bcrypt.DefaultCost)
	user := domain.User{ID: userID, Email: "demo@riskledger.dev", PasswordHash: string(hash), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.users[userID] = user
	s.userByEmail[user.Email] = userID

	portfolio := domain.Portfolio{ID: "portfolio-demo", UserID: userID, Name: "Demo Portfolio", BaseCurrency: "USD", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.portfolios[portfolio.ID] = portfolio

	s.trades[portfolio.ID] = []domain.Trade{
		{ID: "t1", PortfolioID: portfolio.ID, Instrument: "BTC", Side: "buy", Quantity: 1.5, Price: 60000, Fee: 10, ExecutedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "t2", PortfolioID: portfolio.ID, Instrument: "BTC", Side: "buy", Quantity: 0.5, Price: 62000, Fee: 8, ExecutedAt: time.Now().Add(-1 * time.Hour)},
		{ID: "t3", PortfolioID: portfolio.ID, Instrument: "ETH", Side: "buy", Quantity: 6, Price: 3200, Fee: 12, ExecutedAt: time.Now().Add(-30 * time.Minute)},
	}

	s.riskLimits[portfolio.ID] = []domain.RiskLimit{{ID: "rl1", Type: "max_portfolio_value", Value: 200000, Enabled: true}}
}
