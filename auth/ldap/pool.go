package ldap

import (
	"container/list"
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/go-ldap/ldap/v3"
)

// ConnPool manages a pool of LDAP connections
type ConnPool struct {
	config   *Config
	conns    *list.List
	mu       sync.Mutex
	cond     *sync.Cond
	size     int
	maxSize  int
	waitCount int
	closed   bool
}

// NewConnPool creates a new connection pool
func NewConnPool(config *Config) (*ConnPool, error) {
	pool := &ConnPool{
		config:  config,
		conns:   list.New(),
		maxSize: config.PoolSize,
	}

	pool.cond = sync.NewCond(&pool.mu)

	// Initialize with a few connections
	for i := 0; i < 3; i++ {
		conn, err := pool.createConnection()
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to create initial connection: %w", err)
		}
		pool.conns.PushBack(conn)
		pool.size++
	}

	return pool, nil
}

// Get retrieves a connection from the pool
func (p *ConnPool) Get() (*ldap.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for p.conns.Len() == 0 && p.size >= p.maxSize {
		p.waitCount++
		p.cond.Wait()
		p.waitCount--
	}

	// Try to get an existing connection
	for e := p.conns.Front(); e != nil; e = e.Next() {
		conn := e.Value.(*ldap.Conn)
		if conn.IsClosing() {
			p.conns.Remove(e)
			p.size--
			continue
		}

		// Test the connection
		if err := conn.Bind(p.config.AdminDN, p.config.AdminPassword); err != nil {
			p.conns.Remove(e)
			p.size--
			continue
		}

		p.conns.Remove(e)
		return conn, nil
	}

	// Create a new connection if under max size
	if p.size < p.maxSize {
		conn, err := p.createConnection()
		if err != nil {
			return nil, err
		}
		p.size++
		return conn, nil
	}

	// Should not reach here, but wait and retry
	p.cond.Wait()
	return p.Get()
}

// Put returns a connection to the pool
func (p *ConnPool) Put(conn *ldap.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		conn.Close()
		return
	}

	if conn.IsClosing() {
		p.size--
		return
	}

	// Rebind with admin to reset connection state
	if err := conn.Bind(p.config.AdminDN, p.config.AdminPassword); err != nil {
		conn.Close()
		p.size--
		return
	}

	p.conns.PushBack(conn)
	p.cond.Signal()
}

// createConnection creates a new LDAP connection
func (p *ConnPool) createConnection() (*ldap.Conn, error) {
	var conn *ldap.Conn
	var err error

	// Parse the LDAP URL
	uri := p.config.URL

	if len(uri) > 8 && uri[:8] == "ldaps://" {
		// LDAPS connection
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		}
		conn, err = ldap.DialTLS("tcp", uri[8:], tlsConfig)
	} else {
		// LDAP connection
		addr := uri[7:] // Remove "ldap://"
		conn, err = ldap.Dial("tcp", addr)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	// Set timeouts
	conn.SetTimeout(p.config.ConnectTimeout)

	// Initial admin bind for connection testing
	if err := conn.Bind(p.config.AdminDN, p.config.AdminPassword); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to bind: %w", err)
	}

	return conn, nil
}

// Close closes all connections in the pool
func (p *ConnPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true

	var lastErr error
	for e := p.conns.Front(); e != nil; e = e.Next() {
		conn := e.Value.(*ldap.Conn)
		if err := conn.Close(); err != nil {
			lastErr = err
		}
	}
	p.conns.Init()
	p.size = 0

	return lastErr
}

// PoolStats returns pool statistics
type PoolStats struct {
	ActiveConns int
	WaitCount   int
	MaxConns    int
}

func (p *ConnPool) GetStats() *PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return &PoolStats{
		ActiveConns: p.size,
		WaitCount:   p.waitCount,
		MaxConns:    p.maxSize,
	}
}

// HealthCheck performs a health check on the pool
func (p *ConnPool) HealthCheck() error {
	conn, err := p.Get()
	if err != nil {
		return fmt.Errorf("LDAP pool health check failed: %w", err)
	}
	defer p.Put(conn)

	// Execute a simple search to verify connection
	searchRequest := ldap.NewSearchRequest(
		p.config.BaseDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		1, 0, false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)

	_, err = conn.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("LDAP pool health check search failed: %w", err)
	}

	return nil
}