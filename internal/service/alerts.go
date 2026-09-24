package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"riskledger/internal/domain"
)

var allowedAlertTypes = map[string]bool{"breach": true, "drawdown": true, "concentration": true, "limits": true, "unusual_activity": true}
var allowedAlertChannels = map[string]bool{"email": true, "telegram": true, "slack": true, "webhook": true}

func (s *Service) CreateAlertRule(portfolioID, alertType string, threshold float64, severity, actor string, channels []string) (domain.AlertRule, error) {
	if !allowedAlertTypes[alertType] || threshold < 0 {
		return domain.AlertRule{}, errors.New("invalid alert type or threshold")
	}
	if severity == "" {
		severity = "warning"
	}
	if severity != "info" && severity != "warning" && severity != "critical" {
		return domain.AlertRule{}, errors.New("invalid alert severity")
	}
	cleanChannels := make([]string, 0, len(channels))
	for _, channel := range channels {
		channel = strings.ToLower(strings.TrimSpace(channel))
		if !allowedAlertChannels[channel] {
			return domain.AlertRule{}, fmt.Errorf("unsupported alert channel: %s", channel)
		}
		cleanChannels = append(cleanChannels, channel)
	}
	if len(cleanChannels) == 0 {
		cleanChannels = []string{"email"}
	}
	rule := domain.NewAlertRule(portfolioID, alertType, threshold, severity, actor, cleanChannels)
	s.store.AddAlertRule(rule)
	s.recordAudit(portfolioID, actor, "create", "alert_rule", rule.ID, "alert rule configured", "", rule)
	return rule, nil
}

func (s *Service) AlertRules(portfolioID string) []domain.AlertRule {
	return s.store.AlertRulesForPortfolio(portfolioID)
}

func (s *Service) AlertEvents(portfolioID string) []domain.AlertEvent {
	return s.store.AlertEventsForPortfolio(portfolioID)
}

func (s *Service) EvaluateAlerts(portfolioID, actor string) []domain.AlertEvent {
	snapshot := s.RiskSnapshot(portfolioID)
	rules := s.AlertRules(portfolioID)
	events := make([]domain.AlertEvent, 0)
	for _, rule := range rules {
		if !rule.Enabled || !alertRuleTriggered(rule, snapshot, s.TradesForPortfolio(portfolioID)) {
			continue
		}
		message := alertMessage(rule, snapshot)
		delivery := make(map[string]string, len(rule.Channels))
		for _, channel := range rule.Channels {
			delivery[channel] = "not_configured"
		}
		if s.notifier != nil {
			delivery = s.notifier.Dispatch(message, rule.Channels)
		}
		event := domain.NewAlertEvent(portfolioID, rule.ID, rule.Type, rule.Severity, message, delivery)
		s.store.AddAlertEvent(event)
		for channel, status := range delivery {
			job := domain.NewDeliveryJob(event.ID, channel)
			job.Status = status
			job.Attempts = 1
			if status == "failed" || status == "not_configured" {
				job.NextAttemptAt = time.Now().Add(5 * time.Minute)
				job.LastError = "delivery requires retry or configuration"
			}
			s.store.AddDeliveryJob(job)
		}
		s.recordAudit(portfolioID, actor, "trigger", "alert", event.ID, message, "", event)
		events = append(events, event)
	}
	return events
}

func (s *Service) AuditEntries(portfolioID string) []domain.AuditEntry {
	return s.store.AuditEntriesForPortfolio(portfolioID)
}

func (s *Service) ProcessDeliveryJobs() {
	if s.notifier == nil {
		return
	}
	for _, job := range s.store.DeliveryJobs() {
		if job.Status == "sent" || time.Now().Before(job.NextAttemptAt) {
			continue
		}
		event, ok := s.store.AlertEventByID(job.AlertEventID)
		if !ok {
			continue
		}
		status := s.notifier.Dispatch(event.Message, []string{job.Channel})[job.Channel]
		job.Attempts++
		job.Status = status
		if status != "sent" {
			if job.Attempts >= 5 {
				job.Status = "escalated"
				job.LastError = "delivery failed after five attempts"
			} else {
				job.Status = "failed"
				job.NextAttemptAt = time.Now().Add(time.Duration(1<<min(job.Attempts, 4)) * time.Minute)
				job.LastError = "delivery retry scheduled"
			}
		}
		s.store.UpdateDeliveryJob(job)
	}
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func (s *Service) recordAudit(portfolioID, actor, action, entityType, entityID, reason string, before, after any) {
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	s.store.AddAuditEntry(domain.NewAuditEntry(portfolioID, actor, action, entityType, entityID, reason, string(beforeJSON), string(afterJSON)))
}

func alertRuleTriggered(rule domain.AlertRule, snapshot domain.RiskSnapshot, trades []domain.Trade) bool {
	switch rule.Type {
	case "breach", "limits":
		return len(snapshot.Breaches) > 0
	case "drawdown":
		return snapshot.MaxDrawdown >= rule.Threshold
	case "concentration":
		return snapshot.Concentration >= rule.Threshold
	case "unusual_activity":
		return float64(len(trades)) >= rule.Threshold
	default:
		return false
	}
}

func alertMessage(rule domain.AlertRule, snapshot domain.RiskSnapshot) string {
	switch rule.Type {
	case "drawdown":
		return fmt.Sprintf("Drawdown %.1f%% reached alert threshold %.1f%%", snapshot.MaxDrawdown, rule.Threshold)
	case "concentration":
		return fmt.Sprintf("Concentration %.1f%% reached alert threshold %.1f%%", snapshot.Concentration, rule.Threshold)
	case "unusual_activity":
		return fmt.Sprintf("Unusual activity: portfolio has %.0f trades", rule.Threshold)
	default:
		return strings.Join(snapshot.Breaches, "; ")
	}
}
