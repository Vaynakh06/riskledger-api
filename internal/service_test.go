package internal

import "testing"

func TestAverageEntryPriceAndPnl(t *testing.T) {
	trades := []Trade{
		{Side: "buy", Quantity: 2, Price: 100, Fee: 0},
		{Side: "buy", Quantity: 3, Price: 120, Fee: 0},
		{Side: "sell", Quantity: 2, Price: 130, Fee: 5},
	}

	position, err := CalculatePosition(trades)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if position.AverageEntryPrice != 112 {
		t.Fatalf("average entry price = %v, want 112", position.AverageEntryPrice)
	}

	if position.RealizedPnL != 31 {
		t.Fatalf("realized pnl = %v, want 31", position.RealizedPnL)
	}

	if position.OpenQuantity != 3 {
		t.Fatalf("open quantity = %v, want 3", position.OpenQuantity)
	}
}

func TestRejectOverSelling(t *testing.T) {
	trades := []Trade{
		{Side: "buy", Quantity: 2, Price: 100, Fee: 0},
		{Side: "sell", Quantity: 3, Price: 120, Fee: 0},
	}

	_, err := CalculatePosition(trades)
	if err == nil {
		t.Fatal("expected error for selling more than available")
	}
}

func TestRiskLimitViolation(t *testing.T) {
	portfolio := PortfolioSummary{
		TotalValue: 1000,
		Limits:     []RiskLimit{{Type: "max_portfolio_value", Value: 800}},
	}

	violations := EvaluateRiskLimits(portfolio)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}
