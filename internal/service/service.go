package service

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"riskledger/internal/domain"
)

type Service struct {
	mu            sync.RWMutex
	store         Store
	priceProvider PriceProvider
	notifier      AlertNotifier
}

type Store interface {
	CreateUser(domain.User) error
	UserByEmail(string) (domain.User, bool)
	UserByID(string) (domain.User, bool)
	UpdateUser(domain.User) error
	CreatePortfolio(domain.Portfolio)
	PortfolioByID(string) (domain.Portfolio, bool)
	PortfoliosForUser(string) []domain.Portfolio
	AddTrade(domain.Trade)
	TradesForPortfolio(string) []domain.Trade
	AddLedgerEntry(domain.LedgerEntry)
	LedgerEntriesForPortfolio(string) []domain.LedgerEntry
	AddRiskLimit(string, domain.RiskLimit)
	RiskLimitsForPortfolio(string) []domain.RiskLimit
	AddRiskEvent(string, domain.RiskEvent)
	RiskEventsForPortfolio(string) []domain.RiskEvent
	AddImportBatch(domain.ImportBatch)
	AddImportRecord(domain.ImportRecord)
	ImportBatchByID(string) (domain.ImportBatch, bool)
	ImportRecordExists(string, string, string) bool
	ImportBatchesForPortfolio(string) []domain.ImportBatch
	ImportRecordsForBatch(string) []domain.ImportRecord
	AddAlertRule(domain.AlertRule)
	AlertRulesForPortfolio(string) []domain.AlertRule
	AddAlertEvent(domain.AlertEvent)
	AlertEventByID(string) (domain.AlertEvent, bool)
	AlertEventsForPortfolio(string) []domain.AlertEvent
	AddAuditEntry(domain.AuditEntry)
	AuditEntriesForPortfolio(string) []domain.AuditEntry
	AddCashEntry(domain.CashEntry)
	CashEntriesForPortfolio(string) []domain.CashEntry
	AddFXRate(domain.FXRate)
	FXRateForPair(string, string) (domain.FXRate, bool)
	AddBenchmark(domain.Benchmark)
	BenchmarksForPortfolio(string) []domain.Benchmark
	AddAllocation(domain.Allocation)
	AllocationsForPortfolio(string) []domain.Allocation
	AddBrokerConnection(domain.BrokerConnection)
	BrokerConnectionsForPortfolio(string) []domain.BrokerConnection
	AddDeliveryJob(domain.DeliveryJob)
	UpdateDeliveryJob(domain.DeliveryJob)
	DeliveryJobs() []domain.DeliveryJob
}

func NewService(s Store) *Service {
	return &Service{store: s}
}

func NewServiceWithProviders(s Store, priceProvider PriceProvider, notifier AlertNotifier) *Service {
	return &Service{store: s, priceProvider: priceProvider, notifier: notifier}
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
	return s.AddTradeWithMeta(portfolioID, instrument, side, quantity, price, fee, strategy, notes, tags, checklistOK, "system", "trade recorded")
}

func (s *Service) AddTradeWithMeta(portfolioID, instrument, side string, quantity, price, fee float64, strategy, notes string, tags []string, checklistOK bool, createdBy, reason string) error {
	return s.addTradeWithMeta(portfolioID, instrument, side, quantity, price, fee, strategy, notes, tags, checklistOK, createdBy, reason, time.Now())
}

func (s *Service) AddTradeWithMetaAt(portfolioID, instrument, side string, quantity, price, fee float64, strategy, notes string, tags []string, checklistOK bool, createdBy, reason string, executedAt time.Time) error {
	return s.addTradeWithMeta(portfolioID, instrument, side, quantity, price, fee, strategy, notes, tags, checklistOK, createdBy, reason, executedAt)
}

func (s *Service) addTradeWithMeta(portfolioID, instrument, side string, quantity, price, fee float64, strategy, notes string, tags []string, checklistOK bool, createdBy, reason string, executedAt time.Time) error {
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

	trade := domain.NewTrade(portfolioID, instrument, side, quantity, price, fee, strategy, notes, tags, checklistOK)
	if !executedAt.IsZero() {
		trade.ExecutedAt = executedAt
	}
	s.store.AddTrade(trade)

	amount := quantity * price
	if side == "buy" {
		amount += fee
	} else {
		amount = -(amount - fee)
	}
	entry := domain.NewLedgerEntry(portfolioID, "trade", instrument, side, quantity, price, amount, "USD", reason, notes, createdBy)
	entry.Type = "trade"
	s.store.AddLedgerEntry(entry)
	s.recordAudit(portfolioID, createdBy, "create", "trade", trade.ID, reason, "", trade)
	return nil
}

