package engine5client

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shamaton/msgpack"
)

const (
	CtConnect        = "CONNECT"
	CtConnectSuccess = "CONNECT_SUCCESS"
	CtConnectError   = "CONNECT_ERROR"
	CtEvent          = "EVENT"
	CtRecieved       = "RECIEVED"
	CtRequest        = "REQUEST"
	CtResponse       = "RESPONSE"
	CtResponseError  = "RESPONSE_ERROR"
	CtListen         = "LISTEN"
	CtClose          = "CLOSE"
	CtUnauthorized   = "UNAUTHORIZED"
	CtError          = "ERROR"
	CtResponseErrorE5 = "E5"
	CtResponseErrorCL = "CLIENT"
)

type Payload struct {
	Command            string `json:"command"`
	Content            string `json:"content"`
	Subject            string `json:"subject"`
	InstanceId         string `json:"instanceId"`
	MessageId          string `json:"messageId"`
	ResponseOfMessageId string `json:"responseOfMessageId"`
	ResponseErrorSide  string
	AuthKey            string `json:"authKey"`
	Completed          bool   `json:"completed"`
	InstanceGroup      string `json:"instance_group"`
}

type Callback func(data any) any

type Options struct {
	Host           string
	Port           int
	InstanceID     string
	InstanceGroup  string
	AuthKey        string
	TLSConfig      *tls.Config
	ConnectTimeout time.Duration
	RequestTimeout time.Duration
}

type connectionState int

const (
	stateClosed connectionState = iota
	stateConnecting
	stateConnected
)

type responseResult struct {
	payload Payload
	err     error
}

// Client speaks the Engine5 wire protocol over TCP/TLS.
type Client struct {
	opts Options

	mu            sync.Mutex
	writeMu       sync.Mutex
	conn          net.Conn
	state         connectionState
	connectWait    chan error
	pending       map[string]chan responseResult
	listeners     map[string][]Callback
	instanceID    string
	instanceGroup  string
	messageSeq    uint64
}

// NewClient creates a protocol client with sane defaults.
func NewClient(opts Options) *Client {
	if opts.InstanceID == "" {
		opts.InstanceID = "default-id"
	}
	if opts.InstanceGroup == "" {
		opts.InstanceGroup = opts.InstanceID
	}
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = 10 * time.Second
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 30 * time.Second
	}

	return &Client{
		opts:         opts,
		state:        stateClosed,
		pending:      make(map[string]chan responseResult),
		listeners:    make(map[string][]Callback),
		instanceID:   opts.InstanceID,
		instanceGroup: opts.InstanceGroup,
	}
}

func (c *Client) InstanceID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.instanceID
}

func (c *Client) InstanceGroup() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.instanceGroup
}

func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == stateConnected
}

func (c *Client) Connect(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.Lock()
	switch c.state {
	case stateConnected:
		c.mu.Unlock()
		return nil
	case stateConnecting:
		wait := c.connectWait
		c.mu.Unlock()
		return c.waitForConnect(ctx, wait)
	}

	addr := net.JoinHostPort(c.opts.Host, strconv.Itoa(c.opts.Port))
	dialer := &net.Dialer{Timeout: c.opts.ConnectTimeout}

	var (
		conn net.Conn
		err  error
	)

	if c.opts.TLSConfig != nil {
		tlsCfg := c.opts.TLSConfig.Clone()
		if tlsCfg.ServerName == "" {
			tlsCfg.ServerName = c.opts.Host
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		c.mu.Unlock()
		return err
	}

	c.conn = conn
	c.state = stateConnecting
	c.connectWait = make(chan error, 1)
	wait := c.connectWait
	c.mu.Unlock()

	go c.readLoop(conn)

	connectCtx := ctx
	if _, hasDeadline := connectCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(connectCtx, c.opts.ConnectTimeout)
		defer cancel()
	}

	if err := c.writePayload(connectCtx, Payload{
		Command:      CtConnect,
		InstanceId:   c.opts.InstanceID,
		InstanceGroup: c.opts.InstanceGroup,
		AuthKey:      c.opts.AuthKey,
	}); err != nil {
		c.failConnection(err)
		return err
	}

	if err := c.waitForConnect(connectCtx, wait); err != nil {
		c.failConnection(err)
		return err
	}

	return nil
}

