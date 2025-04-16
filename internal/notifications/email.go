package notifications

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"bytes"
	"fmt"
	"net/smtp"
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

	// Create the email message
	msg := bytes.Buffer{}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	msg.WriteString(body)

	// Send the email
	for _, sender := range e.Config.Notifications.Email.SenderEmails {
		auth := smtp.PlainAuth("", sender.Email, sender.Password, e.Config.Notifications.Email.SMTPServer)
		to := e.Config.Notifications.Email.RecipientEmails

		logger.Debugf("Sending email from %s to %v", sender.Email, to)
		err := smtp.SendMail(
			fmt.Sprintf("%s:%d", e.Config.Notifications.Email.SMTPServer, e.Config.Notifications.Email.SMTPPort),
			auth,
			sender.Email,
			to,
			msg.Bytes(),
		)
		if err != nil {
			logger.Error("Failed to send email",
				logger.String("error", err.Error()),
				logger.String("sender", sender.Email))
			return fmt.Errorf("failed to send email: %v", err)
		}
	}

	logger.Debug("Email sent successfully")
	return nil
}
