package genui

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- Protocol Tests ---

func TestParseWSMessage_Valid(t *testing.T) {
	data := []byte(`{"type":"action","payload":{"action_id":"apply"}}`)
	msg, err := ParseWSMessage(data)
	if err != nil {
		t.Fatalf("ParseWSMessage: %v", err)
	}
	if msg.Type != WSMsgAction {
		t.Errorf("Type = %q, want action", msg.Type)
	}
}

func TestParseWSMessage_MissingType(t *testing.T) {
	data := []byte(`{"payload":{"action_id":"apply"}}`)
	_, err := ParseWSMessage(data)
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestParseWSMessage_InvalidJSON(t *testing.T) {
	data := []byte(`not json`)
	_, err := ParseWSMessage(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEncodeWSMessage(t *testing.T) {
	msg := &WSMessage{
		Type:    WSMsgPong,
		Payload: json.RawMessage(`null`),
	}
	data, err := EncodeWSMessage(msg)
	if err != nil {
		t.Fatalf("EncodeWSMessage: %v", err)
	}
	if !strings.Contains(string(data), "pong") {
		t.Error("encoded message should contain 'pong'")
	}
}

func TestNewWSMessage(t *testing.T) {
	msg, err := NewWSMessage(WSMsgError, WSErrorPayload{Code: 500, Message: "internal error"})
	if err != nil {
		t.Fatalf("NewWSMessage: %v", err)
	}
	if msg.Type != WSMsgError {
		t.Errorf("Type = %q, want error", msg.Type)
	}
}

func TestNewUIFullMessage(t *testing.T) {
	ui := &GeneratedUI{
		TaskID:  "t1",
		Format:  FormatHTML,
		Code:    "<div>Hello</div>",
		Sandbox: true,
	}
	msg, err := NewUIFullMessage(ui)
	if err != nil {
		t.Fatalf("NewUIFullMessage: %v", err)
	}
	if msg.Type != WSMsgUIFull {
		t.Errorf("Type = %q, want ui_full", msg.Type)
	}

	var payload WSUIFullPayload
	json.Unmarshal(msg.Payload, &payload)
	if payload.TaskID != "t1" {
		t.Errorf("payload TaskID = %q, want t1", payload.TaskID)
	}
	if payload.HTML != "<div>Hello</div>" {
		t.Errorf("payload HTML = %q", payload.HTML)
	}
}

func TestNewUIStreamMessage(t *testing.T) {
	msg, err := NewUIStreamMessage("<div>chunk", false)
	if err != nil {
		t.Fatalf("NewUIStreamMessage: %v", err)
	}
	if msg.Type != WSMsgUIStream {
		t.Errorf("Type = %q, want ui_stream", msg.Type)
	}

	var payload WSUIStreamPayload
	json.Unmarshal(msg.Payload, &payload)
	if payload.Chunk != "<div>chunk" {
		t.Errorf("Chunk = %q", payload.Chunk)
	}
	if payload.Done {
		t.Error("Done should be false")
	}
}

func TestNewErrorMessage(t *testing.T) {
	msg, err := NewErrorMessage(404, "not found")
	if err != nil {
		t.Fatalf("NewErrorMessage: %v", err)
	}

	var payload WSErrorPayload
	json.Unmarshal(msg.Payload, &payload)
	if payload.Code != 404 {
		t.Errorf("Code = %d, want 404", payload.Code)
	}
	if payload.Message != "not found" {
		t.Errorf("Message = %q", payload.Message)
	}
}

func TestParseActionPayload(t *testing.T) {
	msg := &WSMessage{
		Type:    WSMsgAction,
		Payload: json.RawMessage(`{"action_id":"deploy","data":null}`),
	}
	p, err := ParseActionPayload(msg)
	if err != nil {
		t.Fatalf("ParseActionPayload: %v", err)
	}
	if p.ActionID != "deploy" {
		t.Errorf("ActionID = %q, want deploy", p.ActionID)
	}
}

func TestParseInputPayload(t *testing.T) {
	msg := &WSMessage{
		Type:    WSMsgInput,
		Payload: json.RawMessage(`{"text":"hello world"}`),
	}
	p, err := ParseInputPayload(msg)
	if err != nil {
		t.Fatalf("ParseInputPayload: %v", err)
	}
	if p.Text != "hello world" {
		t.Errorf("Text = %q", p.Text)
	}
}

func TestParseCancelPayload(t *testing.T) {
	msg := &WSMessage{
		Type:    WSMsgCancel,
		Payload: json.RawMessage(`{"reason":"user"}`),
	}
	p, err := ParseCancelPayload(msg)
	if err != nil {
		t.Fatalf("ParseCancelPayload: %v", err)
	}
	if p.Reason != "user" {
		t.Errorf("Reason = %q", p.Reason)
	}
}

func TestParseUIFeedbackPayload(t *testing.T) {
	msg := &WSMessage{
		Type:    WSMsgUIFeedback,
		Payload: json.RawMessage(`{"task_id":"t1","scrolled":true,"actions_used":["apply"]}`),
	}
	p, err := ParseUIFeedbackPayload(msg)
	if err != nil {
		t.Fatalf("ParseUIFeedbackPayload: %v", err)
	}
	if p.TaskID != "t1" {
		t.Errorf("TaskID = %q", p.TaskID)
	}
	if !p.Scrolled {
		t.Error("Scrolled should be true")
	}
	if len(p.ActionsUsed) != 1 || p.ActionsUsed[0] != "apply" {
		t.Errorf("ActionsUsed = %v", p.ActionsUsed)
	}
}

// --- Pipeline Stage Message Tests ---

func TestNewPipelineStageMessage(t *testing.T) {
	msg, err := NewPipelineStageMessage("task_42", 3, "plan", "started", "", 0)
	if err != nil {
		t.Fatalf("NewPipelineStageMessage: %v", err)
	}
	if msg.Type != WSMsgPipelineStage {
		t.Errorf("Type = %q, want pipeline_stage", msg.Type)
	}

	var p WSPipelineStagePayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.TaskID != "task_42" {
		t.Errorf("TaskID = %q, want task_42", p.TaskID)
	}
	if p.Stage != 3 {
		t.Errorf("Stage = %d, want 3", p.Stage)
	}
	if p.Name != "plan" {
		t.Errorf("Name = %q, want plan", p.Name)
	}
	if p.Status != "started" {
		t.Errorf("Status = %q, want started", p.Status)
	}
}

func TestNewPipelineStageMessage_Completed(t *testing.T) {
	msg, err := NewPipelineStageMessage("task_99", 7, "present", "completed", "UI generated", 1500)
	if err != nil {
		t.Fatalf("NewPipelineStageMessage: %v", err)
	}

	var p WSPipelineStagePayload
	json.Unmarshal(msg.Payload, &p)
	if p.Summary != "UI generated" {
		t.Errorf("Summary = %q, want 'UI generated'", p.Summary)
	}
	if p.DurMs != 1500 {
		t.Errorf("DurMs = %d, want 1500", p.DurMs)
	}
}

func TestNewPipelineStageMessage_Error(t *testing.T) {
	msg, err := NewPipelineStageMessage("task_err", 5, "execute", "error", "LLM timeout", 3000)
	if err != nil {
		t.Fatal(err)
	}

	var p WSPipelineStagePayload
	json.Unmarshal(msg.Payload, &p)
	if p.Status != "error" {
		t.Errorf("Status = %q, want error", p.Status)
	}
	if p.Stage != 5 {
		t.Errorf("Stage = %d, want 5", p.Stage)
	}
}

func TestPipelineStageMessage_RoundTrip(t *testing.T) {
	msg, _ := NewPipelineStageMessage("task_rt", 10, "go_live", "completed", "done", 500)
	data, err := EncodeWSMessage(msg)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseWSMessage(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Type != WSMsgPipelineStage {
		t.Errorf("round-trip Type = %q, want pipeline_stage", parsed.Type)
	}

	var p WSPipelineStagePayload
	json.Unmarshal(parsed.Payload, &p)
	if p.TaskID != "task_rt" || p.Stage != 10 || p.Name != "go_live" {
		t.Errorf("round-trip payload mismatch: %+v", p)
	}
}

func TestWSMsgPipelineStage_Constant(t *testing.T) {
	if WSMsgPipelineStage != "pipeline_stage" {
		t.Errorf("WSMsgPipelineStage = %q, want pipeline_stage", WSMsgPipelineStage)
	}
}

func TestPipelineStagePayload_AllStages(t *testing.T) {
	stages := []struct {
		n    int
		name string
	}{
		{1, "intake"}, {2, "clarify"}, {3, "plan"}, {4, "agent"},
		{5, "execute"}, {6, "review"}, {7, "present"}, {8, "meta"},
		{9, "reflect"}, {10, "go_live"},
	}
	for _, s := range stages {
		msg, err := NewPipelineStageMessage("t1", s.n, s.name, "started", "", 0)
		if err != nil {
			t.Errorf("stage %d (%s): %v", s.n, s.name, err)
			continue
		}
		var p WSPipelineStagePayload
		json.Unmarshal(msg.Payload, &p)
		if p.Stage != s.n || p.Name != s.name {
			t.Errorf("stage %d: got stage=%d name=%q", s.n, p.Stage, p.Name)
		}
	}
}

// --- Frame I/O Tests ---

func TestWriteReadFrame_Short(t *testing.T) {
	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()

	payload := []byte(`{"type":"ping","payload":null}`)

	go func() {
		w := bufio.NewWriter(pw)
		writeClientFrame(w, 1, payload)
	}()

	r := bufio.NewReader(pr)
	opcode, data, err := readFrame(r)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if opcode != 1 {
		t.Errorf("opcode = %d, want 1 (text)", opcode)
	}
	if string(data) != string(payload) {
		t.Errorf("payload = %q, want %q", string(data), string(payload))
	}
}

func TestWriteReadFrame_Medium(t *testing.T) {
	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()

	payload := make([]byte, 200)
	for i := range payload {
		payload[i] = byte('a' + (i % 26))
	}

	go func() {
		w := bufio.NewWriter(pw)
		writeClientFrame(w, 1, payload)
	}()

	r := bufio.NewReader(pr)
	_, data, err := readFrame(r)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if len(data) != 200 {
		t.Errorf("payload len = %d, want 200", len(data))
	}
}

func TestWriteFrame_ServerUnmasked(t *testing.T) {
	pr, pw := net.Pipe()
	defer pr.Close()
	defer pw.Close()

	payload := []byte("hello server")

	go func() {
		w := bufio.NewWriter(pw)
		writeFrame(w, wsOpText, payload)
	}()

	buf := make([]byte, 100)
	n, _ := pr.Read(buf)

	if n < 2 {
		t.Fatal("expected at least 2 bytes")
	}
	if buf[0] != 0x81 {
		t.Errorf("byte 0 = %x, want 0x81", buf[0])
	}
	if buf[1]&0x80 != 0 {
		t.Error("server frames should NOT be masked")
	}
}

// --- Server Integration Tests ---

func TestWSServer_StartStop(t *testing.T) {
	srv := NewWSServer(":0")
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	addr := srv.Addr()
	if addr == "" {
		t.Fatal("server addr should not be empty after start")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
}

func TestWSServer_ClientConnect(t *testing.T) {
	srv := NewWSServer(":0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	client := dialWS(t, srv.Addr())
	defer client.conn.Close()

	time.Sleep(50 * time.Millisecond)
	if srv.ClientCount() != 1 {
		t.Errorf("ClientCount = %d, want 1", srv.ClientCount())
	}
}

func TestWSServer_ClientDisconnect(t *testing.T) {
	srv := NewWSServer(":0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	client := dialWS(t, srv.Addr())
	time.Sleep(50 * time.Millisecond)

	if srv.ClientCount() != 1 {
		t.Fatalf("ClientCount = %d, want 1", srv.ClientCount())
	}

	client.conn.Close()
	time.Sleep(100 * time.Millisecond)

	if srv.ClientCount() != 0 {
		t.Errorf("ClientCount = %d, want 0 after disconnect", srv.ClientCount())
	}
}

func TestWSServer_BroadcastUI(t *testing.T) {
	srv := NewWSServer(":0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	client := dialWS(t, srv.Addr())
	defer client.conn.Close()
	time.Sleep(50 * time.Millisecond)

	ui := &GeneratedUI{
		TaskID: "task_broadcast",
		Format: FormatHTML,
		Code:   "<div>Broadcast Test</div>",
	}

	if err := srv.BroadcastUI(ui); err != nil {
		t.Fatalf("BroadcastUI: %v", err)
	}

	msg := client.readMessage(t)
	if msg.Type != WSMsgUIFull {
		t.Errorf("Type = %q, want ui_full", msg.Type)
	}

	var payload WSUIFullPayload
	json.Unmarshal(msg.Payload, &payload)
	if payload.TaskID != "task_broadcast" {
		t.Errorf("TaskID = %q, want task_broadcast", payload.TaskID)
	}
}

func TestWSServer_ReconnectGetsCachedUI(t *testing.T) {
	srv := NewWSServer(":0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Broadcast UI without any clients — caches it.
	ui := &GeneratedUI{
		TaskID: "task_cached",
		Format: FormatHTML,
		Code:   "<div>Cached</div>",
	}
	srv.BroadcastUI(ui)

	// Now connect — should receive the cached UI immediately after handshake.
	client := dialWS(t, srv.Addr())
	defer client.conn.Close()

	msg := client.readMessage(t)
	if msg.Type != WSMsgUIFull {
		t.Errorf("Type = %q, want ui_full", msg.Type)
	}
	var payload WSUIFullPayload
	json.Unmarshal(msg.Payload, &payload)
	if payload.TaskID != "task_cached" {
		t.Errorf("TaskID = %q, want task_cached", payload.TaskID)
	}
}

func TestWSServer_OnMessage(t *testing.T) {
	srv := NewWSServer(":0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received *WSMessage
	var receivedConnID string
	var mu sync.Mutex

	srv.OnMessage(func(connID string, msg *WSMessage) {
		mu.Lock()
		received = msg
		receivedConnID = connID
		mu.Unlock()
	})

	go srv.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	client := dialWS(t, srv.Addr())
	defer client.conn.Close()
	time.Sleep(50 * time.Millisecond)

	client.sendMessage(t, WSMessage{
		Type:    WSMsgAction,
		Payload: json.RawMessage(`{"action_id":"deploy"}`),
	})

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if received == nil {
		t.Fatal("OnMessage callback not called")
	}
	if received.Type != WSMsgAction {
		t.Errorf("received type = %q, want action", received.Type)
	}
	if receivedConnID == "" {
		t.Error("connID should not be empty")
	}
}

func TestWSServer_PingPong(t *testing.T) {
	srv := NewWSServer(":0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	client := dialWS(t, srv.Addr())
	defer client.conn.Close()
	time.Sleep(50 * time.Millisecond)

	client.sendMessage(t, WSMessage{
		Type:    WSMsgPing,
		Payload: json.RawMessage(`null`),
	})

	msg := client.readMessage(t)
	if msg.Type != WSMsgPong {
		t.Errorf("expected pong, got %q", msg.Type)
	}
}

// --- Test helpers ---

// testWSClient wraps a WebSocket connection with a buffered reader
// to avoid losing data between the handshake and subsequent reads.
type testWSClient struct {
	conn net.Conn
	br   *bufio.Reader
}

// dialWS performs a WebSocket handshake and returns a test client
// with a shared buffered reader that preserves any extra bytes read
// during the handshake.
func dialWS(t *testing.T, addr string) *testWSClient {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	key := base64.StdEncoding.EncodeToString([]byte("test-key-1234567"))
	req := fmt.Sprintf("GET /ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", addr, key)
	conn.Write([]byte(req))

	// Use a buffered reader so any extra bytes (WS frames sent right after
	// the handshake) are preserved in the buffer for later reads.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	br := bufio.NewReader(conn)

	// Read HTTP response line by line until we hit the empty line.
	var respLines []string
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("handshake read: %v", err)
		}
		respLines = append(respLines, line)
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	resp := strings.Join(respLines, "")
	if !strings.Contains(resp, "101") {
		t.Fatalf("expected 101 response, got: %s", resp)
	}

	// Verify Sec-WebSocket-Accept.
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	expectedAccept := base64.StdEncoding.EncodeToString(h.Sum(nil))
	if !strings.Contains(resp, expectedAccept) {
		t.Fatalf("missing correct Sec-WebSocket-Accept in response")
	}

	conn.SetReadDeadline(time.Time{})
	return &testWSClient{conn: conn, br: br}
}

// readMessage reads a single WebSocket text frame and parses it as WSMessage.
func (c *testWSClient) readMessage(t *testing.T) *WSMessage {
	t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	defer c.conn.SetReadDeadline(time.Time{})

	// Read frame header.
	b0, err := c.br.ReadByte()
	if err != nil {
		t.Fatalf("read frame b0: %v", err)
	}
	b1, err := c.br.ReadByte()
	if err != nil {
		t.Fatalf("read frame b1: %v", err)
	}

	_ = b0
	length := uint64(b1 & 0x7f)
	switch length {
	case 126:
		var buf [2]byte
		if _, err := io.ReadFull(c.br, buf[:]); err != nil {
			t.Fatalf("read extended length: %v", err)
		}
		length = uint64(binary.BigEndian.Uint16(buf[:]))
	case 127:
		var buf [8]byte
		if _, err := io.ReadFull(c.br, buf[:]); err != nil {
			t.Fatalf("read extended length: %v", err)
		}
		length = binary.BigEndian.Uint64(buf[:])
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		t.Fatalf("read payload (%d bytes): %v", length, err)
	}

	msg, err := ParseWSMessage(payload)
	if err != nil {
		t.Fatalf("parse received message: %v", err)
	}
	return msg
}

// sendMessage sends a masked text frame from the "client" side.
func (c *testWSClient) sendMessage(t *testing.T, msg WSMessage) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	w := bufio.NewWriter(c.conn)
	writeClientFrame(w, wsOpText, data)
}

// writeClientFrame writes a masked text frame (as a client would).
func writeClientFrame(w *bufio.Writer, opcode byte, data []byte) {
	w.WriteByte(0x80 | opcode)

	length := len(data)
	switch {
	case length <= 125:
		w.WriteByte(0x80 | byte(length))
	case length <= 65535:
		w.WriteByte(0x80 | 126)
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], uint16(length))
		w.Write(buf[:])
	default:
		w.WriteByte(0x80 | 127)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(length))
		w.Write(buf[:])
	}

	maskKey := [4]byte{0x12, 0x34, 0x56, 0x78}
	w.Write(maskKey[:])

	masked := make([]byte, len(data))
	for i, b := range data {
		masked[i] = b ^ maskKey[i%4]
	}
	w.Write(masked)
	w.Flush()
}