func (c *Client) Listen(ctx context.Context, subject string, callback Callback) error {
	if subject == "" {
		return errors.New("subject cannot be empty")
	}
	if callback == nil {
		return errors.New("callback cannot be nil")
	}

	c.mu.Lock()
	c.listeners[subject] = append(c.listeners[subject], callback)
	c.mu.Unlock()

	if err := c.ensureConnected(ctx); err != nil {
		return err
	}

	return c.writePayload(ctx, Payload{
		Command:   CtListen,
		Subject:   subject,
		MessageId: c.generateMessageID(),
	})
}

func (c *Client) SendEvent(ctx context.Context, subject string, data any) error {
	if subject == "" {
		return errors.New("subject cannot be empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return err
	}

	return c.writePayload(ctx, Payload{
		Command:   CtEvent,
		Subject:   subject,
		Content:   c.stringifyData(data),
		MessageId: c.generateMessageID(),
	})
}

func (c *Client) Request(ctx context.Context, subject string, data any) (any, error) {
	if subject == "" {
		return nil, errors.New("subject cannot be empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	requestID := c.generateMessageID()
	resultCh := make(chan responseResult, 1)

	c.mu.Lock()
	c.pending[requestID] = resultCh
	c.mu.Unlock()

	if err := c.writePayload(ctx, Payload{
		Command:   CtRequest,
		Subject:   subject,
		Content:   c.stringifyData(data),
		MessageId: requestID,
	}); err != nil {
		c.removePending(requestID)
		return nil, err
	}

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	if _, hasDeadline := requestCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(requestCtx, c.opts.RequestTimeout)
		defer cancel()
	}

	select {
	case result := <-resultCh:
		c.removePending(requestID)
		if result.err != nil {
			return nil, result.err
		}
		decoded, err := c.decodeContent(result.payload.Content)
		if err != nil {
			return nil, err
		}
		return decoded, nil
	case <-requestCtx.Done():
		c.removePending(requestID)
		return nil, requestCtx.Err()
	}
}

func (c *Client) RequestInto(ctx context.Context, subject string, data any, out any) error {
	decoded, err := c.Request(ctx, subject, data)
	if err != nil {
		return err
	}
	if out == nil || decoded == nil {
		return nil
	}

	encoded, err := json.Marshal(decoded)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, out)
}

func (c *Client) Close() error {
	c.mu.Lock()
	if c.state == stateClosed {
		c.mu.Unlock()
		return nil
	}
	conn := c.conn
	c.mu.Unlock()

	if conn != nil {
		_ = c.writePayload(context.Background(), Payload{Command: CtClose})
	}

	c.failConnection(io.EOF)
	return nil
}

func (c *Client) ensureConnected(ctx context.Context) error {
	return c.Connect(ctx)
}

func (c *Client) waitForConnect(ctx context.Context, wait chan error) error {
	if wait == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case err := <-wait:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) readLoop(conn net.Conn) {
	for {
		var length uint32
		if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
			c.failConnection(err)
			return
		}

		data := make([]byte, length)
		if _, err := io.ReadFull(conn, data); err != nil {
			c.failConnection(err)
			return
		}

		var payload Payload
		if err := msgpack.Unmarshal(data, &payload); err != nil {
			c.failConnection(fmt.Errorf("msgpack decode failed: %w", err))
			return
		}

		c.handleIncoming(payload)
	}
}

