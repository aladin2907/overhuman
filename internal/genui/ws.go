package genui

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	wsGUID          = "258EAFA5-E914-47DA-95CA-5AB9CAD40B11"
	wsOpText        = 1
	wsOpClose       = 8
	wsOpPing        = 9
	wsOpPong        = 10
	wsMaxFrameSize  = 1 << 20 // 1MB
	wsPingInterval  = 30 * time.Second
)

// WSConn represents a single WebSocket connection.
type WSConn struct {
	conn   net.Conn
	rw     *bufio.ReadWriter
	mu     sync.Mutex
	closed bool
	id     string
}

// WSServer is a WebSocket server for delivering UI to web clients.
type WSServer struct {
	mu       sync.RWMutex
	clients  map[string]*WSConn
	lastUI   *WSMessage // cached last UI for reconnect
	onMsg    func(connID string, msg *WSMessage)
	addr     string
	srv      *http.Server
	listener net.Listener
}

// NewWSServer creates a new WebSocket server.
func NewWSServer(addr string) *WSServer {
	return &WSServer{
		addr:    addr,
		clients: make(map[string]*WSConn),
	}
}

// OnMessage sets the callback for incoming client messages.
func (s *WSServer) OnMessage(fn func(connID string, msg *WSMessage)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMsg = fn
}

// Start launches the WebSocket server. Blocks until ctx is cancelled.
func (s *WSServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		s.handleUpgrade(w, r, ctx)
	})

	s.mu.Lock()
	s.srv = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.mu.Unlock()

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("ws server: listen: %w", err)
	}

	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.srv.Shutdown(shutCtx)
	}()

	if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("ws server: serve: %w", err)
	}
	return nil
}

// Addr returns the listener address. Returns empty string if not started.
func (s *WSServer) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

// Broadcast sends a message to all connected clients.
func (s *WSServer) Broadcast(msg *WSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	s.mu.RLock()
	clients := make([]*WSConn, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.RUnlock()

	for _, c := range clients {
		if writeErr := c.writeText(data); writeErr != nil {
			log.Printf("[ws] broadcast write error for %s: %v", c.id, writeErr)
		}
	}
	return nil
}

// BroadcastUI sends a GeneratedUI to all connected clients and caches it.
func (s *WSServer) BroadcastUI(ui *GeneratedUI) error {
	msg, err := NewUIFullMessage(ui)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.lastUI = msg
	s.mu.Unlock()
	return s.Broadcast(msg)
}

// ClientCount returns the number of connected clients.
func (s *WSServer) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// Stop gracefully shuts down the server and closes all connections.
func (s *WSServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, c := range s.clients {
		c.close()
		delete(s.clients, id)
	}

	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.srv.Shutdown(ctx)
	}
	return nil
}

// handleUpgrade performs the WebSocket handshake and starts message loop.
func (s *WSServer) handleUpgrade(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "expected websocket upgrade", http.StatusBadRequest)
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	// Compute accept key.
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Hijack the connection.
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "server doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send upgrade response.
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"
	bufrw.WriteString(resp)
	bufrw.Flush()

	// Register client.
	connID := fmt.Sprintf("ws_%d", time.Now().UnixNano())
	wsConn := &WSConn{
		conn: conn,
		rw:   bufrw,
		id:   connID,
	}

	s.mu.Lock()
	s.clients[connID] = wsConn
	lastUI := s.lastUI
	s.mu.Unlock()

	log.Printf("[ws] client connected: %s", connID)

	// Send cached last UI on reconnect.
	if lastUI != nil {
		data, _ := json.Marshal(lastUI)
		wsConn.writeText(data)
	}

	// Start read loop.
	go s.readLoop(wsConn, ctx)
}

// readLoop reads messages from a WebSocket connection.
func (s *WSServer) readLoop(c *WSConn, ctx context.Context) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, c.id)
		s.mu.Unlock()
		c.close()
		log.Printf("[ws] client disconnected: %s", c.id)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		opcode, payload, err := readFrame(c.rw.Reader)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				log.Printf("[ws] read error for %s: %v", c.id, err)
			}
			return
		}

		switch opcode {
		case wsOpText:
			msg, parseErr := ParseWSMessage(payload)
			if parseErr != nil {
				log.Printf("[ws] parse error from %s: %v", c.id, parseErr)
				continue
			}
			s.handleMessage(c, msg)

		case wsOpPing:
			c.writePong(payload)

		case wsOpClose:
			return
		}
	}
}

// handleMessage routes a parsed message to the appropriate handler.
func (s *WSServer) handleMessage(c *WSConn, msg *WSMessage) {
	switch msg.Type {
	case WSMsgPing:
		pong, _ := NewWSMessage(WSMsgPong, nil)
		data, _ := json.Marshal(pong)
		c.writeText(data)

	default:
		s.mu.RLock()
		fn := s.onMsg
		s.mu.RUnlock()
		if fn != nil {
			fn(c.id, msg)
		}
	}
}

// writeText sends a text frame to the WebSocket connection.
func (c *WSConn) writeText(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("connection closed")
	}
	return writeFrame(c.rw.Writer, wsOpText, data)
}

// writePong sends a pong frame.
func (c *WSConn) writePong(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("connection closed")
	}
	return writeFrame(c.rw.Writer, wsOpPong, data)
}

// close closes the connection.
func (c *WSConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.conn.Close()
	}
}

// --- RFC 6455 Frame I/O ---

// readFrame reads a single WebSocket frame. Returns opcode and payload.
// Client frames are always masked (RFC 6455 §5.1).
func readFrame(r *bufio.Reader) (opcode byte, payload []byte, err error) {
	// Byte 0: FIN + opcode.
	b0, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	opcode = b0 & 0x0f

	// Byte 1: MASK + payload length.
	b1, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	masked := b1&0x80 != 0
	length := uint64(b1 & 0x7f)

	// Extended payload length.
	switch length {
	case 126:
		var buf [2]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(buf[:]))
	case 127:
		var buf [8]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(buf[:])
	}

	if length > wsMaxFrameSize {
		return 0, nil, fmt.Errorf("frame too large: %d bytes", length)
	}

	// Masking key (4 bytes, only if masked).
	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(r, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	// Payload.
	payload = make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}

	// Unmask if needed.
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return opcode, payload, nil
}

// writeFrame writes a single WebSocket frame. Server frames are NOT masked.
func writeFrame(w *bufio.Writer, opcode byte, data []byte) error {
	// Byte 0: FIN=1 + opcode.
	if err := w.WriteByte(0x80 | opcode); err != nil {
		return err
	}

	// Payload length (no mask for server frames).
	length := len(data)
	switch {
	case length <= 125:
		if err := w.WriteByte(byte(length)); err != nil {
			return err
		}
	case length <= 65535:
		if err := w.WriteByte(126); err != nil {
			return err
		}
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], uint16(length))
		if _, err := w.Write(buf[:]); err != nil {
			return err
		}
	default:
		if err := w.WriteByte(127); err != nil {
			return err
		}
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(length))
		if _, err := w.Write(buf[:]); err != nil {
			return err
		}
	}

	// Payload.
	if _, err := w.Write(data); err != nil {
		return err
	}
	return w.Flush()
}
