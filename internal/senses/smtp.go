package senses

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// SMTP sender — uses Go stdlib net/smtp.
// ---------------------------------------------------------------------------

// smtpConfig holds SMTP connection parameters.
type smtpConfig struct {
	Host     string // e.g., "smtp.gmail.com:587"
	User     string
	Password string
	From     string
}

// smtpMessage represents an outgoing email.
type smtpMessage struct {
	To      string
	Subject string
	Body    string
	ReplyTo string
}

// sendSMTP sends an email via SMTP with STARTTLS.
func sendSMTP(cfg smtpConfig, msg smtpMessage) error {
	host, _, err := net.SplitHostPort(cfg.Host)
	if err != nil {
		host = cfg.Host
	}

	// Connect to SMTP server.
	conn, err := net.DialTimeout("tcp", cfg.Host, 30*time.Second)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	// STARTTLS if supported.
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{ServerName: host}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	// Authenticate if credentials provided.
	if cfg.User != "" && cfg.Password != "" {
		auth := smtp.PlainAuth("", cfg.User, cfg.Password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	// Set sender.
	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}

	// Set recipients.
	recipients := strings.Split(msg.To, ",")
	for _, rcpt := range recipients {
		rcpt = strings.TrimSpace(rcpt)
		if rcpt == "" {
			continue
		}
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", rcpt, err)
		}
	}

	// Write message body with headers.
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	headers := buildHeaders(cfg.From, msg)
	body := headers + "\r\n" + msg.Body + "\r\n"

	if _, err := wc.Write([]byte(body)); err != nil {
		wc.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}

	return client.Quit()
}

// sendSMTPDirect sends using a pre-connected smtp.Client (for testing with mock servers).
func sendSMTPDirect(client *smtp.Client, from string, msg smtpMessage) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}

	recipients := strings.Split(msg.To, ",")
	for _, rcpt := range recipients {
		rcpt = strings.TrimSpace(rcpt)
		if rcpt == "" {
			continue
		}
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", rcpt, err)
		}
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	headers := buildHeaders(from, msg)
	body := headers + "\r\n" + msg.Body + "\r\n"

	if _, err := wc.Write([]byte(body)); err != nil {
		wc.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	return wc.Close()
}

// buildHeaders constructs RFC 2822 email headers.
func buildHeaders(from string, msg smtpMessage) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("From: %s\r\n", from))
	b.WriteString(fmt.Sprintf("To: %s\r\n", msg.To))
	if msg.Subject != "" {
		b.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	}
	if msg.ReplyTo != "" {
		b.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", msg.ReplyTo))
	}
	b.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z)))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	return b.String()
}
