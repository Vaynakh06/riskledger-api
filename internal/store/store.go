package store

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"riskledger/internal/domain"
)

type InMemoryStore struct {
	mu                sync.RWMutex
	users             map[string]domain.User
	portfolios        map[string]domain.Portfolio
	trades            map[string][]domain.Trade
	ledgerEntries     map[string][]domain.LedgerEntry
	riskLimits        map[string][]domain.RiskLimit
	riskEvents        map[string][]domain.RiskEvent
	importBatches     map[string]domain.ImportBatch
	importRecords     map[string][]domain.ImportRecord
	alertRules        map[string][]domain.AlertRule
	alertEvents       map[string][]domain.AlertEvent
	auditEntries      map[string][]domain.AuditEntry
	cashEntries       map[string][]domain.CashEntry
	fxRates           map[string]domain.FXRate
	benchmarks        map[string][]domain.Benchmark
	allocations       map[string][]domain.Allocation
	brokerConnections map[string][]domain.BrokerConnection
	deliveryJobs      map[string]domain.DeliveryJob
	userByEmail       map[string]string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		users:             make(map[string]domain.User),
		portfolios:        make(map[string]domain.Portfolio),
		trades:            make(map[string][]domain.Trade),
		ledgerEntries:     make(map[string][]domain.LedgerEntry),
		riskLimits:        make(map[string][]domain.RiskLimit),
		riskEvents:        make(map[string][]domain.RiskEvent),
		importBatches:     make(map[string]domain.ImportBatch),
		importRecords:     make(map[string][]domain.ImportRecord),
		alertRules:        make(map[string][]domain.AlertRule),
		alertEvents:       make(map[string][]domain.AlertEvent),
		auditEntries:      make(map[string][]domain.AuditEntry),
		cashEntries:       make(map[string][]domain.CashEntry),
		fxRates:           make(map[string]domain.FXRate),
		benchmarks:        make(map[string][]domain.Benchmark),
		allocations:       make(map[string][]domain.Allocation),
		brokerConnections: make(map[string][]domain.BrokerConnection),
		deliveryJobs:      make(map[string]domain.DeliveryJob),
		userByEmail:       make(map[string]string),
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

func (s *InMemoryStore) UpdateUser(user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[user.ID]; !ok {
		return fmt.Errorf("user not found")
	}
	s.users[user.ID] = user
	return nil
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

func (s *InMemoryStore) AddLedgerEntry(entry domain.LedgerEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ledgerEntries[entry.PortfolioID] = append(s.ledgerEntries[entry.PortfolioID], entry)
}

func (s *InMemoryStore) LedgerEntriesForPortfolio(portfolioID string) []domain.LedgerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]domain.LedgerEntry, len(s.ledgerEntries[portfolioID]))
	copy(entries, s.ledgerEntries[portfolioID])
	return entries
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

func (s *InMemoryStore) AddImportBatch(batch domain.ImportBatch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.importBatches[batch.ID] = batch
}

func (s *InMemoryStore) AddImportRecord(record domain.ImportRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.importRecords[record.BatchID] = append(s.importRecords[record.BatchID], record)
}

func (s *InMemoryStore) ImportBatchesForPortfolio(portfolioID string) []domain.ImportBatch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.ImportBatch, 0)
	for _, batch := range s.importBatches {
		if batch.PortfolioID == portfolioID {
			batch.Records = nil
			out = append(out, batch)
		}
	}
	return out
}

func (s *InMemoryStore) ImportBatchByID(batchID string) (domain.ImportBatch, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	batch, ok := s.importBatches[batchID]
	return batch, ok
}

func (s *InMemoryStore) ImportRecordExists(portfolioID, source, externalID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for batchID, batch := range s.importBatches {
		if batch.PortfolioID != portfolioID || batch.Source != source {
			continue
		}
		for _, record := range s.importRecords[batchID] {
			if record.ExternalID == externalID {
				return true
			}
		}
	}
	return false
}

func (s *InMemoryStore) ImportRecordsForBatch(batchID string) []domain.ImportRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]domain.ImportRecord, len(s.importRecords[batchID]))
	copy(records, s.importRecords[batchID])
	return records
}

func (s *InMemoryStore) AddAlertRule(rule domain.AlertRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertRules[rule.PortfolioID] = append(s.alertRules[rule.PortfolioID], rule)
}