func (c *Client) handleIncoming(payload Payload) {
	switch payload.Command {
	case CtConnectSuccess:
		c.mu.Lock()
		c.instanceID = payload.InstanceId
		if payload.InstanceGroup != "" {
			c.instanceGroup = payload.InstanceGroup
		}
		c.state = stateConnected
		wait := c.connectWait
		c.connectWait = nil
		c.mu.Unlock()
		if wait != nil {
			select {
			case wait <- nil:
			default:
			}
		}

	case CtConnectError:
		if payload.Content == "" {
			payload.Content = "connection rejected"
		}
		c.failConnection(errors.New(payload.Content))

	case CtEvent:
		c.dispatchEvent(payload)

	case CtRequest:
		c.handleRequest(payload)

	case CtResponse:
		c.resolveRequest(payload)

	case CtResponseError:
		c.rejectRequest(payload)

	case CtClose:
		c.failConnection(io.EOF)
	}
}

func (c *Client) dispatchEvent(payload Payload) {
	content, err := c.decodeContent(payload.Content)
	if err != nil {
		return
	}

	c.mu.Lock()
	callbacks := append([]Callback(nil), c.listeners[payload.Subject]...)
	c.mu.Unlock()

	for _, cb := range callbacks {
		callback := cb
		go callback(content)
	}
}

func (c *Client) handleRequest(payload Payload) {
	content, err := c.decodeContent(payload.Content)
	if err != nil {
		return
	}

	c.mu.Lock()
	callbacks := append([]Callback(nil), c.listeners[payload.Subject]...)
	c.mu.Unlock()
	if len(callbacks) == 0 {
		return
	}

	responseValue := callbacks[0](content)
	_ = c.writePayload(context.Background(), Payload{
		Command:            CtResponse,
		Subject:            payload.Subject,
		Content:            c.stringifyData(responseValue),
		MessageId:          c.generateMessageID(),
		ResponseOfMessageId: payload.MessageId,
	})
}

func (c *Client) resolveRequest(payload Payload) {
	c.mu.Lock()
	ch := c.pending[payload.ResponseOfMessageId]
	c.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- responseResult{payload: payload}:
	default:
	}
}

func (c *Client) rejectRequest(payload Payload) {
	c.mu.Lock()
	ch := c.pending[payload.ResponseOfMessageId]
	c.mu.Unlock()
	if ch == nil {
		return
	}
	msg := payload.Content
	if msg == "" {
		msg = "request rejected"
	}
	select {
	case ch <- responseResult{err: errors.New(msg)}:
	default:
	}
}

func (c *Client) removePending(id string) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *Client) failConnection(err error) {
	if err == nil {
		err = io.EOF
	}

	c.mu.Lock()
	if c.state == stateClosed && c.conn == nil && c.connectWait == nil {
		c.mu.Unlock()
		return
	}
	conn := c.conn
	c.conn = nil
	c.state = stateClosed
	connectWait := c.connectWait
	c.connectWait = nil
	pending := c.pending
	c.pending = make(map[string]chan responseResult)
	c.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	if connectWait != nil {
		select {
		case connectWait <- err:
		default:
		}
	}
	for _, ch := range pending {
		select {
		case ch <- responseResult{err: err}:
		default:
		}
	}
}

func (c *Client) writePayload(ctx context.Context, payload Payload) error {
	if ctx == nil {
		ctx = context.Background()
	}

	data, err := msgpack.Marshal(payload)
	if err != nil {
		return err
	}

	frame := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(data)))
	copy(frame[4:], data)

	c.mu.Lock()
	conn := c.conn
	state := c.state
	c.mu.Unlock()
	if conn == nil || state == stateClosed {
		return errors.New("not connected")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return writeFull(conn, frame)
}

func writeFull(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func (c *Client) nextMessageID() string {
	seq := atomic.AddUint64(&c.messageSeq, 1)
	return fmt.Sprintf("%d_%d", time.Now().UnixNano(), seq)
}

func (c *Client) generateMessageID() string {
	return c.nextMessageID()
}

func (c *Client) stringifyData(data any) string {
	if data == nil {
		return "undefined"
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprint(data)
	}
	return string(encoded)
}

func (c *Client) decodeContent(raw string) (any, error) {
	if raw == "" || raw == "undefined" {
		return nil, nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return raw, nil
	}
	return decoded, nil
}
