package auth

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
)

// Provider handles SMTP authentication
type Provider interface {
	Authenticate(client *smtp.Client, username, password string) error
}

// DefaultProvider implements different authentication strategies
type DefaultProvider struct {
	config *config.Config
}

// NewProvider creates a new authentication provider
func NewProvider(cfg *config.Config) Provider {
	return &DefaultProvider{config: cfg}
}

// Authenticate authenticates with the SMTP server using the appropriate method
func (p *DefaultProvider) Authenticate(client *smtp.Client, username, password string) error {
	// First, check if AUTH is supported
	if ok, _ := client.Extension("AUTH"); !ok {
		return fmt.Errorf("server doesn't support AUTH")
	}

	// Check if we should use LOGIN auth (for Office 365/Outlook)
	if p.config.Notifications.Email.UseLoginAuth {
		return p.performLoginAuth(client, username, password)
	}

	// Otherwise use PLAIN auth
	auth := smtp.PlainAuth("", username, password, p.config.Notifications.Email.SMTPServer)
	return client.Auth(auth)
}

// performLoginAuth implements LOGIN authentication for Office 365/Outlook
func (p *DefaultProvider) performLoginAuth(client *smtp.Client, username, password string) error {
	logger.Debug("Using LOGIN authentication method", logger.String("username", username))

	// Initiate AUTH LOGIN
	code, msg, err := command(client, "AUTH LOGIN")
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
	err := client.Text.PrintfLine(cmd)
	if err != nil {
		return 0, "", err
	}

	return client.Text.ReadResponse(0)
}
