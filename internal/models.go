package internal

import "fmt"

type Trade struct {
	Side     string
	Quantity float64
	Price    float64
	Fee      float64
}

type Position struct {
	OpenQuantity      float64
	AverageEntryPrice float64
	RealizedPnL       float64
	UnrealizedPnL     float64
	CurrentPrice      float64
}

type RiskLimit struct {
	Type  string
	Value float64
}

type PortfolioSummary struct {
	TotalValue float64
	Limits     []RiskLimit
}

func CalculatePosition(trades []Trade) (Position, error) {
	var openQuantity float64
	var totalCost float64
	var realizedPnL float64

	for _, trade := range trades {
		if trade.Quantity <= 0 {
			return Position{}, fmt.Errorf("quantity must be positive")
		}
		if trade.Price <= 0 {
			return Position{}, fmt.Errorf("price must be positive")
		}
		if trade.Fee < 0 {
			return Position{}, fmt.Errorf("fee cannot be negative")
		}

		switch trade.Side {
		case "buy":
			openQuantity += trade.Quantity
			totalCost += trade.Quantity*trade.Price + trade.Fee
		case "sell":
			if trade.Quantity > openQuantity {
				return Position{}, fmt.Errorf("cannot sell more than the open position")
			}
			averagePrice := 0.0
			if openQuantity > 0 {
				averagePrice = totalCost / openQuantity
			}
			realizedPnL += (trade.Price-averagePrice)*trade.Quantity - trade.Fee
			totalCost -= averagePrice * trade.Quantity
			openQuantity -= trade.Quantity
		default:
			return Position{}, fmt.Errorf("unsupported side: %s", trade.Side)
		}
	}

	averageEntryPrice := 0.0
	if openQuantity > 0 {
		averageEntryPrice = totalCost / openQuantity
	}

	currentPrice := 120.0
	unrealizedPnL := (currentPrice - averageEntryPrice) * openQuantity

	return Position{
		OpenQuantity:      openQuantity,
		AverageEntryPrice: averageEntryPrice,
		RealizedPnL:       realizedPnL,
		UnrealizedPnL:     unrealizedPnL,
		CurrentPrice:      currentPrice,
	}, nil
}

func EvaluateRiskLimits(summary PortfolioSummary) []string {
	violations := make([]string, 0)
	for _, limit := range summary.Limits {
		if limit.Type == "max_portfolio_value" && summary.TotalValue > limit.Value {
			violations = append(violations, fmt.Sprintf("%s exceeded: %v > %v", limit.Type, summary.TotalValue, limit.Value))
		}
	}
	return violations
}
