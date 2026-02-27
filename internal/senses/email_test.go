package senses

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock IMAP server — speaks minimal IMAP4rev1 for testing.
// ---------------------------------------------------------------------------

type mockIMAPServer struct {
	listener net.Listener
	addr     string
	messages []mockIMAPMessage
	mu       sync.Mutex
	stopped  bool
}

type mockIMAPMessage struct {
	SeqNum  int
	From    string
	To      string
	Subject string
	MsgID   string
	Body    string
	Seen    bool
}

func newMockIMAPServer(messages []mockIMAPMessage) (*mockIMAPServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	s := &mockIMAPServer{
		listener: ln,
		addr:     ln.Addr().String(),
		messages: messages,
	}

	go s.serve()
	return s, nil
}

func (s *mockIMAPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *mockIMAPServer) handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	// Send greeting.
	fmt.Fprintf(conn, "* OK Mock IMAP server ready\r\n")

	selectedFolder := ""

	for {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}

		tag := parts[0]
		cmd := strings.ToUpper(parts[1])
		var args string
		if len(parts) > 2 {
			args = parts[2]
		}

		switch cmd {
		case "LOGIN":
			// Accept any credentials.
			fmt.Fprintf(conn, "%s OK LOGIN completed\r\n", tag)

		case "SELECT":
			selectedFolder = strings.Trim(args, "\" ")
			s.mu.Lock()
			count := len(s.messages)
			s.mu.Unlock()
			fmt.Fprintf(conn, "* %d EXISTS\r\n", count)
			fmt.Fprintf(conn, "* 0 RECENT\r\n")
			fmt.Fprintf(conn, "* FLAGS (\\Seen \\Answered \\Flagged \\Deleted \\Draft)\r\n")
			fmt.Fprintf(conn, "%s OK [READ-WRITE] SELECT completed (%s)\r\n", tag, selectedFolder)

		case "SEARCH":
			s.mu.Lock()
			var unseenNums []string
			for _, m := range s.messages {
				if !m.Seen {
					unseenNums = append(unseenNums, fmt.Sprintf("%d", m.SeqNum))
				}
			}
			s.mu.Unlock()
			if len(unseenNums) > 0 {
				fmt.Fprintf(conn, "* SEARCH %s\r\n", strings.Join(unseenNums, " "))
			} else {
				fmt.Fprintf(conn, "* SEARCH\r\n")
			}
			fmt.Fprintf(conn, "%s OK SEARCH completed\r\n", tag)

		case "FETCH":
			seqStr := ""
			if len(parts) > 2 {
				fetchParts := strings.SplitN(args, " ", 2)
				seqStr = fetchParts[0]
			}
			seqNum := 0
			fmt.Sscanf(seqStr, "%d", &seqNum)

			s.mu.Lock()
			var msg *mockIMAPMessage
			for i := range s.messages {
				if s.messages[i].SeqNum == seqNum {
					msg = &s.messages[i]
					break
				}
			}
			s.mu.Unlock()

			if msg != nil {
				header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\nDate: Mon, 01 Jan 2026 00:00:00 +0000\r\n",
					msg.From, msg.To, msg.Subject, msg.MsgID)
				body := msg.Body

				fmt.Fprintf(conn, "* %d FETCH (BODY[HEADER] {%d}\r\n%s\r\nBODY[TEXT] {%d}\r\n%s\r\nRFC822.SIZE %d)\r\n",
					seqNum,
					len(header), header,
					len(body), body,
					len(header)+len(body)+4)
			}
			fmt.Fprintf(conn, "%s OK FETCH completed\r\n", tag)

		case "STORE":
			// Parse seq num and mark as seen.
			storeArgs := strings.Fields(args)
			if len(storeArgs) > 0 {
				seqNum := 0
				fmt.Sscanf(parts[2], "%d", &seqNum)
				s.mu.Lock()
				for i := range s.messages {
					if s.messages[i].SeqNum == seqNum {
						s.messages[i].Seen = true
						break
					}
				}
				s.mu.Unlock()
			}
			fmt.Fprintf(conn, "%s OK STORE completed\r\n", tag)

		case "LOGOUT":
			fmt.Fprintf(conn, "* BYE Mock IMAP server closing\r\n")
			fmt.Fprintf(conn, "%s OK LOGOUT completed\r\n", tag)
			return

		default:
			fmt.Fprintf(conn, "%s BAD Unknown command\r\n", tag)
		}
	}
}

