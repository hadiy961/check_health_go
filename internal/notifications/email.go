package notifications

import (
	"CheckHealthDO/internal/notifications/mutt_client"
	"CheckHealthDO/internal/notifications/smtp_client"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"fmt"
	"strings"
)

// EmailClient defines the interface for different email provider implementations
type EmailClient interface {
	Send(sender config.SenderEmail, recipients []string, subject, body string) error
}

// EmailManager handles sending email notifications
type EmailManager struct {
	Config      *config.Config
	emailClient EmailClient
}

var initializedEmailManager *EmailManager

// NewEmailManager creates a new instance of EmailManager
func NewEmailManager(cfg *config.Config) *EmailManager {
	// Avoid reinitializing if already initialized
	if initializedEmailManager != nil {
		logger.Debug("Email manager already initialized, reusing existing instance")
		return initializedEmailManager
	}

	var client EmailClient

	// Log all email configuration for debugging
	logger.Debug("Initializing email manager",
		logger.String("provider", cfg.Notifications.Email.Provider),
		logger.String("smtp_server", cfg.Notifications.Email.SMTPServer),
		logger.Int("smtp_port", cfg.Notifications.Email.SMTPPort),
		logger.Bool("use_tls", cfg.Notifications.Email.UseTLS),
		logger.Bool("use_login_auth", cfg.Notifications.Email.UseLoginAuth))

	// Initialize the appropriate email client based on the configured provider
	provider := strings.ToLower(cfg.Notifications.Email.Provider)

	// Log the detected provider before the switch statement
	logger.Debug("Email provider detected", logger.String("provider", provider))

	switch provider {
	case "smtp":
		logger.Debug("Using SMTP native client for email")
		client = smtp_client.NewSMTPClient(cfg)
	case "mutt":
		logger.Debug("Using Mutt email client")
		client = mutt_client.NewMuttClient(cfg)
	default:
		logger.Debug("Unknown email provider specified, using SMTP as default",
			logger.String("provider", provider))
		client = smtp_client.NewSMTPClient(cfg)
	}

	emailMgr := &EmailManager{
		Config:      cfg,
		emailClient: client,
	}

	// Log that email manager is successfully initialized
	logger.Debug("Email manager initialized successfully")

	// Cache the initialized instance
	initializedEmailManager = emailMgr

	return emailMgr
}

// SendEmail sends an email with the given subject and body.
// The body parameter supports HTML content which will be properly rendered in email clients.
func (e *EmailManager) SendEmail(subject, body string) error {
	if !e.Config.Notifications.Email.Enabled {
		logger.Debug("Email notifications are disabled")
		return fmt.Errorf("email notifications are disabled")
	}

	// Check if email client is properly initialized
	if e.emailClient == nil {
		err := fmt.Errorf("email client is not initialized properly")
		logger.Error("Failed to send email", logger.String("error", err.Error()))
		return err
	}

	// Add dynamic app name to the subject
	appName := e.Config.AppName
	subject = fmt.Sprintf("[%s] %s", appName, subject)

	// Log that we're about to send an email
	logger.Debug("Preparing to send email",
		logger.String("subject", subject),
		logger.Int("body_length", len(body)))

	// Try sending with each configured sender
	for _, sender := range e.Config.Notifications.Email.SenderEmails {
		recipients := e.Config.Notifications.Email.RecipientEmails

		logger.Debug("Attempting to send email with sender",
			logger.String("sender", sender.Email),
			logger.Int("recipients", len(recipients)))

		err := e.emailClient.Send(sender, recipients, subject, body)
		if err == nil {
			logger.Debug("Email sent successfully", logger.String("sender", sender.Email))
			return nil
		}

		logger.Error("Failed to send email with sender",
			logger.String("sender", sender.Email),
			logger.String("error", err.Error()))
	}

	return fmt.Errorf("no valid email senders configured or all sending attempts failed")
}
