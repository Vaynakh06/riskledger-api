package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

type AlertNotifier interface {
	Dispatch(message string, channels []string) map[string]string
}

type ChannelNotifier struct {
	smtpHost, smtpPort, smtpUser, smtpPassword, alertFrom              string
	telegramBotToken, telegramChatID, slackWebhookURL, alertWebhookURL string
	client                                                             *http.Client
}

type NotifierConfig struct {
	SMTPHost, SMTPPort, SMTPUser, SMTPPassword, AlertFrom              string
	TelegramBotToken, TelegramChatID, SlackWebhookURL, AlertWebhookURL string
}

func NewChannelNotifier(config NotifierConfig) *ChannelNotifier {
	return &ChannelNotifier{smtpHost: config.SMTPHost, smtpPort: config.SMTPPort, smtpUser: config.SMTPUser, smtpPassword: config.SMTPPassword, alertFrom: config.AlertFrom, telegramBotToken: config.TelegramBotToken, telegramChatID: config.TelegramChatID, slackWebhookURL: config.SlackWebhookURL, alertWebhookURL: config.AlertWebhookURL, client: &http.Client{Timeout: 5 * time.Second}}
}

func (n *ChannelNotifier) Dispatch(message string, channels []string) map[string]string {
	result := make(map[string]string, len(channels))
	for _, channel := range channels {
		var err error
		switch channel {
		case "email":
			err = n.email(message)
		case "telegram":
			err = n.telegram(message)
		case "slack":
			err = n.webhook(n.slackWebhookURL, map[string]string{"text": message})
		case "webhook":
			err = n.webhook(n.alertWebhookURL, map[string]string{"message": message, "source": "riskledger"})
		default:
			err = fmt.Errorf("unsupported channel")
		}
		if err != nil {
			if strings.Contains(err.Error(), "not configured") {
				result[channel] = "not_configured"
			} else {
				result[channel] = "failed"
			}
		} else {
			result[channel] = "sent"
		}
	}
	return result
}

func (n *ChannelNotifier) email(message string) error {
	if n.smtpHost == "" || n.smtpUser == "" || n.smtpPassword == "" || n.alertFrom == "" {
		return fmt.Errorf("email not configured")
	}
	auth := smtp.PlainAuth("", n.smtpUser, n.smtpPassword, n.smtpHost)
	return smtp.SendMail(n.smtpHost+":"+n.smtpPort, auth, n.alertFrom, []string{n.alertFrom}, []byte("Subject: RiskLedger alert\r\n\r\n"+message))
}

func (n *ChannelNotifier) telegram(message string) error {
	if n.telegramBotToken == "" || n.telegramChatID == "" {
		return fmt.Errorf("telegram not configured")
	}
	response, err := n.client.PostForm("https://api.telegram.org/bot"+n.telegramBotToken+"/sendMessage", url.Values{"chat_id": {n.telegramChatID}, "text": {message}})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("telegram status %d", response.StatusCode)
	}
	return nil
}

func (n *ChannelNotifier) webhook(endpoint string, payload map[string]string) error {
	if endpoint == "" {
		return fmt.Errorf("webhook not configured")
	}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := n.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", response.StatusCode)
	}
	return nil
}
