package message

import (
	"CheckHealthDO/internal/pkg/config"
	"bytes"
	"fmt"
	"strings"
)

// Builder creates formatted email messages
type Builder interface {
	Build(sender config.SenderEmail, recipients []string, subject, body string) bytes.Buffer
}

// DefaultBuilder implements the message builder
type DefaultBuilder struct{}

// NewBuilder creates a new message builder
func NewBuilder() Builder {
	return &DefaultBuilder{}
}

// Build creates a properly formatted email message
func (b *DefaultBuilder) Build(sender config.SenderEmail, recipients []string, subject, body string) bytes.Buffer {
	var msg bytes.Buffer

	// Add From header with real name if provided
	fromHeader := sender.Email
	if sender.RealName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", sender.RealName, sender.Email)
	}

	msg.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(recipients, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	msg.WriteString(body)

	return msg
}
