package retry

import (
	"CheckHealthDO/internal/notifications/smtp_client/connection"
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"bytes"
	"context"
	"fmt"
	"time"
)

// EmailRequest contains all data needed to send an email
type EmailRequest struct {
	Config     connection.Config
	Message    bytes.Buffer
	Recipients []string
	Sender     config.SenderEmail
}

// SendFunc defines the function signature for sending emails
type SendFunc func(ctx context.Context, req *EmailRequest) error

// Manager handles retry logic for email sending
type Manager interface {
	Execute(ctx context.Context, req *EmailRequest, sendFunc SendFunc) error
}

// DefaultManager implements the retry manager
type DefaultManager struct {
	config *config.Config
}

// NewManager creates a new retry manager
func NewManager(cfg *config.Config) Manager {
	return &DefaultManager{config: cfg}
}

// Execute runs the email sending operation with retries
func (m *DefaultManager) Execute(ctx context.Context, req *EmailRequest, sendFunc SendFunc) error {
	retryCount := m.config.Notifications.Email.RetryCount
	retryInterval := time.Duration(m.config.Notifications.Email.RetryInterval) * time.Second

	var lastErr error

	for attempt := 0; attempt <= retryCount; attempt++ {
		if attempt > 0 {
			logger.Info("Retrying email sending",
				logger.Int("attempt", attempt),
				logger.Int("max_attempts", retryCount+1))
			time.Sleep(retryInterval)
		}

		// Create a timeout context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, req.Config.Timeout)

		// Try to send the email
		err := sendFunc(attemptCtx, req)
		cancel()

		if err == nil {
			logger.Info("Email sent successfully",
				logger.String("from", req.Sender.Email),
				logger.Int("recipients", len(req.Recipients)),
				logger.Int("attempt", attempt+1))
			return nil
		}

		lastErr = err
		logger.Error("Attempt to send email failed",
			logger.String("error", err.Error()),
			logger.Int("attempt", attempt+1),
			logger.Int("remaining_retries", retryCount-attempt))
	}

	logger.Error("All attempts to send email failed",
		logger.String("error", lastErr.Error()),
		logger.String("from", req.Sender.Email),
		logger.Int("attempts", retryCount+1))

	return fmt.Errorf("failed to send email after %d attempts: %v", retryCount+1, lastErr)
}