func (s *mockIMAPServer) close() {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	s.listener.Close()
}

// ---------------------------------------------------------------------------
// Mock SMTP server — minimal SMTP for testing.
// ---------------------------------------------------------------------------

type mockSMTPServer struct {
	listener net.Listener
	addr     string
	mu       sync.Mutex
	received []mockSMTPMessage
	stopped  bool
}

type mockSMTPMessage struct {
	From    string
	To      []string
	Data    string
}

func newMockSMTPServer() (*mockSMTPServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	s := &mockSMTPServer{
		listener: ln,
		addr:     ln.Addr().String(),
	}

	go s.serve()
	return s, nil
}

func (s *mockSMTPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *mockSMTPServer) handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	// Send greeting.
	fmt.Fprintf(conn, "220 Mock SMTP server ready\r\n")

	var curFrom string
	var curTo []string
	var inData bool
	var dataBuf strings.Builder

	for {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				s.mu.Lock()
				s.received = append(s.received, mockSMTPMessage{
					From: curFrom,
					To:   curTo,
					Data: dataBuf.String(),
				})
				s.mu.Unlock()
				fmt.Fprintf(conn, "250 OK message accepted\r\n")
				curFrom = ""
				curTo = nil
				dataBuf.Reset()
			} else {
				dataBuf.WriteString(line)
				dataBuf.WriteString("\n")
			}
			continue
		}

		cmd := strings.ToUpper(line)

		switch {
		case strings.HasPrefix(cmd, "EHLO") || strings.HasPrefix(cmd, "HELO"):
			fmt.Fprintf(conn, "250-Mock SMTP\r\n")
			fmt.Fprintf(conn, "250 OK\r\n")

		case strings.HasPrefix(cmd, "MAIL FROM:"):
			curFrom = extractAngleBrackets(line[10:])
			fmt.Fprintf(conn, "250 OK\r\n")

		case strings.HasPrefix(cmd, "RCPT TO:"):
			curTo = append(curTo, extractAngleBrackets(line[8:]))
			fmt.Fprintf(conn, "250 OK\r\n")

		case strings.HasPrefix(cmd, "DATA"):
			inData = true
			fmt.Fprintf(conn, "354 Start mail input\r\n")

		case strings.HasPrefix(cmd, "QUIT"):
			fmt.Fprintf(conn, "221 Bye\r\n")
			return

		case strings.HasPrefix(cmd, "RSET"):
			curFrom = ""
			curTo = nil
			dataBuf.Reset()
			fmt.Fprintf(conn, "250 OK\r\n")

		default:
			fmt.Fprintf(conn, "500 Unknown command\r\n")
		}
	}
}

func (s *mockSMTPServer) close() {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	s.listener.Close()
}

func (s *mockSMTPServer) getReceived() []mockSMTPMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]mockSMTPMessage, len(s.received))
	copy(result, s.received)
	return result
}

func extractAngleBrackets(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "<"); idx >= 0 {
		end := strings.Index(s, ">")
		if end > idx {
			return s[idx+1 : end]
		}
	}
	return s
}

// ---------------------------------------------------------------------------
// IMAP client tests
// ---------------------------------------------------------------------------

