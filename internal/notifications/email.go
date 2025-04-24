package notifications

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// EmailManager handles sending email notifications
type EmailManager struct {
	Config *config.Config
}

// NewEmailManager creates a new instance of EmailManager
func NewEmailManager(cfg *config.Config) *EmailManager {
	return &EmailManager{Config: cfg}
}

// SendEmail sends an email with the given subject and body.
// The body parameter supports HTML content which will be properly rendered in email clients.
func (e *EmailManager) SendEmail(subject, body string) error {
	if !e.Config.Notifications.Email.Enabled {
		logger.Debug("Email notifications are disabled")
		return fmt.Errorf("email notifications are disabled")
	}

	// Add dynamic app name to the subject
	appName := e.Config.AppName
	subject = fmt.Sprintf("[%s] %s", appName, subject)

	// Setup email configuration
	smtpServer := e.Config.Notifications.Email.SMTPServer
	smtpPort := e.Config.Notifications.Email.SMTPPort
	useTLS := e.Config.Notifications.Email.UseTLS
	useSSL := e.Config.Notifications.Email.UseSSL
	timeout := time.Duration(e.Config.Notifications.Email.Timeout) * time.Second
	retryCount := e.Config.Notifications.Email.RetryCount
	retryInterval := time.Duration(e.Config.Notifications.Email.RetryInterval) * time.Second

	// Send the email with retries
	for _, sender := range e.Config.Notifications.Email.SenderEmails {
		var lastErr error
		success := false

		// Define the SMTP server address
		smtpAddr := fmt.Sprintf("%s:%d", smtpServer, smtpPort)
		to := e.Config.Notifications.Email.RecipientEmails

		// Add From header with real name if provided
		fromHeader := sender.Email
		if sender.RealName != "" {
			fromHeader = fmt.Sprintf("%s <%s>", sender.RealName, sender.Email)
		}

		// Create message with From and To headers
		msg := bytes.Buffer{} // Reset buffer for each sender
		msg.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))
		msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", "))) // Add To header
		msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		msg.WriteString(body)

		// Try sending with retries
		for attempt := 0; attempt <= retryCount; attempt++ {
			if attempt > 0 {
				logger.Info("Retrying email sending",
					logger.Int("attempt", attempt),
					logger.Int("max_attempts", retryCount+1))
				time.Sleep(retryInterval)
			}

			logger.Info("Attempting to send email",
				logger.String("from", sender.Email),
				logger.String("smtp_server", smtpAddr),
				logger.Bool("use_tls", useTLS),
				logger.Bool("use_ssl", useSSL),
				logger.Int("attempt", attempt+1),
				logger.Int("max_attempts", retryCount+1))

			// Create context with timeout for the entire email sending operation
			ctx, cancel := context.WithTimeout(context.Background(), timeout)

			// Use a channel to track completion
			done := make(chan error, 1)

			// Run the email sending in a goroutine
			go func() {
				err := e.sendEmailWithTimeout(sender, to, smtpAddr, msg.Bytes(), useTLS, useSSL, timeout/2) // Half timeout for operation
				done <- err
			}()

			// Wait for either completion or timeout
			var err error
			select {
			case err = <-done:
				// Email send attempt completed
			case <-ctx.Done():
				err = fmt.Errorf("email sending timed out after %v seconds", timeout.Seconds())
				logger.Error("Email sending timed out",
					logger.String("smtp_server", smtpAddr),
					logger.Float64("timeout_seconds", timeout.Seconds()))
			}

			// Don't forget to cancel the context
			cancel()

			if err == nil {
				success = true
				logger.Info("Email sent successfully",
					logger.String("from", sender.Email),
					logger.Int("recipients", len(to)),
					logger.Int("attempt", attempt+1))
				break
			}

			lastErr = err
			logger.Error("Attempt to send email failed",
				logger.String("error", err.Error()),
				logger.Int("attempt", attempt+1),
				logger.Int("remaining_retries", retryCount-attempt))
		}

		if success {
			return nil
		}

		logger.Error("All attempts to send email failed",
			logger.String("error", lastErr.Error()),
			logger.String("from", sender.Email),
			logger.Int("attempts", retryCount+1))

		return fmt.Errorf("failed to send email after %d attempts: %v", retryCount+1, lastErr)
	}

	return fmt.Errorf("no valid email senders configured")
}

