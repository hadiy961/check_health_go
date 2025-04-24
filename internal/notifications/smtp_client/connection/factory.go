package connection

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"time"
)

// Config holds SMTP connection configuration
type Config struct {
	Server   string
	Port     int
	UseTLS   bool
	UseSSL   bool
	Timeout  time.Duration
	Sender   config.SenderEmail
	Username string
	Password string
}

// Factory creates and manages SMTP connections
type Factory interface {
	Connect(ctx context.Context, cfg Config) (*smtp.Client, net.Conn, error)
	Cleanup(client *smtp.Client, conn net.Conn)
}

// DefaultFactory implements the connection factory
type DefaultFactory struct {
	config *config.Config
}

// NewFactory creates a new connection factory
func NewFactory(cfg *config.Config) Factory {
	return &DefaultFactory{config: cfg}
}

// Connect establishes a connection to the SMTP server
func (f *DefaultFactory) Connect(ctx context.Context, cfg Config) (*smtp.Client, net.Conn, error) {
	smtpAddr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
	logger.Debug("Connecting to SMTP server",
		logger.String("address", smtpAddr),
		logger.Bool("useTLS", cfg.UseTLS),
		logger.Bool("useSSL", cfg.UseSSL))

	// Get hostname for HELO/EHLO
	hostname, _ := os.Hostname()

	// Connect to the SMTP server with timeout
	dialer := &net.Dialer{
		Timeout:   cfg.Timeout,
		DualStack: true,
		Resolver: &net.Resolver{
			PreferGo: true,
		},
		KeepAlive: 30 * time.Second,
	}

	// Create connection
	conn, err := dialer.DialContext(ctx, "tcp", smtpAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SMTP server: %w", err)
	}

	// Set a deadline for operations
	if err := conn.SetDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to set connection deadline: %w", err)
	}

	var client *smtp.Client

	// Handle SSL/TLS
	if cfg.UseSSL {
		tlsConn, client, err := f.setupSSLConnection(conn, cfg, hostname)
		if err != nil {
			conn.Close()
			return nil, nil, err
		}
		// For SSL connections, tlsConn becomes our primary connection
		return client, tlsConn, nil
	}

	// Create SMTP client
	client, err = smtp.NewClient(conn, cfg.Server)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to create SMTP client: %w", err)
	}

	// Send HELO
	if err = client.Hello(hostname); err != nil {
		client.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("HELO error: %w", err)
	}

	// Start TLS if enabled
	if cfg.UseTLS {
		if err := f.setupTLS(client, cfg); err != nil {
			client.Close()
			conn.Close()
			return nil, nil, err
		}
	}

	return client, conn, nil
}

// setupSSLConnection establishes a TLS connection from the beginning
func (f *DefaultFactory) setupSSLConnection(conn net.Conn, cfg Config, hostname string) (net.Conn, *smtp.Client, error) {
	tlsConfig := f.createTLSConfig(cfg.Server)

	tlsConn := tls.Client(conn, tlsConfig)

	// Perform handshake
	if err := tlsConn.HandshakeContext(context.Background()); err != nil {
		tlsConn.Close()
		return nil, nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Create SMTP client over TLS connection
	client, err := smtp.NewClient(tlsConn, cfg.Server)
	if err != nil {
		tlsConn.Close()
		return nil, nil, fmt.Errorf("failed to create SMTP client over SSL: %w", err)
	}

	// Send HELO
	if err = client.Hello(hostname); err != nil {
		client.Close()
		tlsConn.Close()
		return nil, nil, fmt.Errorf("HELO error in SSL mode: %w", err)
	}

	return tlsConn, client, nil
}

// setupTLS starts TLS on an existing connection
func (f *DefaultFactory) setupTLS(client *smtp.Client, cfg Config) error {
	// Check if server supports STARTTLS
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("server doesn't support STARTTLS")
	}

	tlsConfig := f.createTLSConfig(cfg.Server)

	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("StartTLS error: %w", err)
	}

	return nil
}

// createTLSConfig creates a TLS configuration
func (f *DefaultFactory) createTLSConfig(serverName string) *tls.Config {
	return &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		CipherSuites: func() []uint16 {
			var ids []uint16
			for _, suite := range tls.CipherSuites() {
				ids = append(ids, suite.ID)
			}
			return ids
		}(),
	}
}

// Cleanup properly closes SMTP client and connection
func (f *DefaultFactory) Cleanup(client *smtp.Client, conn net.Conn) {
	if client != nil {
		// Try to quit properly
		quitErr := client.Quit()
		if quitErr != nil {
			logger.Warn("Error during SMTP QUIT", logger.String("error", quitErr.Error()))
			client.Close()
		}
	}

	if conn != nil {
		conn.Close()
	}
}
