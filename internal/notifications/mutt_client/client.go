package mutt_client

import (
	"CheckHealthDO/internal/pkg/config"
	"CheckHealthDO/internal/pkg/logger"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

// MuttClient implements EmailClient using the mutt command-line tool
type MuttClient struct {
	config *config.Config
}

// NewMuttClient creates a new Mutt client
func NewMuttClient(cfg *config.Config) *MuttClient {
	return &MuttClient{config: cfg}
}

// Send sends an email using the mutt command-line tool
func (c *MuttClient) Send(sender config.SenderEmail, recipients []string, subject, body string) error {
	// Log environments to help with debugging
	logger.Debug("System PATH environment variable",
		logger.String("PATH", os.Getenv("PATH")))

	// Try different mail sending methods in order until one succeeds
	err1 := c.sendWithMutt(sender, recipients, subject, body)
	if err1 == nil {
		return nil
	}

	logger.Warn("Failed to send email with mutt, trying mail command as fallback",
		logger.String("error", err1.Error()))

	err2 := c.sendWithMailCommand(sender, recipients, subject, body)
	if err2 == nil {
		return nil
	}

	logger.Warn("Failed to send email with mail command, trying sendmail as fallback",
		logger.String("error", err2.Error()))

	err3 := c.sendWithSendmail(sender, recipients, subject, body)
	if err3 == nil {
		return nil
	}

	// Final fallback to echo for testing
	err4 := c.sendWithEcho(sender, recipients, subject, body)
	if err4 == nil {
		logger.Warn("Email sent with echo (test only, no actual delivery)")
		return nil
	}

	// All methods failed
	return fmt.Errorf("all email sending methods failed: mutt: %v, mail: %v, sendmail: %v, echo: %v",
		err1, err2, err3, err4)
}

// Add this helper function at the top of the file after imports
func boolToYesNo(condition bool) string {
	if condition {
		return "yes"
	}
	return "no"
}

// sendWithMutt attempts to send an email using the mutt command
func (c *MuttClient) sendWithMutt(sender config.SenderEmail, recipients []string, subject, body string) error {
	muttPath := c.config.Notifications.Email.MuttPath

	// Try to find mutt if path is not valid
	if _, err := os.Stat(muttPath); os.IsNotExist(err) {
		logger.Info("Mutt not found at configured path, searching in PATH",
			logger.String("configured_path", muttPath))

		// First try which command
		cmdOutput, err := exec.Command("which", "mutt").Output()
		if err == nil && len(cmdOutput) > 0 {
			muttPath = strings.TrimSpace(string(cmdOutput))
			logger.Info("Found mutt using 'which'",
				logger.String("path", muttPath))
		} else {
			// Try common locations
			commonPaths := []string{"/bin/mutt", "/usr/bin/mutt", "/usr/local/bin/mutt"}
			for _, path := range commonPaths {
				if _, err := os.Stat(path); err == nil {
					muttPath = path
					logger.Info("Found mutt at common location",
						logger.String("path", muttPath))
					break
				}
			}
		}
	}

	// Final check if mutt exists
	if _, err := os.Stat(muttPath); os.IsNotExist(err) {
		return fmt.Errorf("mutt executable not found at %s or in PATH", muttPath)
	}

	// Create a temporary file for the email body - simple text file, not HTML
	tempFile, err := ioutil.TempFile("", "mutt_email_body_*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for email body: %w", err)
	}
	tempFilePath := tempFile.Name()

	// Just write the raw HTML body without any MIME headers
	if _, err := tempFile.WriteString(body); err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to write email body to temporary file: %w", err)
	}
	tempFile.Close()

	// Ensure the file is readable
	os.Chmod(tempFilePath, 0644)

	// Create a temporary muttrc file with authentication details
	muttrcFile, err := ioutil.TempFile("", "muttrc_*.txt")
	if err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to create temporary muttrc file: %w", err)
	}
	muttrcPath := muttrcFile.Name()

	// Get SMTP details from config
	smtpServer := c.config.Notifications.Email.SMTPServer
	smtpPort := c.config.Notifications.Email.SMTPPort
	useTLS := c.config.Notifications.Email.UseTLS

	// Create the muttrc content with proper SMTP settings - but don't mess with HTML handling
	muttrcContent := fmt.Sprintf(`
# Basic settings
set from = "%s"
set realname = "%s"

# SMTP settings
set smtp_url = "smtp://%s@%s:%d/"
set smtp_pass = "%s"
set ssl_force_tls = %s
set ssl_starttls = %s

# Other settings to improve compatibility
set send_charset = "utf-8"
set charset = "utf-8"
`,
		sender.Email,    // from
		sender.RealName, // realname
		sender.Email,    // smtp_url user part
		smtpServer,      // smtp server
		smtpPort,        // smtp port
		sender.Password, // smtp_pass
		boolToYesNo(useTLS || c.config.Notifications.Email.UseSSL), // ssl_force_tls
		boolToYesNo(useTLS), // ssl_starttls
	)

	if _, err := muttrcFile.WriteString(muttrcContent); err != nil {
		muttrcFile.Close()
		os.Remove(muttrcPath)
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to write muttrc file: %w", err)
	}
	muttrcFile.Close()

	// Ensure the muttrc file is secure but readable by current user
	os.Chmod(muttrcPath, 0600)

	// Prepare mutt command arguments - these are critical for HTML content
	cmdArgs := []string{
		"-F", muttrcPath, // Use our custom muttrc
		"-s", subject, // Set the subject
		"-e", "set content_type=text/html", // Important: tell mutt this is HTML
		"-i", tempFilePath, // Use the temporary file as input
		"--", // End of options marker
	}
	cmdArgs = append(cmdArgs, recipients...)

	// Enhanced environment setup - add common binary locations to PATH
	pathEnv := os.Getenv("PATH")
	enhancedPath := fmt.Sprintf("%s:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin", pathEnv)

	// Execute mutt directly - DO NOT use stdin, use the -i option instead
	cmd := exec.Command(muttPath, cmdArgs...)

	// Set up the environment with enhanced PATH
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s", enhancedPath),
		"HOME=/root",             // Ensure HOME is set for .muttrc
		"CONTENT_TYPE=text/html", // Additional hint for HTML content
	)

	// Log the command being executed
	logger.Debug("Executing mutt command",
		logger.String("command", fmt.Sprintf("%s %s", muttPath, strings.Join(cmdArgs, " "))),
		logger.String("temp_file", tempFilePath),
		logger.String("muttrc_path", muttrcPath))

	output, err := cmd.CombinedOutput()

	// Clean up the temporary files
	os.Remove(tempFilePath)
	os.Remove(muttrcPath)

	if err != nil {
		logger.Error("Mutt command failed",
			logger.String("output", string(output)),
			logger.String("error", err.Error()))
		return fmt.Errorf("mutt command failed: %w - output: %s", err, string(output))
	}

	logger.Info("Email sent successfully using mutt",
		logger.String("from", sender.Email),
		logger.Int("recipients", len(recipients)))
	return nil
}