// sendEmailWithTimeout sends an email with the specified timeout
func (e *EmailManager) sendEmailWithTimeout(
	sender config.SenderEmail,
	recipients []string,
	smtpAddr string,
	message []byte,
	useTLS bool,
	useSSL bool,
	timeout time.Duration,
) error {
	logger.Debug("Email sending configuration",
		logger.String("smtp_server", smtpAddr),
		logger.String("sender", sender.Email),
		logger.Bool("useTLS", useTLS),
		logger.Bool("useSSL", useSSL),
		logger.Int("timeout_seconds", int(timeout.Seconds())))

	// Log system info for debugging
	hostname, _ := os.Hostname()
	logger.Debug("System information",
		logger.String("hostname", hostname),
		logger.String("os", "RHEL 9.4"))

	// Connect to the SMTP server with timeout
	dialer := &net.Dialer{
		Timeout:   timeout,
		DualStack: true, // Enable both IPv4 and IPv6
		Resolver: &net.Resolver{
			PreferGo: true, // Use Go's resolver which may be more reliable on some Linux distros
		},
		KeepAlive: 30 * time.Second, // Add Keep-Alive to maintain connection
	}

	// First try to resolve the hostname - sometimes RHEL has issues with DNS
	ips, err := net.LookupIP(e.Config.Notifications.Email.SMTPServer)
	if err != nil {
		logger.Warn("Failed to resolve SMTP server hostname, trying direct connection",
			logger.String("server", e.Config.Notifications.Email.SMTPServer),
			logger.String("error", err.Error()))
	} else {
		logger.Debug("Resolved SMTP server IP addresses",
			logger.String("server", e.Config.Notifications.Email.SMTPServer),
			logger.String("ip_addresses", fmt.Sprintf("%v", ips)))
	}

	// Use a much longer initial connection timeout - RHEL may have stricter network policies
	initialTimeout := timeout * 3
	logger.Debug("Connecting to SMTP server with extended timeout",
		logger.String("address", smtpAddr),
		logger.Float64("timeout_seconds", initialTimeout.Seconds()))

	dialer.Timeout = initialTimeout
	conn, err := dialer.Dial("tcp", smtpAddr)
	if err != nil {
		// Try one more time with even longer timeout
		logger.Warn("First connection attempt failed, trying with longer timeout",
			logger.String("error", err.Error()))

		// RHEL firewall might be throttling initial connections - try longer timeout
		dialer.Timeout = timeout * 5
		conn, err = dialer.Dial("tcp", smtpAddr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server %s after multiple attempts: %v", smtpAddr, err)
		}
	}

	logger.Debug("Connected to SMTP server", logger.String("address", smtpAddr))

	// Very generous connection deadline - some SMTP servers can be slow
	if err := conn.SetDeadline(time.Now().Add(timeout * 5)); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set connection deadline: %v", err)
	}

	var client *smtp.Client

	// Establish SMTP client
	if useSSL {
		// For SSL, wrap connection with TLS immediately
		tlsConfig := &tls.Config{
			ServerName:         e.Config.Notifications.Email.SMTPServer,
			InsecureSkipVerify: false, // Set to true for debugging only
			MinVersion:         tls.VersionTLS12,
			// Add additional cipher suites for broader compatibility
			CipherSuites: func() []uint16 {
				var ids []uint16
				for _, suite := range tls.CipherSuites() {
					ids = append(ids, suite.ID)
				}
				return ids
			}(),
		}

		tlsConn := tls.Client(conn, tlsConfig)

		// Set a very generous deadline for TLS handshake
		if err := tlsConn.SetDeadline(time.Now().Add(timeout * 5)); err != nil {
			tlsConn.Close()
			return fmt.Errorf("failed to set TLS connection deadline: %v", err)
		}

		logger.Debug("Performing TLS handshake with extended timeout")

		// Create a context with timeout specifically for the handshake
		handshakeCtx, cancel := context.WithTimeout(context.Background(), timeout*3)
		defer cancel()

		// Use a channel to handle handshake completion
		handshakeDone := make(chan error, 1)
		go func() {
			handshakeDone <- tlsConn.HandshakeContext(handshakeCtx)
		}()

		// Wait for handshake completion or timeout
		select {
		case err := <-handshakeDone:
			if err != nil {
				tlsConn.Close()
				conn.Close()
				logger.Error("TLS handshake failed", logger.String("error", err.Error()))
				return fmt.Errorf("TLS handshake failed: %v", err)
			}
			logger.Debug("TLS handshake completed successfully")
		case <-time.After(timeout * 3):
			tlsConn.Close()
			conn.Close()
			return fmt.Errorf("TLS handshake timed out after %v seconds", timeout*3/time.Second)
		}

		client, err = smtp.NewClient(tlsConn, e.Config.Notifications.Email.SMTPServer)
		if err != nil {
			tlsConn.Close()
			return fmt.Errorf("failed to create SMTP client over SSL: %v", err)
		}
	} else {
		client, err = smtp.NewClient(conn, e.Config.Notifications.Email.SMTPServer)
		if err != nil {
			conn.Close()
			return fmt.Errorf("failed to create SMTP client: %v", err)
		}
	}

	// Create a cleanup function to ensure resources are released
	cleanup := func() {
		// First try QUIT properly
		quitErr := client.Quit()
		if quitErr != nil {
			logger.Warn("Error during SMTP QUIT", logger.String("error", quitErr.Error()))
			// If QUIT fails, force close the client
			closeErr := client.Close()
			if closeErr != nil {
				logger.Warn("Error closing SMTP client", logger.String("error", closeErr.Error()))
			}
		}

		// Always try to close the underlying connection as well
		if !useSSL {
			closeErr := conn.Close()
			if closeErr != nil {
				logger.Warn("Error closing TCP connection", logger.String("error", closeErr.Error()))
			}
		}
	}
	defer cleanup()

	// Send EHLO command with FQDN hostname to avoid RHEL-specific issues
	logger.Debug("Sending HELO command")
	if err = client.Hello(hostname); err != nil {
		return fmt.Errorf("EHLO error: %v", err)
	}

	// Check if server supports AUTH
	supported, _ := client.Extension("AUTH")
	logger.Debug("Server AUTH support", logger.Bool("supported", supported))

	// Start TLS if not using SSL and TLS is enabled
	if useTLS && !useSSL {
		// Check if server supports STARTTLS
		supported, _ := client.Extension("STARTTLS")
		logger.Debug("Server STARTTLS support", logger.Bool("supported", supported))

		if !supported {
			return fmt.Errorf("server doesn't support STARTTLS, TLS is required")
		}

		tlsConfig := &tls.Config{
			ServerName:         e.Config.Notifications.Email.SMTPServer,
			InsecureSkipVerify: false, // Set to true only for debugging
			MinVersion:         tls.VersionTLS12,
			// Add cipher suites for broader compatibility
			CipherSuites: func() []uint16 {
				var ids []uint16
				for _, suite := range tls.CipherSuites() {
					ids = append(ids, suite.ID)
				}
				return ids
			}(),
		}

		logger.Debug("Starting TLS handshake with server",
			logger.String("server_name", tlsConfig.ServerName))

		// Use a channel to handle TLS start completion
		startTLSDone := make(chan error, 1)
		go func() {
			startTLSDone <- client.StartTLS(tlsConfig)
		}()

		// Wait for TLS start completion or timeout
		select {
		case err := <-startTLSDone:
			if err != nil {
				logger.Error("TLS handshake failed", logger.String("error", err.Error()))
				return fmt.Errorf("StartTLS error: %v", err)
			}
			logger.Debug("TLS handshake completed successfully")
		case <-time.After(timeout * 3):
			return fmt.Errorf("StartTLS handshake timed out after %v seconds", timeout*3/time.Second)
		}
	}

	// Custom LOGIN authentication for Office 365
	logger.Debug("Authenticating")
	if err = performLoginAuth(client, sender.Email, sender.Password); err != nil {
		return fmt.Errorf("authentication failed: %v (make sure your email/password are correct)", err)
	}
	logger.Debug("Authentication successful")

	// Set sender (use email address only for MAIL FROM command)
	logger.Debug("Setting sender", logger.String("sender", sender.Email))
	if err = client.Mail(sender.Email); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	// Add recipients
	for _, recipient := range recipients {
		logger.Debug("Adding recipient", logger.String("recipient", recipient))
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to add recipient %s: %v", recipient, err)
		}
	}

	// Send the email data
	logger.Debug("Starting data phase")
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start email data: %v", err)
	}

	logger.Debug("Writing email content")
	_, err = wc.Write(message)
	if err != nil {
		wc.Close()
		return fmt.Errorf("failed to write email content: %v", err)
	}

	// Close the writer
	logger.Debug("Closing data writer")
	if err = wc.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %v", err)
	}

	// Quit the connection
	logger.Debug("Quitting SMTP connection")
	if err = client.Quit(); err != nil {
		logger.Warn("Failed to quit SMTP connection properly, but email might have been sent",
			logger.String("error", err.Error()))
	}

	logger.Debug("Email sent successfully")
	return nil
}

