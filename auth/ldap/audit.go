package ldap

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// AuditLogger logs authentication and authorization events
type AuditLogger struct {
	mu      sync.Mutex
	logger  *log.Logger
	enabled bool
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger() *AuditLogger {
	return &AuditLogger{
		logger:  log.New(os.Stdout, "[AUDIT] ", log.LstdFlags),
		enabled: true,
	}
}

// LogAuthAttempt logs an authentication attempt
func (al *AuditLogger) LogAuthAttempt(userDN string, success bool, reason string) {
	if !al.enabled {
		return
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	status := "FAIL"
	if success {
		status = "SUCCESS"
	}

	al.logger.Printf("%s | user=%s | auth=%s | reason=%s",
		status, userDN, time.Now().Format(time.RFC3339), reason)
}

// LogGroupLookup logs a group lookup operation
func (al *AuditLogger) LogGroupLookup(userDN string, groups []string, status string) {
	if !al.enabled {
		return
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	al.logger.Printf("GROUP_LOOKUP | user=%s | groups=%v | status=%s | time=%s",
		userDN, groups, status, time.Now().Format(time.RFC3339))
}

// LogRoleResolution logs a role resolution operation
func (al *AuditLogger) LogRoleResolution(userDN string, roles []string, status string) {
	if !al.enabled {
		return
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	al.logger.Printf("ROLE_RESOLVE | user=%s | roles=%v | status=%s | time=%s",
		userDN, roles, status, time.Now().Format(time.RFC3339))
}

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	Timestamp time.Time
	EventType string
	UserDN    string
	Success   bool
	Details   string
}

// GetRecentEntries returns recent audit entries (for debugging/testing)
func (al *AuditLogger) GetRecentEntries() []AuditEntry {
	// This would typically read from a buffer or database
	// For now, return empty slice
	return []AuditEntry{}
}

// Disable disables audit logging
func (al *AuditLogger) Disable() {
	al.enabled = false
}

// Enable enables audit logging
func (al *AuditLogger) Enable() {
	al.enabled = true
}

// FormatAuditEntry formats an audit entry as a string
func FormatAuditEntry(entry AuditEntry) string {
	return fmt.Sprintf("[%s] %s | user=%s | success=%v | %s",
		entry.Timestamp.Format(time.RFC3339),
		entry.EventType,
		entry.UserDN,
		entry.Success,
		entry.Details,
	)
}