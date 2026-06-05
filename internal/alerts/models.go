package alerts

import (
	"errors"
	"time"

	"github.com/devops-toolkit/backend/internal/database"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Severity is the impact level of an Alert. The three values
// correspond to the PRD supplement's severity ladder:
//   - info:     human-curiosity, no action required
//   - warning:  investigate at next business hour
//   - critical: wake someone up
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Valid reports whether s is one of the three recognised
// severities. Used by the service layer to reject bogus create /
// update requests with a 400 VALIDATION_ERROR.
func (s Severity) Valid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	}
	return false
}

// State is the lifecycle state of an Alert. The three values are
// the alert-state machine: firing -> acknowledged (optional) ->
// resolved. A resolved alert is terminal.
type State string

const (
	StateFiring       State = "firing"
	StateAcknowledged State = "acknowledged"
	StateResolved     State = "resolved"
)

// Valid mirrors Severity.Valid for the alert-state machine.
func (s State) Valid() bool {
	switch s {
	case StateFiring, StateAcknowledged, StateResolved:
		return true
	}
	return false
}

// SourceType identifies the system that produced the alert. The
// spec calls out three (physical_host, k8s, pipeline) plus a
// catch-all "custom" for ad-hoc triggers.
type SourceType string

const (
	SourceTypePhysicalHost SourceType = "physical_host"
	SourceTypeK8s          SourceType = "k8s"
	SourceTypePipeline     SourceType = "pipeline"
	SourceTypeCustom       SourceType = "custom"
)

// Valid reports whether t is one of the recognised source types.
func (t SourceType) Valid() bool {
	switch t {
	case SourceTypePhysicalHost, SourceTypeK8s, SourceTypePipeline, SourceTypeCustom:
		return true
	}
	return false
}

// ChannelType is the destination technology for a Channel. The
// spec mandates five: slack, pagerduty, webhook, email, log.
type ChannelType string

const (
	ChannelTypeSlack     ChannelType = "slack"
	ChannelTypePagerDuty ChannelType = "pagerduty"
	ChannelTypeWebhook   ChannelType = "webhook"
	ChannelTypeEmail     ChannelType = "email"
	ChannelTypeLog       ChannelType = "log"
)

// Valid reports whether t is one of the recognised channel types.
func (t ChannelType) Valid() bool {
	switch t {
	case ChannelTypeSlack, ChannelTypePagerDuty, ChannelTypeWebhook, ChannelTypeEmail, ChannelTypeLog:
		return true
	}
	return false
}

// IsExternal reports whether a channel type is "external" for the
// purpose of maintenance suppression. Log channels are internal
// audit records and must always receive the alert (with the
// suppressed flag) so a host being in maintenance does not lose
// the audit trail.
func (t ChannelType) IsExternal() bool {
	switch t {
	case ChannelTypeSlack, ChannelTypePagerDuty, ChannelTypeWebhook, ChannelTypeEmail:
		return true
	}
	return false
}

// Alert is the row the alert-notification subsystem writes. It
// captures the lifecycle of a single firing event: who triggered
// it, what the source is, and the acknowledge / resolve trail.
// The Suppressed / SuppressionReason columns are populated by
// the service layer when the alert would otherwise have been
// delivered to an external channel that was suppressed by
// maintenance mode.
type Alert struct {
	database.BaseModel

	Name             string     `gorm:"column:name;size:128;not null;index" json:"name"`
	Severity         Severity   `gorm:"column:severity;size:16;not null;index" json:"severity"`
	State            State      `gorm:"column:state;size:16;not null;index;default:firing" json:"state"`
	SourceType       SourceType `gorm:"column:source_type;size:32;not null;index" json:"source_type"`
	SourceID         string     `gorm:"column:source_id;type:text;not null;index" json:"source_id"`
	Labels           JSONMap    `gorm:"column:labels;type:text" json:"labels,omitempty"`
	FiredAt          time.Time  `gorm:"column:fired_at;not null;index" json:"fired_at"`
	ResolvedAt       *time.Time `gorm:"column:resolved_at;index" json:"resolved_at,omitempty"`
	AcknowledgedBy   *string    `gorm:"column:acknowledged_by;type:text" json:"acknowledged_by,omitempty"`
	AcknowledgedAt   *time.Time `gorm:"column:acknowledged_at" json:"acknowledged_at,omitempty"`
	Suppressed       bool       `gorm:"column:suppressed;not null;default:false;index" json:"suppressed"`
	SuppressionReason string    `gorm:"column:suppression_reason;type:text" json:"suppression_reason,omitempty"`
}

// TableName pins the GORM-generated table name. Hard-coding
// keeps the SQL predictable for migrations and ad-hoc queries.
func (Alert) TableName() string { return "alerts" }

// Channel is a single notification destination. The Type selects
// the dispatcher; Config carries type-specific parameters
// (webhook URL, recipients, etc.). Sensitive keys in Config are
// masked by the handler layer before the channel is serialised
// back to the client.
type Channel struct {
	database.BaseModel

	Type    ChannelType `gorm:"column:type;size:32;not null;index" json:"type"`
	Config  JSONMap     `gorm:"column:config;type:text" json:"config,omitempty"`
	Enabled bool        `gorm:"column:enabled;not null;default:true;index" json:"enabled"`
}

// TableName pins the channels table.
func (Channel) TableName() string { return "channels" }

// AlertRule binds a ConditionDSL to a list of channel IDs. The
// service layer (Phase 6) will evaluate the DSL against incoming
// metric samples and fire alerts on the matched channels. For
// Phase 5 the rule is just a persisted record.
type AlertRule struct {
	database.BaseModel

	Name              string        `gorm:"column:name;size:128;not null;uniqueIndex" json:"name"`
	ConditionDSL      string        `gorm:"column:condition_dsl;type:text;not null" json:"condition_dsl"`
	ChannelIDs        StringList    `gorm:"column:channel_ids;type:text" json:"channel_ids"`
	Enabled           bool          `gorm:"column:enabled;not null;default:true;index" json:"enabled"`
	SuppressionWindow time.Duration `gorm:"column:suppression_window;not null;default:0" json:"suppression_window"`
}

// TableName pins the alert_rules table.
func (AlertRule) TableName() string { return "alert_rules" }

// AllModels returns every GORM model this package owns. The
// registration point in cmd/devops-toolkit/main.go (Phase 5) uses
// this so the model list does not have to be repeated.
func AllModels() []any {
	return []any{
		&Alert{},
		&Channel{},
		&AlertRule{},
	}
}

// IsNotFound reports whether err is (or wraps) a NotFound
// APIError, or the repository.ErrNotFound sentinel. Both
// surfaces are accepted so callers can use one helper for
// repository-level and service-level errors.
func IsNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var ae *contracts.APIError
	if errors.As(err, &ae) {
		return ae.Code == contracts.CodeNotFound
	}
	return false
}

// IsValidation reports whether err is (or wraps) a VALIDATION_ERROR
// APIError. Used by the service and handler tests.
func IsValidation(err error) bool {
	var ae *contracts.APIError
	if errors.As(err, &ae) {
		return ae.Code == contracts.CodeValidation
	}
	return false
}
