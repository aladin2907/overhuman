package senses

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// EmailConfig holds email adapter configuration.
type EmailConfig struct {
	// IMAP settings (receiving).
	IMAPServer string `json:"imap_server"` // e.g., "imap.gmail.com:993"
	IMAPUser   string `json:"imap_user"`
	IMAPPass   string `json:"imap_pass"`
	IMAPTLS    bool   `json:"imap_tls"` // Use TLS (default: true for port 993)

	// SMTP settings (sending).
	SMTPServer string `json:"smtp_server"` // e.g., "smtp.gmail.com:587"
	SMTPUser   string `json:"smtp_user"`
	SMTPPass   string `json:"smtp_pass"`
	FromAddr   string `json:"from_addr"`

	// Polling interval for IMAP.
	PollInterval time.Duration `json:"poll_interval"`

	// Filters.
	AllowedSenders []string `json:"allowed_senders"` // Whitelist (empty = allow all)
	FolderName     string   `json:"folder_name"`     // IMAP folder to watch (default: INBOX)

	// TLS configuration (optional, for testing).
	TLSConfig *tls.Config `json:"-"`

	// DialFunc overrides the default IMAP dial for testing.
	// If set, called instead of dialIMAP/dialIMAPPlain.
	DialFunc func(addr string) (*imapClient, error) `json:"-"`

	// SMTPSendFunc overrides the default SMTP send for testing.
	SMTPSendFunc func(cfg smtpConfig, msg smtpMessage) error `json:"-"`
}

// EmailSense receives messages via IMAP and sends via SMTP.
type EmailSense struct {
	config  EmailConfig
	mu      sync.Mutex
	stopped bool
	cancel  context.CancelFunc
	logger  *slog.Logger
}

// NewEmailSense creates an email adapter.
func NewEmailSense(config EmailConfig) *EmailSense {
	if config.PollInterval == 0 {
		config.PollInterval = 60 * time.Second
	}
	if config.FolderName == "" {
		config.FolderName = "INBOX"
	}
	// Default to TLS for standard IMAP port.
	if !config.IMAPTLS && strings.HasSuffix(config.IMAPServer, ":993") {
		config.IMAPTLS = true
	}
	return &EmailSense{
		config: config,
		logger: slog.Default(),
	}
}

func (s *EmailSense) Name() string { return "Email" }

// Start begins polling IMAP for new messages.
func (s *EmailSense) Start(ctx context.Context, out chan<- *UnifiedInput) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return fmt.Errorf("email sense already stopped")
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	return s.pollIMAP(ctx, out)
}

// pollIMAP periodically checks for new emails.
func (s *EmailSense) pollIMAP(ctx context.Context, out chan<- *UnifiedInput) error {
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			emails, err := s.fetchNewEmails(ctx)
			if err != nil {
				s.logger.Warn("imap fetch error", "error", err)
				continue
			}
			for _, email := range emails {
				if len(s.config.AllowedSenders) > 0 && !s.isAllowed(email.from) {
					continue
				}

				input := NewUnifiedInput(SourceEmail, email.body)
				input.SourceMeta.Channel = "email"
				input.SourceMeta.Sender = email.from
				input.SourceMeta.Extra = map[string]string{
					"subject":    email.subject,
					"message_id": email.messageID,
					"to":         email.to,
				}
				input.ResponseChannel = email.from

				// Add attachments.
				for _, att := range email.attachments {
					input.Attachments = append(input.Attachments, Attachment{
						Name: att.name,
						Type: att.mimeType,
						Size: att.size,
					})
				}

				select {
				case out <- input:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

// fetchNewEmails connects to IMAP and retrieves unread messages.
func (s *EmailSense) fetchNewEmails(_ context.Context) ([]emailMessage, error) {
	if s.config.IMAPServer == "" {
		return nil, nil // No IMAP configured.
	}

	// Connect to IMAP.
	var client *imapClient
	var err error

	if s.config.DialFunc != nil {
		client, err = s.config.DialFunc(s.config.IMAPServer)
	} else if s.config.IMAPTLS {
		client, err = dialIMAP(s.config.IMAPServer, s.config.TLSConfig)
	} else {
		client, err = dialIMAPPlain(s.config.IMAPServer)
	}
	if err != nil {
		return nil, fmt.Errorf("imap connect: %w", err)
	}
	defer client.Close()

	// Authenticate.
	if err := client.Login(s.config.IMAPUser, s.config.IMAPPass); err != nil {
		return nil, fmt.Errorf("imap login: %w", err)
	}

	// Select folder.
	if err := client.Select(s.config.FolderName); err != nil {
		return nil, fmt.Errorf("imap select: %w", err)
	}

	// Search for unseen messages.
	seqNums, err := client.SearchUnseen()
	if err != nil {
		return nil, fmt.Errorf("imap search: %w", err)
	}

	if len(seqNums) == 0 {
		client.Logout()
		return nil, nil
	}

	// Fetch each message.
	var messages []emailMessage
	for _, seq := range seqNums {
		result, err := client.Fetch(seq)
		if err != nil {
			s.logger.Warn("imap fetch message", "seq", seq, "error", err)
			continue
		}

		msg := emailMessage{
			messageID: result.MsgID,
			from:      extractEmailAddress(result.From),
			to:        extractEmailAddress(result.To),
			subject:   result.Subject,
			body:      result.Body,
		}
		messages = append(messages, msg)

		// Mark as seen.
		if err := client.MarkSeen(seq); err != nil {
			s.logger.Warn("imap mark seen", "seq", seq, "error", err)
		}
	}

	client.Logout()
	return messages, nil
}

// Send sends an email reply via SMTP.
func (s *EmailSense) Send(_ context.Context, target string, message string) error {
	if s.config.SMTPServer == "" && s.config.SMTPSendFunc == nil {
		return nil // No SMTP configured — silent no-op.
	}

	cfg := smtpConfig{
		Host:     s.config.SMTPServer,
		User:     s.config.SMTPUser,
		Password: s.config.SMTPPass,
		From:     s.config.FromAddr,
	}

	msg := smtpMessage{
		To:      target,
		Subject: "Re: Overhuman",
		Body:    message,
	}

	if s.config.SMTPSendFunc != nil {
		return s.config.SMTPSendFunc(cfg, msg)
	}

	return sendSMTP(cfg, msg)
}

// Stop gracefully shuts down the email adapter.
func (s *EmailSense) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *EmailSense) isAllowed(sender string) bool {
	for _, allowed := range s.config.AllowedSenders {
		if strings.EqualFold(allowed, sender) {
			return true
		}
	}
	return false
}

// --- Internal email types ---

type emailMessage struct {
	messageID   string
	from        string
	to          string
	subject     string
	body        string
	attachments []emailAttachment
}

type emailAttachment struct {
	name     string
	mimeType string
	size     int64
}

// extractEmailAddress extracts the bare email address from a "Name <addr>"
// or bare "addr" string.
func extractEmailAddress(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Handle "Display Name <email@example.com>".
	if idx := strings.LastIndex(s, "<"); idx >= 0 {
		end := strings.Index(s[idx:], ">")
		if end > 0 {
			return s[idx+1 : idx+end]
		}
	}
	return s
}
