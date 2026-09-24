package domain

import (
	"crypto/rand"
	"fmt"
	"time"
)

type User struct {
	ID               string    `json:"id"`
	Email            string    `json:"email"`
	PasswordHash     string    `json:"-"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Role             string    `json:"role"`
	TwoFactorEnabled bool      `json:"two_factor_enabled"`
	TwoFactorSecret  string    `json:"-"`
}

type Portfolio struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Name         string    `json:"name"`
	BaseCurrency string    `json:"base_currency"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CashEntry struct {
	ID          string    `json:"id"`
	PortfolioID string    `json:"portfolio_id"`
	Type        string    `json:"type"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Reason      string    `json:"reason"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type FXRate struct {
	Base   string    `json:"base"`
	Quote  string    `json:"quote"`
	Rate   float64   `json:"rate"`
	AsOf   time.Time `json:"as_of"`
	Source string    `json:"source"`
}

type Benchmark struct {
	ID          string `json:"id"`
	PortfolioID string `json:"portfolio_id"`
	Symbol      string `json:"symbol"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
}

type Allocation struct {
	ID            string  `json:"id"`
	PortfolioID   string  `json:"portfolio_id"`
	Instrument    string  `json:"instrument"`
	Category      string  `json:"category"`
	TargetWeight  float64 `json:"target_weight"`
	CurrentWeight float64 `json:"current_weight"`
}

type BrokerConnection struct {
	ID          string     `json:"id"`
	PortfolioID string     `json:"portfolio_id"`
	Provider    string     `json:"provider"`
	Status      string     `json:"status"`
	LastSyncAt  *time.Time `json:"last_sync_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
}

type DeliveryJob struct {
	ID            string    `json:"id"`
	AlertEventID  string    `json:"alert_event_id"`
	Channel       string    `json:"channel"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
	LastError     string    `json:"last_error,omitempty"`
}

type PerformanceReport struct {
	PortfolioID     string       `json:"portfolio_id"`
	TotalValue      float64      `json:"total_value"`
	CashValue       float64      `json:"cash_value"`
	TotalReturn     float64      `json:"total_return"`
	TWR             float64      `json:"twr"`
	IRR             float64      `json:"irr"`
	BenchmarkSymbol string       `json:"benchmark_symbol,omitempty"`
	BenchmarkReturn float64      `json:"benchmark_return,omitempty"`
	Allocations     []Allocation `json:"allocations"`
}

type Trade struct {
	ID          string    `json:"id"`
	PortfolioID string    `json:"portfolio_id"`
	Instrument  string    `json:"instrument"`
	Side        string    `json:"side"`
	Quantity    float64   `json:"quantity"`
	Price       float64   `json:"price"`
	Fee         float64   `json:"fee"`
	Strategy    string    `json:"strategy,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	ChecklistOK bool      `json:"checklist_ok"`
	ExecutedAt  time.Time `json:"executed_at"`
}

type Position struct {
	Instrument        string  `json:"instrument"`
	Quantity          float64 `json:"quantity"`
	AverageEntryPrice float64 `json:"average_entry_price"`
	MarketPrice       float64 `json:"market_price"`
	MarketValue       float64 `json:"market_value"`
	RealizedPnL       float64 `json:"realized_pnl"`
	UnrealizedPnL     float64 `json:"unrealized_pnl"`
}

type RiskLimit struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Value   float64 `json:"value"`
	Enabled bool    `json:"enabled"`
}

type RiskEvent struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	CurrentValue float64   `json:"current_value"`
	AllowedValue float64   `json:"allowed_value"`
	Status       string    `json:"status"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type RiskAlert struct {
	Level  string `json:"level"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type RiskSnapshot struct {
	PortfolioID   string      `json:"portfolio_id"`
	Status        string      `json:"status"`
	VaR           float64     `json:"var"`
	CVaR          float64     `json:"cvar"`
	MaxDrawdown   float64     `json:"max_drawdown"`
	Concentration float64     `json:"concentration"`
	Exposure      float64     `json:"exposure"`
	LimitStatus   string      `json:"limit_status"`
	Breaches      []string    `json:"breaches"`
	Alerts        []RiskAlert `json:"alerts"`
}

type AlertRule struct {
	ID          string    `json:"id"`
	PortfolioID string    `json:"portfolio_id"`
	Type        string    `json:"type"`
	Threshold   float64   `json:"threshold"`
	Severity    string    `json:"severity"`
	Channels    []string  `json:"channels"`
	Enabled     bool      `json:"enabled"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type AlertEvent struct {
	ID          string            `json:"id"`
	PortfolioID string            `json:"portfolio_id"`
	RuleID      string            `json:"rule_id"`
	Type        string            `json:"type"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"`
	Message     string            `json:"message"`
	Channels    map[string]string `json:"channels"`
	TriggeredAt time.Time         `json:"triggered_at"`
}

type AuditEntry struct {
	ID          string    `json:"id"`
	PortfolioID string    `json:"portfolio_id"`
	Actor       string    `json:"actor"`
	Action      string    `json:"action"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	Reason      string    `json:"reason"`
	Before      string    `json:"before,omitempty"`
	After       string    `json:"after,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type ScenarioRequest struct {
	Action      string  `json:"action"`
	Instrument  string  `json:"instrument"`
	Side        string  `json:"side"`
	Quantity    float64 `json:"quantity"`
	Price       float64 `json:"price"`
	RiskPercent float64 `json:"risk_percent"`
	Strategy    string  `json:"strategy,omitempty"`
}

type ScenarioImpact struct {
	Action             string       `json:"action"`
	Before             RiskSnapshot `json:"before"`
	After              RiskSnapshot `json:"after"`
	DeltaVaR           float64      `json:"delta_var"`
	DeltaCVaR          float64      `json:"delta_cvar"`
	DeltaDrawdown      float64      `json:"delta_drawdown"`
	DeltaConcentration float64      `json:"delta_concentration"`
	DeltaExposure      float64      `json:"delta_exposure"`
}

type LedgerEntry struct {
	ID          string    `json:"id"`
	PortfolioID string    `json:"portfolio_id"`
	Type        string    `json:"type"`
	Instrument  string    `json:"instrument,omitempty"`
	Side        string    `json:"side,omitempty"`
	Quantity    float64   `json:"quantity"`
	Price       float64   `json:"price"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Reason      string    `json:"reason"`
	Notes       string    `json:"notes,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type ImportRecord struct {
	ID         string    `json:"id"`
	BatchID    string    `json:"batch_id"`
	Source     string    `json:"source"`
	ExternalID string    `json:"external_id,omitempty"`
	Kind       string    `json:"kind"`
	Status     string    `json:"status"`
	Instrument string    `json:"instrument,omitempty"`
	Side       string    `json:"side,omitempty"`
	Quantity   float64   `json:"quantity"`
	Price      float64   `json:"price"`
	Fee        float64   `json:"fee"`
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
	ExecutedAt time.Time `json:"executed_at"`
	Reason     string    `json:"reason,omitempty"`
	Raw        string    `json:"raw,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type ImportBatch struct {
	ID          string         `json:"id"`
	PortfolioID string         `json:"portfolio_id"`
	Source      string         `json:"source"`
	Filename    string         `json:"filename"`
	CreatedAt   time.Time      `json:"created_at"`
	Total       int            `json:"total"`
	Matched     int            `json:"matched"`
	Unmatched   int            `json:"unmatched"`
	Review      int            `json:"needs_review"`
	Records     []ImportRecord `json:"records,omitempty"`
}

func NewImportBatch(portfolioID, source, filename string) ImportBatch {
	return ImportBatch{ID: generateID(), PortfolioID: portfolioID, Source: source, Filename: filename, CreatedAt: time.Now()}
}

func NewImportRecord(batchID, source, kind string) ImportRecord {
	return ImportRecord{ID: generateID(), BatchID: batchID, Source: source, Kind: kind, Status: "needs_review", Currency: "USD", CreatedAt: time.Now()}
}

func NewAlertRule(portfolioID, alertType string, threshold float64, severity, createdBy string, channels []string) AlertRule {
	return AlertRule{ID: generateID(), PortfolioID: portfolioID, Type: alertType, Threshold: threshold, Severity: severity, Channels: channels, Enabled: true, CreatedBy: createdBy, CreatedAt: time.Now()}
}

func NewAlertEvent(portfolioID, ruleID, alertType, severity, message string, channels map[string]string) AlertEvent {
	return AlertEvent{ID: generateID(), PortfolioID: portfolioID, RuleID: ruleID, Type: alertType, Severity: severity, Status: "triggered", Message: message, Channels: channels, TriggeredAt: time.Now()}
}

func NewAuditEntry(portfolioID, actor, action, entityType, entityID, reason, before, after string) AuditEntry {
	return AuditEntry{ID: generateID(), PortfolioID: portfolioID, Actor: actor, Action: action, EntityType: entityType, EntityID: entityID, Reason: reason, Before: before, After: after, CreatedAt: time.Now()}
}

func generateID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("00000000-0000-4000-8000-%012d", time.Now().UnixNano()%1_000_000_000_000)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func NewUser(email string, passwordHash string) User {
	return User{
		ID:           generateID(),
		Email:        email,
		PasswordHash: passwordHash,
		Role:         "owner",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func NewCashEntry(portfolioID, entryType string, amount float64, currency, reason string) CashEntry {
	return CashEntry{ID: generateID(), PortfolioID: portfolioID, Type: entryType, Amount: amount, Currency: currency, Reason: reason, OccurredAt: time.Now()}
}
func NewBenchmark(portfolioID, symbol, name string) Benchmark {
	return Benchmark{ID: generateID(), PortfolioID: portfolioID, Symbol: symbol, Name: name, Enabled: true}
}
func NewAllocation(portfolioID, instrument, category string, target float64) Allocation {
	return Allocation{ID: generateID(), PortfolioID: portfolioID, Instrument: instrument, Category: category, TargetWeight: target}
}
func NewBrokerConnection(portfolioID, provider string) BrokerConnection {
	return BrokerConnection{ID: generateID(), PortfolioID: portfolioID, Provider: provider, Status: "configured"}
}
func NewDeliveryJob(eventID, channel string) DeliveryJob {
	return DeliveryJob{ID: generateID(), AlertEventID: eventID, Channel: channel, Status: "pending", NextAttemptAt: time.Now()}
}

func NewPortfolio(userID, name, baseCurrency string) Portfolio {
	return Portfolio{
		ID:           generateID(),
		UserID:       userID,
		Name:         name,
		BaseCurrency: baseCurrency,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func NewTrade(portfolioID, instrument, side string, quantity, price, fee float64, strategy, notes string, tags []string, checklistOK bool) Trade {
	return Trade{
		ID:          generateID(),
		PortfolioID: portfolioID,
		Instrument:  instrument,
		Side:        side,
		Quantity:    quantity,
		Price:       price,
		Fee:         fee,
		Strategy:    strategy,
		Tags:        tags,
		Notes:       notes,
		ChecklistOK: checklistOK,
		ExecutedAt:  time.Now(),
	}
}

func NewRiskLimit(limitType string, value float64) RiskLimit {
	return RiskLimit{ID: generateID(), Type: limitType, Value: value, Enabled: true}
}

func NewRiskEvent(limitType string, currentValue, allowedValue float64) RiskEvent {
	return RiskEvent{
		ID:           generateID(),
		Type:         limitType,
		CurrentValue: currentValue,
		AllowedValue: allowedValue,
		Status:       "open",
		OccurredAt:   time.Now(),
	}
}

func NewLedgerEntry(portfolioID, entryType, instrument, side string, quantity, price, amount float64, currency, reason, notes, createdBy string) LedgerEntry {
	if currency == "" {
		currency = "USD"
	}
	if reason == "" {
		reason = "trade recorded"
	}
	if createdBy == "" {
		createdBy = "system"
	}
	return LedgerEntry{
		ID:          generateID(),
		PortfolioID: portfolioID,
		Type:        entryType,
		Instrument:  instrument,
		Side:        side,
		Quantity:    quantity,
		Price:       price,
		Amount:      amount,
		Currency:    currency,
		Reason:      reason,
		Notes:       notes,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
	}
}
