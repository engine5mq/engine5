package aes

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

const (
	defaultDialTimeout    = 10 * time.Second
	defaultReconnectDelay = 2 * time.Second
	initialScannerBuffer  = 64 * 1024
	maxScannerBuffer      = 4 * 1024 * 1024
)

type TapEvent struct {
	Time      time.Time `json:"time"`
	Level     string    `json:"level"`
	Kind      string    `json:"kind"`
	Instance  string    `json:"instance"`
	Group     string    `json:"group"`
	Subject   string    `json:"subject"`
	MessageId string    `json:"messageId"`
	Remote    string    `json:"remote"`
	Content   string    `json:"content"`
	Err       string    `json:"err"`
	Msg       string    `json:"msg"`
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
	willClose  bool
	onTapEvent func(TapEvent)
}

// NewTapManager creates a new manager with an optional event callback.
func NewTapManager(onTapEvent func(TapEvent)) *TapManager {
	return &TapManager{onTapEvent: onTapEvent}
}

func (tm *TapManager) Close() {
	tm.willClose = true
}

func (tm *TapManager) HandleEvent(ev TapEvent) {
	if tm.onTapEvent != nil {
		tm.onTapEvent(ev)
	}
}

func (tm *TapManager) shouldClose() bool {
	return tm.willClose
}

func (tm *TapManager) Connect(info ConnectionInfo) {
	addr := net.JoinHostPort(info.Host, info.Port)
	for {
		if tm.shouldClose() {
			return
		}

		err := stream(addr, info.Key, info.UseTLS, false, info.CAFile, false, tm.HandleEvent, tm.shouldClose)
		if err != nil {
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
	if !useTLS {
		return net.DialTimeout("tcp", addr, defaultDialTimeout)
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
	return tls.DialWithDialer(&net.Dialer{Timeout: defaultDialTimeout}, "tcp", addr, tlsConf)
}

// stream connects to remote tap source and processes incoming NDJSON events.
func stream(addr, key string, useTLS, insecure bool, caFile string, raw bool, cb func(TapEvent), cbCloseRequest func() bool) error {
	if cbCloseRequest != nil && cbCloseRequest() {
		return fmt.Errorf("connection closed by request")
	}
	conn, err := dial(addr, useTLS, insecure, caFile)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Ortak anahtar varsa ilk satır olarak gönder.
	if key != "" {
		if _, err := conn.Write([]byte(key + "\n")); err != nil {
			return fmt.Errorf("failed to send key: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "e5-tap: connected to %s\n", addr)

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, initialScannerBuffer), maxScannerBuffer)
	for scanner.Scan() {
		line := scanner.Bytes()
		if raw {
			fmt.Println(string(line))
			continue
		}
		var ev TapEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			fmt.Println(string(line)) // parse edilemiyorsa ham bas
			continue
		}

		if cb != nil {
			cb(ev)
		}

		if cbCloseRequest != nil && cbCloseRequest() {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("connection lost: %w", err)
	}
	return fmt.Errorf("connection closed by server")
}
