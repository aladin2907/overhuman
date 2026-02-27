package senses

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EmailConfig holds email adapter configuration.
type EmailConfig struct {
	// IMAP settings (receiving).
	IMAPServer string `json:"imap_server"` // e.g., "imap.gmail.com:993"
	IMAPUser   string `json:"imap_user"`
	IMAPPass   string `json:"imap_pass"`

	// SMTP settings (sending).
	SMTPServer string `json:"smtp_server"` // e.g., "smtp.gmail.com:587"
	SMTPUser   string `json:"smtp_user"`
	SMTPPass   string `json:"smtp_pass"`
	FromAddr   string `json:"from_addr"`

	// Polling interval for IMAP IDLE fallback.
	PollInterval time.Duration `json:"poll_interval"`

	// Filters.
	AllowedSenders []string `json:"allowed_senders"` // Whitelist (empty = allow all)
	FolderName     string   `json:"folder_name"`     // IMAP folder to watch (default: INBOX)
}

// EmailSense receives messages via IMAP and sends via SMTP.
type EmailSense struct {
	config  EmailConfig
	mu      sync.Mutex
	stopped bool
	cancel  context.CancelFunc
}

// NewEmailSense creates an email adapter.
func NewEmailSense(config EmailConfig) *EmailSense {
	if config.PollInterval == 0 {
		config.PollInterval = 60 * time.Second
	}
	if config.FolderName == "" {
		config.FolderName = "INBOX"
	}
	return &EmailSense{config: config}
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
				continue // Log and retry.
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
// This is a placeholder — actual IMAP implementation would use
// an IMAP client library or raw protocol.
func (s *EmailSense) fetchNewEmails(_ context.Context) ([]emailMessage, error) {
	// Placeholder: In production, connect to IMAP server,
	// select folder, search UNSEEN, fetch messages.
	return nil, nil
}

// Send sends an email reply via SMTP.
func (s *EmailSense) Send(ctx context.Context, target string, message string) error {
	// target is the recipient email address.
	// Placeholder: In production, connect to SMTP server,
	// compose message with proper headers, send.
	_ = ctx
	_ = target
	_ = message
	return nil
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
		if allowed == sender {
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
