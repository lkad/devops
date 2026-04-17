package ldap

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Role represents a system role mapped from LDAP groups
type Role string

const (
	RoleOperator   Role = "Operator"
	RoleDeveloper  Role = "Developer"
	RoleAuditor    Role = "Auditor"
	RoleSuperAdmin Role = "SuperAdmin"
)

// GroupRoleMapping maps LDAP group DNs to system roles
var GroupRoleMapping = map[string]string{
	"cn=it_ops,ou=groups,dc=example,dc=com":            "Operator",
	"cn=devteam_payments,ou=groups,dc=example,dc=com":  "Developer",
	"cn=security_auditors,ou=groups,dc=example,dc=com": "Auditor",
	"cn=sre_lead,ou=groups,dc=example,dc=com":           "SuperAdmin",
}

// Client is an LDAP client for authentication and group resolution
type Client struct {
	config      *Config
	pool        *ConnPool
	rateLimiter *RateLimiter
	auditLog    *AuditLogger
	mu          sync.RWMutex
}

// NewClient creates a new LDAP client with the given configuration
func NewClient(config *Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	pool, err := NewConnPool(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &Client{
		config:      config,
		pool:        pool,
		rateLimiter: NewRateLimiter(config.RateLimitPerSecond),
		auditLog:    NewAuditLogger(),
	}, nil
}

// Close closes the LDAP client and releases all connections
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pool != nil {
		return c.pool.Close()
	}
	return nil
}

// Authenticate attempts to bind with the given user DN and password
// Returns true if authentication succeeds, false otherwise
func (c *Client) Authenticate(userDN, password string) (bool, error) {
	// Rate limit check
	if !c.rateLimiter.Allow() {
		c.auditLog.LogAuthAttempt(userDN, false, "rate limited")
		return false, &AuthError{UserDN: userDN, Reason: "rate limited"}
	}

	// Get connection from pool
	conn, err := c.pool.Get()
	if err != nil {
		c.auditLog.LogAuthAttempt(userDN, false, fmt.Sprintf("pool error: %v", err))
		return false, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(conn)

	// Attempt to bind with user credentials
	err = conn.Bind(userDN, password)
	if err != nil {
		// Check if it's invalid credentials by examining error message
		if strings.Contains(err.Error(), "Invalid Credentials") {
			c.auditLog.LogAuthAttempt(userDN, false, "invalid credentials")
			return false, nil // Return false, not error, for invalid credentials
		}
		c.auditLog.LogAuthAttempt(userDN, false, fmt.Sprintf("bind error: %v", err))
		return false, fmt.Errorf("bind error: %w", err)
	}

	c.auditLog.LogAuthAttempt(userDN, true, "success")
	return true, nil
}

// GetGroups retrieves all LDAP groups for a given user DN
// Searches for groupOfNames entries where the user is a member
func (c *Client) GetGroups(userDN string) ([]string, error) {
	// Rate limit check
	if !c.rateLimiter.Allow() {
		return nil, &AuthError{UserDN: userDN, Reason: "rate limited"}
	}

	// Get connection from pool
	conn, err := c.pool.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(conn)

	// Search for groups containing the user
	searchBase := fmt.Sprintf("%s,%s", c.config.GroupSearchBase, c.config.BaseDN)
	filter := fmt.Sprintf("(member=%s)", ldap.EscapeFilter(userDN))

	searchRequest := ldap.NewSearchRequest(
		searchBase,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{"dn"},
		nil,
	)

	var result *ldap.SearchResult
	err = retryWithBackoff(c.config.MaxRetries, func() error {
		result, err = conn.Search(searchRequest)
		return err
	})
	if err != nil {
		c.auditLog.LogGroupLookup(userDN, nil, fmt.Sprintf("search error: %v", err))
		return nil, fmt.Errorf("group search error: %w", err)
	}

	groups := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		groups = append(groups, entry.DN)
	}

	c.auditLog.LogGroupLookup(userDN, groups, "success")
	return groups, nil
}

// GetRoles maps LDAP group DNs to internal system roles
// Returns unique slice of roles with no duplicates
func (c *Client) GetRoles(userDN string) ([]string, error) {
	groups, err := c.GetGroups(userDN)
	if err != nil {
		return nil, err
	}

	roleSet := make(map[string]bool)
	for _, groupDN := range groups {
		// Normalize the group DN for lookup
		normalizedDN := normalizeDN(groupDN)
		if role, ok := GroupRoleMapping[normalizedDN]; ok {
			roleSet[role] = true
		}
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}

	c.auditLog.LogRoleResolution(userDN, roles, "success")
	return roles, nil
}

// GetUserInfo retrieves user information from LDAP
type UserInfo struct {
	DN         string
	Username   string
	Email      string
	CommonName string
}

// GetUserInfo returns detailed user information
func (c *Client) GetUserInfo(userDN string) (*UserInfo, error) {
	conn, err := c.pool.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(conn)

	searchRequest := ldap.NewSearchRequest(
		userDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		1, 0, false,
		"(objectClass=*)",
		[]string{"cn", "mail", "uid"},
		nil,
	)

	var result *ldap.SearchResult
	err = retryWithBackoff(c.config.MaxRetries, func() error {
		result, err = conn.Search(searchRequest)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("user search error: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, &AuthError{UserDN: userDN, Reason: "user not found"}
	}

	entry := result.Entries[0]
	return &UserInfo{
		DN:         userDN,
		Username:   entry.GetAttributeValue("uid"),
		Email:      entry.GetAttributeValue("mail"),
		CommonName: entry.GetAttributeValue("cn"),
	}, nil
}

// Helper function to normalize DN for case-insensitive comparison
func normalizeDN(dn string) string {
	return strings.ToLower(strings.TrimSpace(dn))
}

// retryWithBackoff executes a function with exponential backoff retry
func retryWithBackoff(maxRetries int, fn func() error) error {
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		// Don't retry on validation errors
		if _, ok := err.(*AuthError); ok {
			return err
		}

		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
			time.Sleep(backoff)
		}
	}
	return err
}

// AuthError represents an authentication-related error
type AuthError struct {
	UserDN string
	Reason string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("auth error for %s: %s", e.UserDN, e.Reason)
}