// performLoginAuth implements LOGIN authentication for Office 365/Outlook
// This is needed because the standard smtp.PlainAuth doesn't always work with Outlook
func performLoginAuth(client *smtp.Client, username, password string) error {
	// First, check if AUTH is supported
	if ok, _ := client.Extension("AUTH"); !ok {
		return fmt.Errorf("server doesn't support AUTH")
	}

	// Use the Auth method with a custom implementation of smtp.Auth
	return authOutlook(client, username, password)
}

// CustomLoginAuth implements the smtp.Auth interface for LOGIN authentication
type CustomLoginAuth struct {
	username, password string
}

func (a *CustomLoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *CustomLoginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			// The server may send base64 encoded challenges
			// Try to decode and match expected patterns
			decoded, err := base64.StdEncoding.DecodeString(string(fromServer))
			if err == nil {
				lowerChallenge := strings.ToLower(string(decoded))
				if strings.Contains(lowerChallenge, "username") {
					return []byte(a.username), nil
				} else if strings.Contains(lowerChallenge, "password") {
					return []byte(a.password), nil
				}
			}
			return nil, fmt.Errorf("unexpected server challenge: %s", fromServer)
		}
	}
	return nil, nil
}

// authOutlook is a custom function to handle Outlook's LOGIN authentication
func authOutlook(client *smtp.Client, username, password string) error {
	// For Office 365, we'll use a direct command approach
	// First, initiate AUTH LOGIN
	cmd := "AUTH LOGIN"
	code, msg, err := command(client, cmd)
	if err != nil {
		return fmt.Errorf("AUTH command failed: %v", err)
	}
	if code != 334 {
		return fmt.Errorf("expected 334 response to AUTH LOGIN, got %d: %s", code, msg)
	}

	// Send username in base64
	usernameB64 := base64.StdEncoding.EncodeToString([]byte(username))
	code, msg, err = command(client, usernameB64)
	if err != nil {
		return fmt.Errorf("sending username failed: %v", err)
	}
	if code != 334 {
		return fmt.Errorf("username rejected with code %d: %s", code, msg)
	}

	// Send password in base64
	passwordB64 := base64.StdEncoding.EncodeToString([]byte(password))
	code, msg, err = command(client, passwordB64)
	if err != nil {
		if strings.Contains(err.Error(), "535") {
			return fmt.Errorf("authentication failed - incorrect username or password")
		}
		return fmt.Errorf("sending password failed: %v", err)
	}
	if code != 235 {
		return fmt.Errorf("authentication failed with code %d: %s", code, msg)
	}

	return nil
}

// command sends a raw SMTP command and returns the response
func command(client *smtp.Client, cmd string) (int, string, error) {
	// We need to use a custom Cmd method because we're implementing
	// a custom authentication flow
	err := client.Text.PrintfLine(cmd)
	if err != nil {
		return 0, "", err
	}

	return client.Text.ReadResponse(0)
}
