package alerts

import (
	"time"

	"github.com/google/uuid"
)

type AlertChannelType string

const (
	ChannelSlack   AlertChannelType = "slack"
	ChannelWebhook AlertChannelType = "webhook"
	ChannelEmail  AlertChannelType = "email"
	ChannelLog    AlertChannelType = "log"
)

type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityWarning  AlertSeverity = "warning"
	SeverityInfo     AlertSeverity = "info"
)

type AlertStatus string

const (
	StatusSent       AlertStatus = "sent"
	StatusRateLimited AlertStatus = "rate_limited"
	StatusFailed     AlertStatus = "failed"
)

type AlertChannel struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Type      AlertChannelType `json:"type"`
	Config    JSONMap         `json:"config"`
	CreatedAt time.Time       `json:"createdAt"`
}

type JSONMap map[string]interface{}

type AlertHistory struct {
	ID          uuid.UUID     `json:"id"`
	Name        string       `json:"name"`
	Severity    AlertSeverity `json:"severity"`
	Message     string       `json:"message"`
	Channel     string       `json:"channel"`
	Status      AlertStatus  `json:"status"`
	TriggeredBy string       `json:"triggeredBy"`
	CreatedAt   time.Time    `json:"createdAt"`
}

type AlertRule struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Condition         string    `json:"condition"`
	NotificationChannel string `json:"notificationChannel"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"createdAt"`
}