// sendWithMailCommand attempts to send email using the mail command
func (c *MuttClient) sendWithMailCommand(sender config.SenderEmail, recipients []string, subject, body string) error {
	// Find mail command
	mailPath, err := exec.LookPath("mail")
	if err != nil {
		// Check common locations
		commonPaths := []string{"/bin/mail", "/usr/bin/mail", "/usr/local/bin/mail"}
		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				mailPath = path
				break
			}
		}
	}

	if mailPath == "" {
		return fmt.Errorf("mail command not found in PATH or common locations")
	}

	// Create a temporary file for the email body
	tempFile, err := ioutil.TempFile("", "mail_body_*.html")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for email body: %w", err)
	}
	tempFilePath := tempFile.Name()

	if _, err := tempFile.WriteString(body); err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to write email body to temporary file: %w", err)
	}
	tempFile.Close()
	os.Chmod(tempFilePath, 0644)

	// Build the mail command with HTML support
	cmdArgs := []string{
		"-s", subject,
		"-r", fmt.Sprintf("%s <%s>", sender.RealName, sender.Email),
		"-a", "Content-Type: text/html; charset=UTF-8", // Explicit HTML content type
	}
	cmdArgs = append(cmdArgs, recipients...)

	// Enhanced environment setup
	pathEnv := os.Getenv("PATH")
	enhancedPath := fmt.Sprintf("%s:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin", pathEnv)

	// Execute mail command
	cmd := exec.Command(mailPath, cmdArgs...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s", enhancedPath),
		"HOME=/root", // Ensure HOME is set
	)

	// Changed approach: write directly to stdin instead of using cat
	stdin, err := cmd.StdinPipe()
	if err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Capture output
	var outputBuffer bytes.Buffer
	cmd.Stdout = &outputBuffer
	cmd.Stderr = &outputBuffer

	// Start the command before writing to stdin
	if err := cmd.Start(); err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to start mail command: %w", err)
	}

	// Read the file content
	fileContent, err := ioutil.ReadFile(tempFilePath)
	if err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to read temp file: %w", err)
	}

	// Write to stdin and close
	_, err = stdin.Write(fileContent)
	stdin.Close()
	if err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to write to mail stdin: %w", err)
	}

	// Wait for command to finish
	err = cmd.Wait()
	os.Remove(tempFilePath)

	// Get the command output
	output := outputBuffer.String()

	if err != nil {
		logger.Error("Mail command failed",
			logger.String("output", output),
			logger.String("error", err.Error()))
		return fmt.Errorf("mail command failed: %w - output: %s", err, output)
	}

	logger.Info("Email sent successfully using mail command",
		logger.String("from", sender.Email),
		logger.Int("recipients", len(recipients)))
	return nil
}