func TestIMAPClient_Connect(t *testing.T) {
	srv, err := newMockIMAPServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	client, err := dialIMAPPlain(srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
}

func TestIMAPClient_Login(t *testing.T) {
	srv, err := newMockIMAPServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	client, err := dialIMAPPlain(srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Login("user@test.com", "password"); err != nil {
		t.Fatal(err)
	}
}

func TestIMAPClient_SelectFolder(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{SeqNum: 1, From: "a@test.com", Subject: "Test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	client, err := dialIMAPPlain(srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Login("user", "pass"); err != nil {
		t.Fatal(err)
	}
	if err := client.Select("INBOX"); err != nil {
		t.Fatal(err)
	}
}

func TestIMAPClient_SearchUnseen(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{SeqNum: 1, From: "a@test.com", Subject: "Msg1", Seen: false},
		{SeqNum: 2, From: "b@test.com", Subject: "Msg2", Seen: true},
		{SeqNum: 3, From: "c@test.com", Subject: "Msg3", Seen: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	client, err := dialIMAPPlain(srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	client.Login("user", "pass")
	client.Select("INBOX")

	seqNums, err := client.SearchUnseen()
	if err != nil {
		t.Fatal(err)
	}
	if len(seqNums) != 2 {
		t.Fatalf("unseen = %d, want 2", len(seqNums))
	}
	if seqNums[0] != 1 || seqNums[1] != 3 {
		t.Errorf("seqNums = %v, want [1, 3]", seqNums)
	}
}

func TestIMAPClient_SearchUnseen_Empty(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{SeqNum: 1, From: "a@test.com", Subject: "Msg1", Seen: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	client, err := dialIMAPPlain(srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	client.Login("user", "pass")
	client.Select("INBOX")

	seqNums, err := client.SearchUnseen()
	if err != nil {
		t.Fatal(err)
	}
	if len(seqNums) != 0 {
		t.Errorf("unseen = %d, want 0", len(seqNums))
	}
}

func TestIMAPClient_Fetch(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{
			SeqNum:  1,
			From:    "Alice <alice@test.com>",
			To:      "bob@test.com",
			Subject: "Hello World",
			MsgID:   "<msg001@test.com>",
			Body:    "This is the body text.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	client, err := dialIMAPPlain(srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	client.Login("user", "pass")
	client.Select("INBOX")

	result, err := client.Fetch(1)
	if err != nil {
		t.Fatal(err)
	}

	if result.From != "Alice <alice@test.com>" {
		t.Errorf("From = %q", result.From)
	}
	if result.To != "bob@test.com" {
		t.Errorf("To = %q", result.To)
	}
	if result.Subject != "Hello World" {
		t.Errorf("Subject = %q", result.Subject)
	}
	if result.MsgID != "<msg001@test.com>" {
		t.Errorf("MsgID = %q", result.MsgID)
	}
	if !strings.Contains(result.Body, "This is the body text.") {
		t.Errorf("Body = %q", result.Body)
	}
}

func TestIMAPClient_MarkSeen(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{SeqNum: 1, From: "a@test.com", Subject: "Test", Seen: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	client, err := dialIMAPPlain(srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	client.Login("user", "pass")
	client.Select("INBOX")

	if err := client.MarkSeen(1); err != nil {
		t.Fatal(err)
	}

	// Verify the message is now seen.
	seqNums, _ := client.SearchUnseen()
	if len(seqNums) != 0 {
		t.Errorf("should have 0 unseen after MarkSeen, got %d", len(seqNums))
	}
}

func TestIMAPClient_Logout(t *testing.T) {
	srv, err := newMockIMAPServer(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	client, err := dialIMAPPlain(srv.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	client.Login("user", "pass")
	if err := client.Logout(); err != nil {
		t.Fatal(err)
	}
}

func TestIMAPClient_NextTag(t *testing.T) {
	c := &imapClient{}
	if tag := c.nextTag(); tag != "A001" {
		t.Errorf("first tag = %q, want A001", tag)
	}
	if tag := c.nextTag(); tag != "A002" {
		t.Errorf("second tag = %q, want A002", tag)
	}
}

func TestIMAPQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", `"simple"`},
		{`has"quote`, `"has\"quote"`},
		{`has\back`, `"has\\back"`},
		{"", `""`},
	}
	for _, tt := range tests {
		got := imapQuote(tt.input)
		if got != tt.want {
			t.Errorf("imapQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// SMTP tests
// ---------------------------------------------------------------------------

func TestSMTP_Send(t *testing.T) {
	srv, err := newMockSMTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	// Connect directly using net/smtp stdlib.
	client, err := smtp.Dial(srv.addr)
	if err != nil {
		t.Fatal(err)
	}

	msg := smtpMessage{
		To:      "recipient@test.com",
		Subject: "Test Subject",
		Body:    "Hello from test.",
	}

	err = sendSMTPDirect(client, "sender@test.com", msg)
	if err != nil {
		t.Fatal(err)
	}

	client.Quit()

	// Verify server received the message.
	received := srv.getReceived()
	if len(received) != 1 {
		t.Fatalf("received %d messages, want 1", len(received))
	}
	if received[0].From != "sender@test.com" {
		t.Errorf("From = %q", received[0].From)
	}
	if len(received[0].To) != 1 || received[0].To[0] != "recipient@test.com" {
		t.Errorf("To = %v", received[0].To)
	}
	if !strings.Contains(received[0].Data, "Hello from test.") {
		t.Errorf("Data missing body: %q", received[0].Data)
	}
	if !strings.Contains(received[0].Data, "Subject: Test Subject") {
		t.Errorf("Data missing subject: %q", received[0].Data)
	}
}

func TestSMTP_MultipleRecipients(t *testing.T) {
	srv, err := newMockSMTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	client, err := smtp.Dial(srv.addr)
	if err != nil {
		t.Fatal(err)
	}

	msg := smtpMessage{
		To:      "a@test.com, b@test.com",
		Subject: "Multi",
		Body:    "To both.",
	}

	err = sendSMTPDirect(client, "sender@test.com", msg)
	if err != nil {
		t.Fatal(err)
	}
	client.Quit()

	received := srv.getReceived()
	if len(received) != 1 {
		t.Fatalf("received %d messages, want 1", len(received))
	}
	if len(received[0].To) != 2 {
		t.Errorf("To = %v, want 2 recipients", received[0].To)
	}
}

func TestBuildHeaders(t *testing.T) {
	msg := smtpMessage{
		To:      "recipient@test.com",
		Subject: "Test",
		Body:    "Body",
		ReplyTo: "<msg123@test.com>",
	}

	headers := buildHeaders("sender@test.com", msg)

	if !strings.Contains(headers, "From: sender@test.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(headers, "To: recipient@test.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(headers, "Subject: Test") {
		t.Error("missing Subject header")
	}
	if !strings.Contains(headers, "In-Reply-To: <msg123@test.com>") {
		t.Error("missing In-Reply-To header")
	}
	if !strings.Contains(headers, "MIME-Version: 1.0") {
		t.Error("missing MIME-Version header")
	}
	if !strings.Contains(headers, "Content-Type: text/plain") {
		t.Error("missing Content-Type header")
	}
}

// ---------------------------------------------------------------------------
// EmailSense integration tests
// ---------------------------------------------------------------------------

func TestEmailSense_FetchNewEmails_Real(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{
			SeqNum:  1,
			From:    "Alice <alice@example.com>",
			To:      "me@example.com",
			Subject: "Hello",
			MsgID:   "<msg001@example.com>",
			Body:    "Hi there!",
			Seen:    false,
		},
		{
			SeqNum:  2,
			From:    "Bob <bob@example.com>",
			To:      "me@example.com",
			Subject: "Update",
			MsgID:   "<msg002@example.com>",
			Body:    "Status update.",
			Seen:    false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	sense := NewEmailSense(EmailConfig{
		IMAPServer: srv.addr,
		IMAPUser:   "me@example.com",
		IMAPPass:   "password",
		FolderName: "INBOX",
		DialFunc: func(addr string) (*imapClient, error) {
			return dialIMAPPlain(addr)
		},
	})

	emails, err := sense.fetchNewEmails(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(emails) != 2 {
		t.Fatalf("emails = %d, want 2", len(emails))
	}

	if emails[0].from != "alice@example.com" {
		t.Errorf("emails[0].from = %q", emails[0].from)
	}
	if emails[0].subject != "Hello" {
		t.Errorf("emails[0].subject = %q", emails[0].subject)
	}
	if !strings.Contains(emails[0].body, "Hi there!") {
		t.Errorf("emails[0].body = %q", emails[0].body)
	}
	if emails[0].messageID != "<msg001@example.com>" {
		t.Errorf("emails[0].messageID = %q", emails[0].messageID)
	}

	if emails[1].from != "bob@example.com" {
		t.Errorf("emails[1].from = %q", emails[1].from)
	}
	if emails[1].subject != "Update" {
		t.Errorf("emails[1].subject = %q", emails[1].subject)
	}
}

func TestEmailSense_FetchNewEmails_NoServer(t *testing.T) {
	sense := NewEmailSense(EmailConfig{})
	emails, err := sense.fetchNewEmails(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(emails) != 0 {
		t.Errorf("emails = %d, want 0 (no server configured)", len(emails))
	}
}

func TestEmailSense_FetchNewEmails_NoUnseen(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{SeqNum: 1, From: "a@test.com", Subject: "Old", Seen: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	sense := NewEmailSense(EmailConfig{
		IMAPServer: srv.addr,
		IMAPUser:   "user",
		IMAPPass:   "pass",
		DialFunc: func(addr string) (*imapClient, error) {
			return dialIMAPPlain(addr)
		},
	})

	emails, err := sense.fetchNewEmails(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(emails) != 0 {
		t.Errorf("emails = %d, want 0 (all seen)", len(emails))
	}
}

func TestEmailSense_FetchMarksAsSeen(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{SeqNum: 1, From: "a@test.com", Subject: "New", Seen: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	sense := NewEmailSense(EmailConfig{
		IMAPServer: srv.addr,
		IMAPUser:   "user",
		IMAPPass:   "pass",
		DialFunc: func(addr string) (*imapClient, error) {
			return dialIMAPPlain(addr)
		},
	})

	// First fetch — should get 1 message.
	emails, err := sense.fetchNewEmails(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(emails) != 1 {
		t.Fatalf("first fetch = %d, want 1", len(emails))
	}

	// Second fetch — should get 0 (marked as seen).
	emails, err = sense.fetchNewEmails(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(emails) != 0 {
		t.Errorf("second fetch = %d, want 0 (marked seen)", len(emails))
	}
}

func TestEmailSense_Send_WithMockSMTP(t *testing.T) {
	srv, err := newMockSMTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	var sentCfg smtpConfig
	var sentMsg smtpMessage

	sense := NewEmailSense(EmailConfig{
		SMTPServer: srv.addr,
		SMTPUser:   "user",
		SMTPPass:   "pass",
		FromAddr:   "bot@example.com",
		SMTPSendFunc: func(cfg smtpConfig, msg smtpMessage) error {
			sentCfg = cfg
			sentMsg = msg
			return nil
		},
	})

	err = sense.Send(context.Background(), "user@example.com", "Hello from bot")
	if err != nil {
		t.Fatal(err)
	}

	if sentCfg.From != "bot@example.com" {
		t.Errorf("From = %q", sentCfg.From)
	}
	if sentMsg.To != "user@example.com" {
		t.Errorf("To = %q", sentMsg.To)
	}
	if sentMsg.Body != "Hello from bot" {
		t.Errorf("Body = %q", sentMsg.Body)
	}
}

func TestEmailSense_Send_NoSMTP(t *testing.T) {
	sense := NewEmailSense(EmailConfig{})
	err := sense.Send(context.Background(), "user@example.com", "hello")
	if err != nil {
		t.Fatal("Send with no SMTP should be no-op, not error")
	}
}

func TestEmailSense_AllowedSenders_Filter(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{SeqNum: 1, From: "alice@example.com", Subject: "Allowed", Body: "ok", Seen: false},
		{SeqNum: 2, From: "eve@evil.com", Subject: "Blocked", Body: "bad", Seen: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	sense := NewEmailSense(EmailConfig{
		IMAPServer:     srv.addr,
		IMAPUser:       "user",
		IMAPPass:       "pass",
		AllowedSenders: []string{"alice@example.com"},
		PollInterval:   50 * time.Millisecond,
		DialFunc: func(addr string) (*imapClient, error) {
			return dialIMAPPlain(addr)
		},
	})

	out := make(chan *UnifiedInput, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go sense.Start(ctx, out)

	// Collect messages within timeout.
	var received []*UnifiedInput
	timer := time.After(180 * time.Millisecond)
loop:
	for {
		select {
		case msg := <-out:
			received = append(received, msg)
		case <-timer:
			break loop
		}
	}

	// Should only receive alice's message, not eve's.
	if len(received) != 1 {
		t.Fatalf("received %d messages, want 1 (filtered)", len(received))
	}
	if received[0].SourceMeta.Sender != "alice@example.com" {
		t.Errorf("Sender = %q", received[0].SourceMeta.Sender)
	}
}

func TestEmailSense_PollIMAP_ProducesUnifiedInput(t *testing.T) {
	srv, err := newMockIMAPServer([]mockIMAPMessage{
		{
			SeqNum:  1,
			From:    "sender@test.com",
			To:      "me@test.com",
			Subject: "Important",
			MsgID:   "<msg123@test.com>",
			Body:    "Please review.",
			Seen:    false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	sense := NewEmailSense(EmailConfig{
		IMAPServer:   srv.addr,
		IMAPUser:     "user",
		IMAPPass:     "pass",
		PollInterval: 50 * time.Millisecond,
		DialFunc: func(addr string) (*imapClient, error) {
			return dialIMAPPlain(addr)
		},
	})

	out := make(chan *UnifiedInput, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go sense.Start(ctx, out)

	select {
	case input := <-out:
		if input.SourceType != SourceEmail {
			t.Errorf("SourceType = %q", input.SourceType)
		}
		if input.SourceMeta.Channel != "email" {
			t.Errorf("Channel = %q", input.SourceMeta.Channel)
		}
		if input.SourceMeta.Sender != "sender@test.com" {
			t.Errorf("Sender = %q", input.SourceMeta.Sender)
		}
		if input.SourceMeta.Extra["subject"] != "Important" {
			t.Errorf("subject = %q", input.SourceMeta.Extra["subject"])
		}
		if input.SourceMeta.Extra["message_id"] != "<msg123@test.com>" {
			t.Errorf("message_id = %q", input.SourceMeta.Extra["message_id"])
		}
		if input.ResponseChannel != "sender@test.com" {
			t.Errorf("ResponseChannel = %q", input.ResponseChannel)
		}
		if !strings.Contains(input.Payload, "Please review.") {
			t.Errorf("Payload = %q", input.Payload)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}
}

func TestEmailSense_IsAllowed_CaseInsensitive(t *testing.T) {
	sense := NewEmailSense(EmailConfig{
		AllowedSenders: []string{"Alice@Example.com"},
	})

	if !sense.isAllowed("alice@example.com") {
		t.Error("should be case-insensitive match")
	}
	if !sense.isAllowed("ALICE@EXAMPLE.COM") {
		t.Error("should be case-insensitive match (upper)")
	}
}

// ---------------------------------------------------------------------------
// extractEmailAddress tests
// ---------------------------------------------------------------------------

func TestExtractEmailAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user@example.com", "user@example.com"},
		{"Alice <alice@example.com>", "alice@example.com"},
		{"  Bob Smith <bob@test.com>  ", "bob@test.com"},
		{"<noreply@test.com>", "noreply@test.com"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractEmailAddress(tt.input)
		if got != tt.want {
			t.Errorf("extractEmailAddress(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// parseIMAPFetch tests
// ---------------------------------------------------------------------------

func TestParseIMAPFetch(t *testing.T) {
	raw := `* 1 FETCH (BODY[HEADER] {120}
From: Alice <alice@test.com>
To: bob@test.com
Subject: Hello World
Message-ID: <msg001@test.com>
Date: Mon, 01 Jan 2026 00:00:00 +0000

BODY[TEXT] {22}
This is the body text.
RFC822.SIZE 142)
`

	result := &imapFetchResult{UID: 1}
	parseIMAPFetch(raw, result)

	if result.From != "Alice <alice@test.com>" {
		t.Errorf("From = %q", result.From)
	}
	if result.To != "bob@test.com" {
		t.Errorf("To = %q", result.To)
	}
	if result.Subject != "Hello World" {
		t.Errorf("Subject = %q", result.Subject)
	}
	if result.MsgID != "<msg001@test.com>" {
		t.Errorf("MsgID = %q", result.MsgID)
	}
	if !strings.Contains(result.Body, "This is the body text.") {
		t.Errorf("Body = %q", result.Body)
	}
}

func TestParseIMAPFetch_EmptyBody(t *testing.T) {
	raw := `* 1 FETCH (BODY[HEADER] {50}
From: a@test.com
Subject: No body

BODY[TEXT] {0}
)
`
	result := &imapFetchResult{UID: 1}
	parseIMAPFetch(raw, result)

	if result.From != "a@test.com" {
		t.Errorf("From = %q", result.From)
	}
	if result.Subject != "No body" {
		t.Errorf("Subject = %q", result.Subject)
	}
}
