package service

import (
	"testing"
	"time"

	"riskledger/internal/domain"
	"riskledger/internal/store"
)

func TestAddTradeCreatesLedgerEntry(t *testing.T) {
	mem := store.NewInMemoryStore()
	mem.CreatePortfolio(domain.Portfolio{ID: "p1", UserID: "u1", Name: "Demo", BaseCurrency: "USD", CreatedAt: time.Now(), UpdatedAt: time.Now()})

	svc := NewService(mem)
	if err := svc.AddTrade("p1", "AAPL", "buy", 10, 150, 2, "growth", "first trade", []string{"tech", "core"}, true); err != nil {
		t.Fatalf("AddTrade returned error: %v", err)
	}

	entries := svc.LedgerEntries("p1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(entries))
	}
	if entries[0].Type != "trade" {
		t.Fatalf("expected entry type trade, got %q", entries[0].Type)
	}
	if entries[0].Reason == "" {
		t.Fatal("expected ledger reason to be set")
	}
	if entries[0].CreatedBy == "" {
		t.Fatal("expected ledger author to be set")
	}
}

func TestRiskSnapshotProducesStatus(t *testing.T) {
	mem := store.NewInMemoryStore()
	mem.CreatePortfolio(domain.Portfolio{ID: "p2", UserID: "u1", Name: "Risk", BaseCurrency: "USD", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	mem.AddTrade(domain.Trade{ID: "t1", PortfolioID: "p2", Instrument: "AAPL", Side: "buy", Quantity: 50, Price: 200, Fee: 0, ExecutedAt: time.Now()})
	mem.AddTrade(domain.Trade{ID: "t2", PortfolioID: "p2", Instrument: "MSFT", Side: "buy", Quantity: 40, Price: 300, Fee: 0, ExecutedAt: time.Now()})

	svc := NewService(mem)
	snapshot := svc.RiskSnapshot("p2")
	if snapshot.Status == "" {
		t.Fatal("expected non-empty risk status")
	}
	if snapshot.VaR <= 0 || snapshot.CVaR <= 0 {
		t.Fatal("expected risk metrics to be positive")
	}
}

func TestImportBrokerReconcilesRows(t *testing.T) {
	mem := store.NewInMemoryStore()
	mem.CreatePortfolio(domain.Portfolio{ID: "p3", UserID: "u1", Name: "Imports", BaseCurrency: "USD", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	svc := NewService(mem)
	data := []byte("transaction_id,type,symbol,side,quantity,price,fee,amount,currency,date\n" +
		"tx-1,trade,AAPL,buy,2,100,1,201,USD,2026-01-02\n" +
		"tx-2,fee,,,0,0,3,-3,USD,2026-01-02\n" +
		"tx-3,dividend,MSFT,,,0,0,25,USD,2026-01-03\n" +
		"tx-4,trade,,buy,1,100,0,100,USD,2026-01-04\n")

	batch, err := svc.ImportBroker("p3", "broker-a", "activity.csv", data)
	if err != nil {
		t.Fatalf("ImportBroker returned error: %v", err)
	}
	if batch.Total != 4 || batch.Matched != 3 || batch.Unmatched != 1 {
		t.Fatalf("unexpected reconciliation summary: %+v", batch)
	}
	if len(svc.TradesForPortfolio("p3")) != 1 {
		t.Fatalf("expected one imported trade")
	}
	if len(svc.LedgerEntries("p3")) != 3 {
		t.Fatalf("expected trade, fee and dividend ledger entries, got %d records=%+v", len(svc.LedgerEntries("p3")), svc.ImportRecords(batch.ID))
	}
	if len(svc.ImportRecords(batch.ID)) != 4 {
		t.Fatalf("expected four import records")
	}
}

func TestScenarioDoesNotMutatePortfolio(t *testing.T) {
	mem := store.NewInMemoryStore()
	mem.CreatePortfolio(domain.Portfolio{ID: "p4", UserID: "u1", Name: "Scenario", BaseCurrency: "USD", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	mem.AddTrade(domain.Trade{ID: "t1", PortfolioID: "p4", Instrument: "BTC", Side: "buy", Quantity: 1, Price: 100, Fee: 0, ExecutedAt: time.Now()})

	svc := NewService(mem)
	before := svc.RiskSnapshot("p4")
	impact, err := svc.Scenario("p4", domain.ScenarioRequest{Action: "trade", Instrument: "ETH", Side: "buy", Quantity: 10, Price: 100})
	if err != nil {
		t.Fatalf("Scenario returned error: %v", err)
	}
	if impact.After.Exposure <= impact.Before.Exposure || impact.DeltaExposure <= 0 {
		t.Fatalf("expected positive scenario exposure delta: %+v", impact)
	}
	if len(svc.TradesForPortfolio("p4")) != 1 {
		t.Fatalf("scenario must not create a trade")
	}
	if after := svc.RiskSnapshot("p4"); after.Exposure != before.Exposure {
		t.Fatalf("scenario mutated portfolio exposure: before=%v after=%v", before.Exposure, after.Exposure)
	}
}

func TestAlertsAndAuditHistory(t *testing.T) {
	mem := store.NewInMemoryStore()
	mem.CreatePortfolio(domain.Portfolio{ID: "p5", UserID: "u1", Name: "Alerts", BaseCurrency: "USD", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	svc := NewService(mem)
	if err := svc.AddTrade("p5", "BTC", "buy", 1, 1000, 0, "", "seed", nil, false); err != nil {
		t.Fatalf("AddTrade returned error: %v", err)
	}
	if _, err := svc.CreateAlertRule("p5", "concentration", 70, "critical", "u1", []string{"email", "telegram", "slack", "webhook"}); err != nil {
		t.Fatalf("CreateAlertRule returned error: %v", err)
	}
	events := svc.EvaluateAlerts("p5", "u1")
	if len(events) != 1 || events[0].Channels["telegram"] != "not_configured" {
		t.Fatalf("expected explicit multi-channel delivery status, got %+v", events)
	}
	if len(svc.AuditEntries("p5")) < 3 {
		t.Fatalf("expected trade, rule, and alert audit entries")
	}
}

func TestCashAndPerformanceReport(t *testing.T) {
	mem := store.NewInMemoryStore()
	mem.CreatePortfolio(domain.Portfolio{ID: "p6", UserID: "u1", Name: "Performance", BaseCurrency: "USD", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	svc := NewService(mem)
	if _, err := svc.AddCash("p6", "deposit", 1000, "USD", "funding", "u1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTrade("p6", "BTC", "buy", 1, 100, 0, "", "", nil, false); err != nil {
		t.Fatal(err)
	}
	report := svc.PerformanceReport("p6")
	if report.CashValue != 1000 || report.TotalValue <= 0 {
		t.Fatalf("unexpected performance report: %+v", report)
	}
}