func (s *Service) AddManualLedgerEntry(portfolioID, entryType, instrument, side string, quantity, price float64, reason, notes, createdBy string) (domain.LedgerEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.store.PortfolioByID(portfolioID); !ok {
		return domain.LedgerEntry{}, fmt.Errorf("portfolio not found")
	}
	if strings.TrimSpace(entryType) == "" {
		return domain.LedgerEntry{}, fmt.Errorf("entry type is required")
	}
	amount := quantity * price
	if side == "sell" {
		amount = -amount
	}
	if amount == 0 && price == 0 {
		amount = 0
	}
	entry := domain.NewLedgerEntry(portfolioID, entryType, instrument, side, quantity, price, amount, "USD", reason, notes, createdBy)
	s.store.AddLedgerEntry(entry)
	s.recordAudit(portfolioID, createdBy, "create", "ledger_entry", entry.ID, reason, "", entry)
	return entry, nil
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
		marketPrice := latestTradePrice(items)
		if s.priceProvider != nil {
			if livePrice, ok := s.priceProvider.CurrentPrice(symbol); ok {
				marketPrice = livePrice
			}
		}
		positions = append(positions, buildPosition(symbol, items, marketPrice))
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

func (s *Service) RiskSnapshot(portfolioID string) domain.RiskSnapshot {
	return s.riskSnapshotForPositions(portfolioID, s.Positions(portfolioID))
}

func (s *Service) Scenario(portfolioID string, request domain.ScenarioRequest) (domain.ScenarioImpact, error) {
	if request.Action == "" {
		return domain.ScenarioImpact{}, errors.New("scenario action is required")
	}
	if request.Quantity < 0 || request.Price < 0 || request.RiskPercent < 0 || request.RiskPercent > 100 {
		return domain.ScenarioImpact{}, errors.New("scenario values are invalid")
	}
	positions := s.Positions(portfolioID)
	before := s.riskSnapshotForPositions(portfolioID, positions)
	afterPositions := make([]domain.Position, len(positions))
	copy(afterPositions, positions)

	switch request.Action {
	case "trade", "currency_exposure":
		if request.Instrument == "" || request.Quantity <= 0 || request.Price <= 0 {
			return domain.ScenarioImpact{}, errors.New("instrument, quantity and price are required")
		}
		if request.Action == "trade" && request.Side != "buy" && request.Side != "sell" {
			return domain.ScenarioImpact{}, errors.New("scenario side must be buy or sell")
		}
		value := request.Quantity * request.Price
		if request.Side == "sell" {
			value = -value
		}
		found := false
		for i := range afterPositions {
			if afterPositions[i].Instrument == request.Instrument {
				afterPositions[i].MarketValue = max(0, afterPositions[i].MarketValue+value)
				found = true
				break
			}
		}
		if !found {
			afterPositions = append(afterPositions, domain.Position{Instrument: request.Instrument, MarketValue: max(0, value)})
		}
	case "reduce_risk":
		if request.RiskPercent <= 0 {
			return domain.ScenarioImpact{}, errors.New("risk percent must be greater than zero")
		}
		factor := 1 - request.RiskPercent/100
		for i := range afterPositions {
			afterPositions[i].MarketValue *= factor
		}
	default:
		return domain.ScenarioImpact{}, errors.New("unsupported scenario action")
	}
	after := s.riskSnapshotForPositions(portfolioID, afterPositions)
	return domain.ScenarioImpact{Action: request.Action, Before: before, After: after, DeltaVaR: after.VaR - before.VaR, DeltaCVaR: after.CVaR - before.CVaR, DeltaDrawdown: after.MaxDrawdown - before.MaxDrawdown, DeltaConcentration: after.Concentration - before.Concentration, DeltaExposure: after.Exposure - before.Exposure}, nil
}

func (s *Service) riskSnapshotForPositions(portfolioID string, positions []domain.Position) domain.RiskSnapshot {
	totalValue := 0.0
	largestExposure := 0.0
	for _, pos := range positions {
		totalValue += pos.MarketValue
		if pos.MarketValue > largestExposure {
			largestExposure = pos.MarketValue
		}
	}

	breaches := make([]string, 0)
	for _, limit := range s.RiskLimits(portfolioID) {
		if limit.Enabled && limit.Type == "max_portfolio_value" && totalValue > limit.Value {
			breaches = append(breaches, fmt.Sprintf("Portfolio value %.2f exceeds %.2f", totalValue, limit.Value))
		}
	}

	concentration := 0.0
	if totalValue > 0 {
		concentration = (largestExposure / totalValue) * 100
	}
	volatility := historicalVolatility(s.store.TradesForPortfolio(portfolioID))
	if volatility == 0 {
		volatility = 0.06
	}
	var95 := totalValue * volatility * 1.645
	cvar := var95 * 1.25
	drawdown := 0.0
	if totalValue > 0 {
		drawdown = historicalMaxDrawdown(s.store.TradesForPortfolio(portfolioID), totalValue)
	}
	status := "healthy"
	if len(breaches) > 0 || concentration > 70 || drawdown > 20 {
		status = "critical"
	} else if concentration > 50 || drawdown > 10 {
		status = "watch"
	}

	alerts := []domain.RiskAlert{}
	if len(breaches) > 0 {
		alerts = append(alerts, domain.RiskAlert{Level: "red", Title: "Portfolio limit breach", Detail: strings.Join(breaches, "; ")})
	} else if concentration > 50 {
		alerts = append(alerts, domain.RiskAlert{Level: "yellow", Title: "High concentration", Detail: fmt.Sprintf("Largest position represents %.1f%% of portfolio", concentration)})
	} else {
		alerts = append(alerts, domain.RiskAlert{Level: "green", Title: "Risk stable", Detail: "No active limit breaches detected."})
	}
	if drawdown > 10 {
		alerts = append(alerts, domain.RiskAlert{Level: "yellow", Title: "Drawdown drift", Detail: fmt.Sprintf("Estimated drawdown %.1f%%", drawdown)})
	}
	if concentration > 70 {
		alerts = append(alerts, domain.RiskAlert{Level: "red", Title: "Concentration risk", Detail: fmt.Sprintf("Largest exposure %.1f%% exceeds preferred threshold", concentration)})
	}
	if len(alerts) == 0 {
		alerts = append(alerts, domain.RiskAlert{Level: "green", Title: "Risk stable", Detail: "Portfolio is within expected exposure levels."})
	}

	return domain.RiskSnapshot{
		PortfolioID:   portfolioID,
		Status:        status,
		VaR:           var95,
		CVaR:          cvar,
		MaxDrawdown:   drawdown,
		Concentration: concentration,
		Exposure:      totalValue,
		LimitStatus:   status,
		Breaches:      breaches,
		Alerts:        alerts,
	}
}

func historicalMaxDrawdown(trades []domain.Trade, currentValue float64) float64 {
	if len(trades) < 2 || currentValue <= 0 {
		return 0
	}
	sort.SliceStable(trades, func(i, j int) bool { return trades[i].ExecutedAt.Before(trades[j].ExecutedAt) })
	quantity := make(map[string]float64)
	prices := make(map[string]float64)
	costBasis := make(map[string]float64)
	capital := 0.0
	pnl := 0.0
	peak := 0.0
	maxDrawdown := 0.0
	for _, trade := range trades {
		if previous := prices[trade.Instrument]; previous > 0 {
			pnl += quantity[trade.Instrument] * (trade.Price - previous)
		}
		switch trade.Side {
		case "buy":
			quantity[trade.Instrument] += trade.Quantity
			costBasis[trade.Instrument] += trade.Quantity*trade.Price + trade.Fee
			capital += trade.Quantity * trade.Price
		case "sell":
			open := quantity[trade.Instrument]
			if open > 0 && trade.Quantity <= open {
				average := costBasis[trade.Instrument] / open
				pnl += (trade.Price-average)*trade.Quantity - trade.Fee
				costBasis[trade.Instrument] -= average * trade.Quantity
				quantity[trade.Instrument] -= trade.Quantity
			}
		}
		prices[trade.Instrument] = trade.Price
		if pnl > peak {
			peak = pnl
		}
		base := math.Max(capital, currentValue)
		if base > 0 && peak-pnl > maxDrawdown {
			maxDrawdown = (peak - pnl) / base * 100
		}
	}
	return maxDrawdown
}

func historicalVolatility(trades []domain.Trade) float64 {
	prices := make(map[string]float64)
	returns := make([]float64, 0, len(trades))
	sort.SliceStable(trades, func(i, j int) bool { return trades[i].ExecutedAt.Before(trades[j].ExecutedAt) })
	for _, trade := range trades {
		if previous := prices[trade.Instrument]; previous > 0 && trade.Price > 0 {
			returns = append(returns, math.Log(trade.Price/previous))
		}
		if trade.Price > 0 {
			prices[trade.Instrument] = trade.Price
		}
	}
	if len(returns) < 2 {
		return 0
	}
	mean := 0.0
	for _, value := range returns {
		mean += value
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, value := range returns {
		delta := value - mean
		variance += delta * delta
	}
	return math.Sqrt(variance / float64(len(returns)-1))
}

func (s *Service) LedgerEntries(portfolioID string) []domain.LedgerEntry {
	return s.store.LedgerEntriesForPortfolio(portfolioID)
}

func (s *Service) AddRiskLimit(portfolioID, limitType string, value float64) (domain.RiskLimit, error) {
	return s.AddRiskLimitWithActor(portfolioID, limitType, value, "system")
}

func (s *Service) AddRiskLimitWithActor(portfolioID, limitType string, value float64, actor string) (domain.RiskLimit, error) {
	if limitType == "" || value <= 0 {
		return domain.RiskLimit{}, errors.New("invalid risk limit")
	}
	limit := domain.NewRiskLimit(limitType, value)
	s.store.AddRiskLimit(portfolioID, limit)
	s.recordAudit(portfolioID, actor, "create", "risk_limit", limit.ID, "risk limit configured", "", limit)
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
			s.recordAudit(portfolioID, "system", "trigger", "risk_event", event.ID, "risk limit breached", "", event)
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

func buildPosition(symbol string, trades []domain.Trade, marketPrice float64) domain.Position {
	var quantity, totalCost, realized, currentPrice float64
	currentPrice = marketPrice
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

func latestTradePrice(trades []domain.Trade) float64 {
	latest := time.Time{}
	price := 0.0
	for _, trade := range trades {
		if trade.Price > 0 && trade.ExecutedAt.After(latest) {
			latest = trade.ExecutedAt
			price = trade.Price
		}
	}
	return price
}
