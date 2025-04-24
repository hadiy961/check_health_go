package smtp_client

import (
	"CheckHealthDO/internal/notifications/smtp_client/auth"
	"CheckHealthDO/internal/notifications/smtp_client/connection"
	"CheckHealthDO/internal/notifications/smtp_client/message"
	"CheckHealthDO/internal/notifications/smtp_client/retry"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"context"
	"fmt"
	"time"
)

// SMTPClient implements the EmailClient interface using SMTP
type SMTPClient struct {
	config         *config.Config
	connFactory    connection.Factory
	authProvider   auth.Provider
	messageBuilder message.Builder
	retryManager   retry.Manager
}

// NewSMTPClient creates a new SMTP client
func NewSMTPClient(cfg *config.Config) *SMTPClient {
	logger.Debug("Creating new SMTP client instance")
	return &SMTPClient{
		config:         cfg,
		connFactory:    connection.NewFactory(cfg),
		authProvider:   auth.NewProvider(cfg),
		messageBuilder: message.NewBuilder(),
		retryManager:   retry.NewManager(cfg),
	}
}

// Send sends an email using SMTP
func (c *SMTPClient) Send(sender config.SenderEmail, recipients []string, subject, body string) error {
	logger.Debug("SMTP client received send request",
		logger.String("from", sender.Email),
		logger.Int("recipients", len(recipients)))

	// Create message with headers
	msg := c.messageBuilder.Build(sender, recipients, subject, body)

	// Define the SMTP server address
	smtpConfig := connection.Config{
		Server:   c.config.Notifications.Email.SMTPServer,
		Port:     c.config.Notifications.Email.SMTPPort,
		UseTLS:   c.config.Notifications.Email.UseTLS,
		UseSSL:   c.config.Notifications.Email.UseSSL,
		Timeout:  time.Duration(c.config.Notifications.Email.Timeout) * time.Second,
		Sender:   sender,
		Username: sender.Email,
		Password: sender.Password,
	}

	// Create email request object
	emailReq := &retry.EmailRequest{
		Config:     smtpConfig,
		Message:    msg,
		Recipients: recipients,
		Sender:     sender,
	}

	// Run the send operation with retry
	return c.retryManager.Execute(context.Background(), emailReq, c.sendEmail)
}

// sendEmail handles the actual email sending logic
func (c *SMTPClient) sendEmail(ctx context.Context, req *retry.EmailRequest) error {
	logger.Debug("Starting email send operation",
		logger.String("from", req.Sender.Email),
		logger.String("server", req.Config.Server))

	// Establish connection
	client, conn, err := c.connFactory.Connect(ctx, req.Config)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer c.connFactory.Cleanup(client, conn)

	// Authenticate
	if err := c.authProvider.Authenticate(client, req.Sender.Email, req.Sender.Password); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Set sender
	if err := client.Mail(req.Sender.Email); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Add recipients
	for _, recipient := range req.Recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", recipient, err)
		}
	}

	// Send the email data
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start email data: %w", err)
	}

	if _, err = wc.Write(req.Message.Bytes()); err != nil {
		wc.Close()
		return fmt.Errorf("failed to write email content: %w", err)
	}

	if err = wc.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	logger.Debug("Email sent successfully")
	return nil
}
