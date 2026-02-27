package senses

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Minimal IMAP4rev1 client — stdlib only, no external deps.
// Supports: LOGIN, SELECT, SEARCH UNSEEN, FETCH, STORE +FLAGS, LOGOUT.
// ---------------------------------------------------------------------------

// imapClient is a minimal IMAP client for fetching unseen messages.
type imapClient struct {
	conn   net.Conn
	reader *bufio.Reader
	writer io.Writer
	tag    int
	mu     sync.Mutex
}

// imapFetchResult holds the result of a FETCH command.
type imapFetchResult struct {
	UID     int
	From    string
	To      string
	Subject string
	Date    string
	MsgID   string
	Body    string
	Size    int
}

// dialIMAP connects to an IMAP server over TLS.
func dialIMAP(addr string, tlsConfig *tls.Config) (*imapClient, error) {
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}

	// Extract host for SNI if not set.
	if tlsConfig.ServerName == "" {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		tlsConfig = tlsConfig.Clone()
		tlsConfig.ServerName = host
	}

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 30 * time.Second},
		"tcp", addr, tlsConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("imap dial %s: %w", addr, err)
	}

	c := &imapClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: conn,
	}

	// Read server greeting.
	if _, err := c.readLine(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("imap greeting: %w", err)
	}

	return c, nil
}

// dialIMAPPlain connects to an IMAP server over plain TCP (for testing).
func dialIMAPPlain(addr string) (*imapClient, error) {
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("imap dial %s: %w", addr, err)
	}

	c := &imapClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: conn,
	}

	// Read server greeting.
	if _, err := c.readLine(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("imap greeting: %w", err)
	}

	return c, nil
}

// nextTag returns the next command tag (A001, A002, ...).
func (c *imapClient) nextTag() string {
	c.tag++
	return fmt.Sprintf("A%03d", c.tag)
}

// command sends an IMAP command and waits for the tagged response.
// Returns all untagged response lines and the final tagged status line.
func (c *imapClient) command(cmd string) (untagged []string, status string, err error) {
	tag := c.nextTag()
	line := tag + " " + cmd + "\r\n"

	if _, err := io.WriteString(c.writer, line); err != nil {
		return nil, "", fmt.Errorf("imap write: %w", err)
	}

	for {
		resp, err := c.readLine()
		if err != nil {
			return untagged, "", fmt.Errorf("imap read: %w", err)
		}

		if strings.HasPrefix(resp, tag+" ") {
			return untagged, resp[len(tag)+1:], nil
		}
		untagged = append(untagged, resp)
	}
}

// commandLiteral sends an IMAP command and reads the full response including
// literal data (for FETCH). Returns the complete response as a single string.
func (c *imapClient) commandLiteral(cmd string) (string, string, error) {
	tag := c.nextTag()
	line := tag + " " + cmd + "\r\n"

	if _, err := io.WriteString(c.writer, line); err != nil {
		return "", "", fmt.Errorf("imap write: %w", err)
	}

	var buf strings.Builder
	var statusLine string

	for {
		resp, err := c.readLine()
		if err != nil {
			return buf.String(), "", fmt.Errorf("imap read: %w", err)
		}

		if strings.HasPrefix(resp, tag+" ") {
			statusLine = resp[len(tag)+1:]
			break
		}

		// Check for literal {NNN}.
		if idx := strings.LastIndex(resp, "{"); idx >= 0 {
			end := strings.LastIndex(resp, "}")
			if end > idx {
				sizeStr := resp[idx+1 : end]
				size, parseErr := strconv.Atoi(sizeStr)
				if parseErr == nil && size > 0 {
					buf.WriteString(resp)
					buf.WriteString("\n")

					// Read literal data.
					literal := make([]byte, size)
					if _, err := io.ReadFull(c.reader, literal); err != nil {
						return buf.String(), "", fmt.Errorf("imap literal read: %w", err)
					}
					buf.Write(literal)
					continue
				}
			}
		}

		buf.WriteString(resp)
		buf.WriteString("\n")
	}

	return buf.String(), statusLine, nil
}