func (s *InMemoryStore) AlertRulesForPortfolio(portfolioID string) []domain.AlertRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rules := make([]domain.AlertRule, len(s.alertRules[portfolioID]))
	copy(rules, s.alertRules[portfolioID])
	return rules
}

func (s *InMemoryStore) AddAlertEvent(event domain.AlertEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertEvents[event.PortfolioID] = append(s.alertEvents[event.PortfolioID], event)
}
func (s *InMemoryStore) AlertEventByID(id string) (domain.AlertEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, events := range s.alertEvents {
		for _, event := range events {
			if event.ID == id {
				return event, true
			}
		}
	}
	return domain.AlertEvent{}, false
}

func (s *InMemoryStore) AlertEventsForPortfolio(portfolioID string) []domain.AlertEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]domain.AlertEvent, len(s.alertEvents[portfolioID]))
	copy(events, s.alertEvents[portfolioID])
	return events
}

func (s *InMemoryStore) AddAuditEntry(entry domain.AuditEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditEntries[entry.PortfolioID] = append(s.auditEntries[entry.PortfolioID], entry)
}

func (s *InMemoryStore) AuditEntriesForPortfolio(portfolioID string) []domain.AuditEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]domain.AuditEntry, len(s.auditEntries[portfolioID]))
	copy(entries, s.auditEntries[portfolioID])
	return entries
}

func (s *InMemoryStore) AddCashEntry(entry domain.CashEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cashEntries[entry.PortfolioID] = append(s.cashEntries[entry.PortfolioID], entry)
}
func (s *InMemoryStore) CashEntriesForPortfolio(id string) []domain.CashEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]domain.CashEntry(nil), s.cashEntries[id]...)
	return out
}
func (s *InMemoryStore) AddFXRate(rate domain.FXRate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fxRates[rate.Base+":"+rate.Quote] = rate
}
func (s *InMemoryStore) FXRateForPair(base, quote string) (domain.FXRate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rate, ok := s.fxRates[base+":"+quote]
	return rate, ok
}
func (s *InMemoryStore) AddBenchmark(item domain.Benchmark) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.benchmarks[item.PortfolioID] = append(s.benchmarks[item.PortfolioID], item)
}
func (s *InMemoryStore) BenchmarksForPortfolio(id string) []domain.Benchmark {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Benchmark(nil), s.benchmarks[id]...)
}
func (s *InMemoryStore) AddAllocation(item domain.Allocation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allocations[item.PortfolioID] = append(s.allocations[item.PortfolioID], item)
}
func (s *InMemoryStore) AllocationsForPortfolio(id string) []domain.Allocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Allocation(nil), s.allocations[id]...)
}
func (s *InMemoryStore) AddBrokerConnection(item domain.BrokerConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.brokerConnections[item.PortfolioID] = append(s.brokerConnections[item.PortfolioID], item)
}
func (s *InMemoryStore) BrokerConnectionsForPortfolio(id string) []domain.BrokerConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.BrokerConnection(nil), s.brokerConnections[id]...)
}
func (s *InMemoryStore) AddDeliveryJob(item domain.DeliveryJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveryJobs[item.ID] = item
}
func (s *InMemoryStore) UpdateDeliveryJob(item domain.DeliveryJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveryJobs[item.ID] = item
}
func (s *InMemoryStore) DeliveryJobs() []domain.DeliveryJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.DeliveryJob, 0, len(s.deliveryJobs))
	for _, item := range s.deliveryJobs {
		out = append(out, item)
	}
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
	s.ledgerEntries[portfolio.ID] = []domain.LedgerEntry{
		{ID: "l1", PortfolioID: portfolio.ID, Type: "trade", Instrument: "BTC", Side: "buy", Quantity: 1.5, Price: 60000, Amount: 90000, Currency: "USD", Reason: "trade recorded", Notes: "Initial BTC position", CreatedBy: "system", CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "l2", PortfolioID: portfolio.ID, Type: "trade", Instrument: "ETH", Side: "buy", Quantity: 6, Price: 3200, Amount: 19200, Currency: "USD", Reason: "trade recorded", Notes: "ETH allocation", CreatedBy: "system", CreatedAt: time.Now().Add(-30 * time.Minute)},
	}
}
