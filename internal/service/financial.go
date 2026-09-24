package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"riskledger/internal/domain"
)

type BrokerSyncProvider interface {
	Sync(portfolioID string) ([]domain.Trade, error)
}

type HTTPBrokerProvider struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewHTTPBrokerProvider(endpoint, token string) *HTTPBrokerProvider {
	return &HTTPBrokerProvider{endpoint: strings.TrimRight(endpoint, "/"), token: token, client: &http.Client{Timeout: 10 * time.Second}}
}
func (p *HTTPBrokerProvider) Sync(portfolioID string) ([]domain.Trade, error) {
	if p.endpoint == "" || p.token == "" {
		return nil, errors.New("broker API is not configured")
	}
	request, err := http.NewRequest(http.MethodGet, p.endpoint+"/portfolios/"+portfolioID+"/transactions", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+p.token)
	response, err := p.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("broker API status %d", response.StatusCode)
	}
	var trades []domain.Trade
	if err := json.NewDecoder(response.Body).Decode(&trades); err != nil {
		return nil, err
	}
	return trades, nil
}

func (s *Service) AddCash(portfolioID, entryType string, amount float64, currency, reason, actor string) (domain.CashEntry, error) {
	if amount <= 0 || (entryType != "deposit" && entryType != "withdrawal") {
		return domain.CashEntry{}, errors.New("invalid cash entry")
	}
	entry := domain.NewCashEntry(portfolioID, entryType, amount, strings.ToUpper(currency), reason)
	s.store.AddCashEntry(entry)
	s.recordAudit(portfolioID, actor, "create", "cash_entry", entry.ID, reason, "", entry)
	return entry, nil
}

func (s *Service) CashBalance(portfolioID string) float64 {
	balance := 0.0
	for _, entry := range s.store.CashEntriesForPortfolio(portfolioID) {
		if entry.Type == "withdrawal" {
			balance -= entry.Amount
		} else {
			balance += entry.Amount
		}
	}
	return balance
}
func (s *Service) CashEntries(portfolioID string) []domain.CashEntry {
	return s.store.CashEntriesForPortfolio(portfolioID)
}
func (s *Service) Benchmarks(portfolioID string) []domain.Benchmark {
	return s.store.BenchmarksForPortfolio(portfolioID)
}
func (s *Service) Allocations(portfolioID string) []domain.Allocation {
	return s.store.AllocationsForPortfolio(portfolioID)
}

func (s *Service) SetFXRate(base, quote string, rate float64) error {
	if rate <= 0 || base == "" || quote == "" {
		return errors.New("invalid FX rate")
	}
	s.store.AddFXRate(domain.FXRate{Base: strings.ToUpper(base), Quote: strings.ToUpper(quote), Rate: rate, AsOf: time.Now(), Source: "manual"})
	return nil
}

func (s *Service) AddBenchmark(portfolioID, symbol, name string, actor string) (domain.Benchmark, error) {
	if strings.TrimSpace(symbol) == "" {
		return domain.Benchmark{}, errors.New("benchmark symbol is required")
	}
	item := domain.NewBenchmark(portfolioID, strings.ToUpper(symbol), name)
	s.store.AddBenchmark(item)
	s.recordAudit(portfolioID, actor, "create", "benchmark", item.ID, "benchmark configured", "", item)
	return item, nil
}

func (s *Service) AddAllocation(portfolioID, instrument, category string, target float64, actor string) (domain.Allocation, error) {
	if instrument == "" || target < 0 || target > 100 {
		return domain.Allocation{}, errors.New("invalid allocation")
	}
	item := domain.NewAllocation(portfolioID, instrument, category, target)
	positions := s.Positions(portfolioID)
	total := 0.0
	for _, position := range positions {
		total += position.MarketValue
	}
	for _, position := range positions {
		if position.Instrument == instrument && total > 0 {
			item.CurrentWeight = position.MarketValue / total * 100
		}
	}
	s.store.AddAllocation(item)
	s.recordAudit(portfolioID, actor, "create", "allocation", item.ID, "allocation target configured", "", item)
	return item, nil
}

func (s *Service) PerformanceReport(portfolioID string) domain.PerformanceReport {
	positions := s.Positions(portfolioID)
	value, unrealized, realized := 0.0, 0.0, 0.0
	for _, position := range positions {
		value += position.MarketValue
		unrealized += position.UnrealizedPnL
		realized += position.RealizedPnL
	}
	cash := s.CashBalance(portfolioID)
	totalReturn := realized + unrealized
	invested := value - totalReturn
	twr := 0.0
	if invested > 0 {
		twr = totalReturn / invested * 100
	}
	irr := twr
	trades := s.TradesForPortfolio(portfolioID)
	if len(trades) > 0 && invested > 0 {
		years := time.Since(trades[0].ExecutedAt).Hours() / (24 * 365.25)
		if years > 0 {
			irr = (math.Pow((value+cash)/invested, 1/years) - 1) * 100
		}
	}
	report := domain.PerformanceReport{PortfolioID: portfolioID, TotalValue: value, CashValue: cash, TotalReturn: totalReturn, TWR: twr, IRR: irr, Allocations: s.store.AllocationsForPortfolio(portfolioID)}
	benchmarks := s.store.BenchmarksForPortfolio(portfolioID)
	if len(benchmarks) > 0 {
		report.BenchmarkSymbol = benchmarks[0].Symbol
	}
	return report
}

func (s *Service) SyncBroker(portfolioID string, provider BrokerSyncProvider, actor string) (int, error) {
	if provider == nil {
		return 0, errors.New("broker provider is not configured")
	}
	trades, err := provider.Sync(portfolioID)
	if err != nil {
		return 0, err
	}
	synced := 0
	for _, trade := range trades {
		if err := s.AddTradeWithMetaAt(portfolioID, trade.Instrument, trade.Side, trade.Quantity, trade.Price, trade.Fee, trade.Strategy, trade.Notes, trade.Tags, trade.ChecklistOK, actor, "broker API sync", trade.ExecutedAt); err == nil {
			synced++
		}
	}
	return synced, nil
}
