package domain

import (
	"crypto/rand"
	"fmt"
	"time"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Portfolio struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Name         string    `json:"name"`
	BaseCurrency string    `json:"base_currency"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
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
