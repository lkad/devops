package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ChannelSender interface {
	Send(alert *AlertHistory) error
	Name() string
}

type SlackSender struct {
	webhookURL string
	client     *http.Client
}

func NewSlackSender(webhookURL string) *SlackSender {
	return &SlackSender{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *SlackSender) Name() string {
	return "slack"
}

func (s *SlackSender) Send(alert *AlertHistory) error {
	payload := map[string]interface{}{
		"text": fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Name, alert.Message),
		"attachments": []map[string]interface{}{
			{
				"color": colorForSeverity(alert.Severity),
				"fields": []map[string]interface{}{
					{"title": "Channel", "value": alert.Channel, "short": true},
					{"title": "Triggered By", "value": alert.TriggeredBy, "short": true},
					{"title": "Time", "value": alert.CreatedAt.Format(time.RFC3339), "short": true},
				},
			},
		},
	}

	data, _ := json.Marshal(payload)
	resp, err := s.client.Post(s.webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack API error: %d", resp.StatusCode)
	}
	return nil
}

type WebhookSender struct {
	url    string
	client *http.Client
}

func NewWebhookSender(url string) *WebhookSender {
	return &WebhookSender{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *WebhookSender) Name() string {
	return "webhook"
}

func (s *WebhookSender) Send(alert *AlertHistory) error {
	payload := map[string]interface{}{
		"alert": alert,
	}

	data, _ := json.Marshal(payload)
	resp, err := s.client.Post(s.url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook error: %d", resp.StatusCode)
	}
	return nil
}

type LogSender struct{}

func NewLogSender() *LogSender {
	return &LogSender{}
}

func (s *LogSender) Name() string {
	return "log"
}

func (s *LogSender) Send(alert *AlertHistory) error {
	// Just log the alert - actual logging handled by logs package
	fmt.Printf("[ALERT][%s][%s] %s: %s\n", alert.Severity, alert.Channel, alert.Name, alert.Message)
	return nil
}

type EmailSender struct {
	smtpHost string
	smtpPort int
	from     string
	client   *http.Client
}

func NewEmailSender(smtpHost string, smtpPort int, from string) *EmailSender {
	return &EmailSender{
		smtpHost: smtpHost,
		smtpPort: smtpPort,
		from:     from,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *EmailSender) Name() string {
	return "email"
}

func (s *EmailSender) Send(alert *AlertHistory) error {
	// Simplified - in production use smtp package
	fmt.Printf("[EMAIL][%s] To: %s, Subject: [%s] %s\n",
		alert.Channel, s.from, alert.Severity, alert.Name)
	return nil
}

func colorForSeverity(severity AlertSeverity) string {
	switch severity {
	case SeverityCritical:
		return "#FF0000"
	case SeverityWarning:
		return "#FFA500"
	case SeverityInfo:
		return "#00FF00"
	default:
		return "#808080"
	}
}
