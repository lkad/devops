package auth

import (
	"fmt"
	"sync"
	"time"

	"github.com/devops-toolkit/pkg/config"
	"github.com/go-ldap/ldap/v3"
)

// PoolConfig holds configuration for the LDAP connection pool.
type PoolConfig struct {
	MaxConnections int
	MaxAge         time.Duration
}

// LDAPPool manages a pool of LDAP connections.
type LDAPPool struct {
	config    *config.LDAPConfig
	poolConfig PoolConfig
	pool       chan *ldap.Conn
	mu         sync.Mutex
}

// NewLDAPPool creates a new LDAP connection pool.
func NewLDAPPool(cfg *config.LDAPConfig, poolConfig PoolConfig) (*LDAPPool, error) {
	if poolConfig.MaxConnections <= 0 {
		poolConfig.MaxConnections = 5
	}
	if poolConfig.MaxAge <= 0 {
		poolConfig.MaxAge = 5 * time.Minute
	}

	pool := &LDAPPool{
		config:    cfg,
		poolConfig: poolConfig,
		pool:      make(chan *ldap.Conn, poolConfig.MaxConnections),
	}

	// Pre-populate the pool with connections
	for i := 0; i < poolConfig.MaxConnections; i++ {
		conn, err := pool.dial()
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to create initial connection: %w", err)
		}
		pool.pool <- conn
	}

	return pool, nil
}

// Get retrieves a connection from the pool.
func (p *LDAPPool) Get() (*ldap.Conn, error) {
	select {
	case conn := <-p.pool:
		if conn == nil || conn.IsClosing() {
			return p.dial()
		}
		return conn, nil
	default:
		return p.dial()
	}
}

// Put returns a connection to the pool.
func (p *LDAPPool) Put(conn *ldap.Conn) {
	if conn == nil || conn.IsClosing() {
		return
	}
	select {
	case p.pool <- conn:
	default:
		conn.Close()
	}
}

// Close closes all connections in the pool.
func (p *LDAPPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	close(p.pool)
	for conn := range p.pool {
		if conn != nil {
			conn.Close()
		}
	}
}

// dial creates a new LDAP connection.
func (p *LDAPPool) dial() (*ldap.Conn, error) {
	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", p.config.Host, p.config.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP: %w", err)
	}
	return conn, nil
}

// LDAPClient handles LDAP authentication with connection pooling and retry logic.
type LDAPClient struct {
	config *config.LDAPConfig
	pool   *LDAPPool
}

// NewLDAPClient creates a new LDAP client with connection pooling.
func NewLDAPClient(cfg *config.LDAPConfig) *LDAPClient {
	return &LDAPClient{config: cfg}
}

// NewLDAPClientWithPool creates a new LDAP client with an existing connection pool.
func NewLDAPClientWithPool(cfg *config.LDAPConfig, pool *LDAPPool) *LDAPClient {
	return &LDAPClient{config: cfg, pool: pool}
}

// SetPool sets the connection pool for the LDAP client.
func (c *LDAPClient) SetPool(pool *LDAPPool) {
	c.pool = pool
}

const (
	maxRetries     = 3
	baseRetryDelay = 100 * time.Millisecond
)

// isTransientError determines if an LDAP error is transient and worth retrying.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	if ldapErr, ok := err.(*ldap.Error); ok {
		switch ldapErr.ResultCode {
		case ldap.ErrorNetwork:
			return true
		}
	}
	return false
}

// Authenticate authenticates a user with retry logic and connection pooling.
func (c *LDAPClient) Authenticate(username, password string) (*User, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("LDAP is disabled")
	}

	var lastErr error
	var conn *ldap.Conn

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseRetryDelay * time.Duration(1<<(attempt-1)) // exponential backoff
			time.Sleep(delay)
		}

		// Use connection pool if available
		if c.pool != nil {
			conn, lastErr = c.pool.Get()
			if lastErr != nil {
				continue
			}
			defer c.pool.Put(conn)
		} else {
			// Fallback to direct dial
			conn, lastErr = ldap.Dial("tcp", fmt.Sprintf("%s:%d", c.config.Host, c.config.Port))
			if lastErr != nil {
				continue
			}
			defer conn.Close()
		}

		// Bind with user credentials
		userDN, err := c.getUserDN(conn, username)
		if err != nil {
			lastErr = fmt.Errorf("user not found: %w", err)
			continue
		}

		err = conn.Bind(userDN, password)
		if err != nil {
			lastErr = fmt.Errorf("authentication failed: %w", err)
			continue
		}

		// Get user groups
		groups, err := c.getUserGroups(conn, userDN)
		if err != nil {
			lastErr = fmt.Errorf("failed to get groups: %w", err)
			continue
		}

		// Map groups to roles
		roles := c.mapGroupsToRoles(groups)

		return &User{
			Username: username,
			Email:    username + "@example.com",
			LDAPDN:   userDN,
			Roles:    roles,
		}, nil
	}

	return nil, fmt.Errorf("authentication failed after %d attempts: %w", maxRetries, lastErr)
}

func (c *LDAPClient) getUserDN(conn *ldap.Conn, username string) (string, error) {
	searchRequest := ldap.NewSearchRequest(
		c.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf(c.config.UserFilter, ldap.EscapeFilter(username)),
		[]string{"dn"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return "", err
	}

	if len(result.Entries) == 0 {
		return "", fmt.Errorf("user not found")
	}

	return result.Entries[0].DN, nil
}

func (c *LDAPClient) getUserGroups(conn *ldap.Conn, userDN string) ([]string, error) {
	searchRequest := ldap.NewSearchRequest(
		userDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{"memberOf"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	if len(result.Entries) == 0 {
		return []string{}, nil
	}

	return result.Entries[0].GetAttributeValues("memberOf"), nil
}

func (c *LDAPClient) mapGroupsToRoles(groups []string) []string {
	rolesMap := make(map[string]bool)

	for _, group := range groups {
		if role, ok := c.config.GroupMapping[group]; ok {
			rolesMap[role] = true
		}
	}

	var roles []string
	for role := range rolesMap {
		roles = append(roles, role)
	}

	if len(roles) == 0 {
		roles = []string{"developer"} // default role
	}

	return roles
}

// HealthCheck tests LDAP server connectivity and returns status and reason
func (c *LDAPClient) HealthCheck() (bool, string) {
	if !c.config.Enabled {
		return false, "LDAP is disabled"
	}

	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", c.config.Host, c.config.Port))
	if err != nil {
		return false, fmt.Sprintf("failed to connect to LDAP server: %v", err)
	}
	defer conn.Close()

	// Try a simple bind with the configured bind DN if available
	if c.config.BindDN != "" && c.config.BindPassword != "" {
		err = conn.Bind(c.config.BindDN, c.config.BindPassword)
		if err != nil {
			return false, fmt.Sprintf("failed to bind to LDAP server: %v", err)
		}
	}

	return true, "LDAP server is reachable"
}
