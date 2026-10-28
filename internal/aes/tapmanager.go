package aes

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultDialTimeout    = 10 * time.Second
	defaultReconnectDelay = 2 * time.Second
	initialScannerBuffer  = 64 * 1024
	maxScannerBuffer      = 4 * 1024 * 1024
)

type TapEvent struct {
	Time              time.Time `json:"time"`
	Level             string    `json:"level"`
	Kind              string    `json:"kind"`
	Instance          string    `json:"instance"`
	Group             string    `json:"group"`
	Subject           string    `json:"subject"`
	MessageId         string    `json:"messageId"`
	OriginalMessageId string    `json:"originalMessageId"`
	Remote            string    `json:"remote"`
	Content           string    `json:"content"`
	Err               string    `json:"err"`
	Msg               string    `json:"msg"`
	IsError           bool      `json:"isError"`
}

type ConnectionInfo struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	Key       string `json:"key"`
	UseTLS    bool   `json:"useTLS"`
	CAFile    string `json:"caFile"`
	Reconnect bool   `json:"reconnect"`
}

type TapManager struct {
	willClose  atomic.Bool
	mu         sync.Mutex
	conn       net.Conn
	onTapEvent func(TapEvent)
}

// NewTapManager creates a new manager with an optional event callback.
func NewTapManager(onTapEvent func(TapEvent)) *TapManager {
	return &TapManager{onTapEvent: onTapEvent}
}

func (tm *TapManager) Close() {
	tm.willClose.Store(true)
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.conn != nil {
		tm.conn.Close()
	}
}

func (tm *TapManager) HandleEvent(ev TapEvent) {
	if tm.onTapEvent != nil {
		tm.onTapEvent(ev)
	}
}

func (tm *TapManager) shouldClose() bool {
	return tm.willClose.Load()
}

func (tm *TapManager) Connect(info ConnectionInfo) {
	addr := net.JoinHostPort(info.Host, info.Port)
	for {
		if tm.shouldClose() {
			return
		}

		err := tm.stream(addr, info)
		if err != nil && !tm.shouldClose() {
			fmt.Fprintf(os.Stderr, "e5-tap: %v\n", err)
		}

		if !info.Reconnect || tm.shouldClose() {
			return
		}

		time.Sleep(defaultReconnectDelay)
		if tm.shouldClose() {
			return
		}
		fmt.Fprintln(os.Stderr, "e5-tap: reconnecting...")
	}
}

// dial creates a TCP/TLS connection depending on the given options.
func dial(addr string, useTLS, insecure bool, caFile string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	defer cancel()
	return dialContext(ctx, addr, useTLS, insecure, caFile)
}

func dialContext(ctx context.Context, addr string, useTLS, insecure bool, caFile string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: defaultDialTimeout}
	if !useTLS {
		return dialer.DialContext(ctx, "tcp", addr)
	}

	tlsConf := &tls.Config{InsecureSkipVerify: insecure} // #nosec G402 -- -insecure yalnızca açıkça istenirse
	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConf.RootCAs = pool
	}
	return (&tls.Dialer{NetDialer: dialer, Config: tlsConf}).DialContext(ctx, "tcp", addr)
}

// stream connects to remote tap source and processes incoming NDJSON events.
func (tm *TapManager) stream(addr string, info ConnectionInfo) error {
	if tm.shouldClose() {
		return fmt.Errorf("connection closed by request")
	}
	conn, err := dial(addr, info.UseTLS, false, info.CAFile)
	if err != nil {
		return err
	}
	defer conn.Close()
	tm.mu.Lock()
	tm.conn = conn
	closed := tm.shouldClose()
	tm.mu.Unlock()
	if closed {
		return nil
	}

	// Ortak anahtar varsa ilk satır olarak gönder.
	if info.Key != "" {
		if _, err := conn.Write([]byte(info.Key + "\n")); err != nil {
			return fmt.Errorf("failed to send key: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "e5-tap: connected to %s\n", addr)

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, initialScannerBuffer), maxScannerBuffer)
	for scanner.Scan() {
		line := scanner.Bytes()
		var ev TapEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			fmt.Fprintf(os.Stderr, "aes tap: invalid event: %v\n", err)
			continue
		}

		tm.HandleEvent(ev)

		if tm.shouldClose() {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("connection lost: %w", err)
	}
	return fmt.Errorf("connection closed by server")
}