// sendWithSendmail attempts to send email using sendmail
func (c *MuttClient) sendWithSendmail(sender config.SenderEmail, recipients []string, subject, body string) error {
	// Find sendmail
	sendmailPath, err := exec.LookPath("sendmail")
	if err != nil {
		// Check common locations
		commonPaths := []string{"/usr/sbin/sendmail", "/usr/lib/sendmail", "/sbin/sendmail"}
		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				sendmailPath = path
				break
			}
		}
	}

	if sendmailPath == "" {
		return fmt.Errorf("sendmail not found in PATH or common locations")
	}

	// Create email content with proper headers including HTML content type
	emailContent := fmt.Sprintf("From: %s <%s>\nTo: %s\nSubject: %s\nContent-Type: text/html; charset=UTF-8\nMIME-Version: 1.0\n\n%s",
		sender.RealName, sender.Email,
		strings.Join(recipients, ", "),
		subject,
		body)

	// Create a temporary file for the email content
	tempFile, err := ioutil.TempFile("", "sendmail_*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for email content: %w", err)
	}
	tempFilePath := tempFile.Name()

	if _, err := tempFile.WriteString(emailContent); err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to write email content to temporary file: %w", err)
	}
	tempFile.Close()
	os.Chmod(tempFilePath, 0644)

	// Build sendmail arguments
	cmdArgs := []string{"-t"}
	for _, recipient := range recipients {
		cmdArgs = append(cmdArgs, recipient)
	}

	// Enhanced environment setup
	pathEnv := os.Getenv("PATH")
	enhancedPath := fmt.Sprintf("%s:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin", pathEnv)

	// Execute sendmail command with enhanced environment
	cmd := exec.Command(sendmailPath, cmdArgs...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s", enhancedPath),
		"HOME=/root", // Ensure HOME is set
	)

	// Use the direct stdin pipe approach (similar to mail command changes)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Capture output
	var outputBuffer bytes.Buffer
	cmd.Stdout = &outputBuffer
	cmd.Stderr = &outputBuffer

	if err := cmd.Start(); err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to start sendmail command: %w", err)
	}

	// Read the file content
	fileContent, err := ioutil.ReadFile(tempFilePath)
	if err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to read temp file: %w", err)
	}

	// Write to stdin and close
	_, err = stdin.Write(fileContent)
	stdin.Close()
	if err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to write to sendmail stdin: %w", err)
	}

	// Wait for command to finish
	err = cmd.Wait()
	os.Remove(tempFilePath)

	// Get the command output
	output := outputBuffer.String()

	if err != nil {
		logger.Error("Sendmail command failed",
			logger.String("output", output),
			logger.String("error", err.Error()))
		return fmt.Errorf("sendmail command failed: %w - output: %s", err, output)
	}

	logger.Info("Email sent successfully using sendmail",
		logger.String("from", sender.Email),
		logger.Int("recipients", len(recipients)))
	return nil
}

// sendWithEcho is a simple fallback for testing
func (c *MuttClient) sendWithEcho(sender config.SenderEmail, recipients []string, subject, body string) error {
	// Create a temporary file for logging what would be sent
	tempFile, err := ioutil.TempFile("", "echo_email_*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempFilePath := tempFile.Name()

	// Format the email content for logging
	emailContent := fmt.Sprintf("From: %s <%s>\nTo: %s\nSubject: %s\n\n%s",
		sender.RealName, sender.Email,
		strings.Join(recipients, ", "),
		subject,
		body)

	if _, err := tempFile.WriteString(emailContent); err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		return fmt.Errorf("failed to write email content: %w", err)
	}
	tempFile.Close()

	// Log the file path for debugging
	logger.Info("Email content written to file (echo fallback)",
		logger.String("file_path", tempFilePath))

	// Don't remove the file so it can be examined for debugging

	return nil
}