// readLine reads a single CRLF-terminated line from the server.
func (c *imapClient) readLine() (string, error) {
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// Login authenticates with the IMAP server.
func (c *imapClient) Login(user, pass string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Quote credentials to handle special characters.
	_, status, err := c.command(fmt.Sprintf("LOGIN %s %s",
		imapQuote(user), imapQuote(pass)))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(status, "OK") {
		return fmt.Errorf("imap login: %s", status)
	}
	return nil
}

// Select selects a mailbox folder.
func (c *imapClient) Select(folder string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, status, err := c.command(fmt.Sprintf("SELECT %s", imapQuote(folder)))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(status, "OK") {
		return fmt.Errorf("imap select: %s", status)
	}
	return nil
}

// SearchUnseen searches for UNSEEN messages and returns their sequence numbers.
func (c *imapClient) SearchUnseen() ([]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	untagged, status, err := c.command("SEARCH UNSEEN")
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(status, "OK") {
		return nil, fmt.Errorf("imap search: %s", status)
	}

	var seqNums []int
	for _, line := range untagged {
		if strings.HasPrefix(line, "* SEARCH") {
			parts := strings.Fields(line)
			for _, p := range parts[2:] { // Skip "* SEARCH"
				num, err := strconv.Atoi(p)
				if err == nil {
					seqNums = append(seqNums, num)
				}
			}
		}
	}
	return seqNums, nil
}

// Fetch retrieves message headers and body for a given sequence number.
func (c *imapClient) Fetch(seqNum int) (*imapFetchResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := fmt.Sprintf("FETCH %d (BODY[HEADER] BODY[TEXT] RFC822.SIZE)", seqNum)
	raw, status, err := c.commandLiteral(cmd)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(status, "OK") {
		return nil, fmt.Errorf("imap fetch: %s", status)
	}

	result := &imapFetchResult{UID: seqNum}
	parseIMAPFetch(raw, result)
	return result, nil
}

// MarkSeen marks a message as \Seen.
func (c *imapClient) MarkSeen(seqNum int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, status, err := c.command(fmt.Sprintf("STORE %d +FLAGS (\\Seen)", seqNum))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(status, "OK") {
		return fmt.Errorf("imap store: %s", status)
	}
	return nil
}

// Logout sends LOGOUT command.
func (c *imapClient) Logout() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, _, err := c.command("LOGOUT")
	return err
}

// Close closes the underlying connection.
func (c *imapClient) Close() error {
	return c.conn.Close()
}

// ---------------------------------------------------------------------------
// IMAP helpers
// ---------------------------------------------------------------------------

// imapQuote quotes a string for IMAP (double-quoted string).
func imapQuote(s string) string {
	// Escape backslashes and double quotes.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// parseIMAPFetch extracts headers and body from raw FETCH response.
func parseIMAPFetch(raw string, result *imapFetchResult) {
	lines := strings.Split(raw, "\n")
	inHeader := false
	inBody := false
	var headerBuf, bodyBuf strings.Builder
	prevHeaderKey := ""

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r ")

		// Detect section boundaries from FETCH response.
		if strings.Contains(line, "BODY[HEADER]") {
			inHeader = true
			inBody = false
			continue
		}
		if strings.Contains(line, "BODY[TEXT]") {
			inHeader = false
			inBody = true
			continue
		}

		// Parse RFC822.SIZE.
		if strings.Contains(line, "RFC822.SIZE") {
			for _, part := range strings.Fields(line) {
				if n, err := strconv.Atoi(part); err == nil && n > 0 {
					result.Size = n
				}
			}
		}

		if inHeader {
			// End of headers: empty line.
			if trimmed == "" || trimmed == ")" {
				inHeader = false
				continue
			}
			headerBuf.WriteString(trimmed)
			headerBuf.WriteString("\n")

			// Parse header fields (handle folded headers).
			if len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t') {
				// Continuation of previous header.
				if prevHeaderKey != "" {
					val := strings.TrimSpace(trimmed)
					switch prevHeaderKey {
					case "subject":
						result.Subject += " " + val
					case "from":
						result.From += " " + val
					case "to":
						result.To += " " + val
					}
				}
				continue
			}

			if idx := strings.Index(trimmed, ":"); idx > 0 {
				key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
				val := strings.TrimSpace(trimmed[idx+1:])

				switch key {
				case "from":
					result.From = val
					prevHeaderKey = "from"
				case "to":
					result.To = val
					prevHeaderKey = "to"
				case "subject":
					result.Subject = val
					prevHeaderKey = "subject"
				case "date":
					result.Date = val
					prevHeaderKey = "date"
				case "message-id":
					result.MsgID = val
					prevHeaderKey = "message-id"
				default:
					prevHeaderKey = key
				}
			}
		} else if inBody {
			// End of body: closing paren.
			if trimmed == ")" {
				inBody = false
				continue
			}
			bodyBuf.WriteString(trimmed)
			bodyBuf.WriteString("\n")
		}
	}

	result.Body = strings.TrimSpace(bodyBuf.String())
}
