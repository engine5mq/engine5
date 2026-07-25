// Command e5-tap, Engine5'in egzoz çıkışını (exhaust tap) dinleyen bağımsız
// bir araçtır. Tap portuna bağlanır, gerekiyorsa ortak anahtarı gönderir ve
// gelen NDJSON olaylarını okunabilir biçimde yazdırır.
//
// Örnek:
//
//	E5_EXHAUST_KEY=secret go run ./cmd/e5-tap -host localhost -port 3536
//	go run ./cmd/e5-tap -insecure          # self-signed sertifika ile dev
//	go run ./cmd/e5-tap -raw                # ham JSON satırları
package main

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
}

func (tm *TapManager) Connect(info ConnectionInfo) {
	addr := net.JoinHostPort(info.Host, info.Port)
	for {
		if err := stream(addr, info.Key, info.UseTLS, false, info.CAFile, false, tm.HandleEvent); err != nil {
			fmt.Fprintf(os.Stderr, "e5-tap: %v\n", err)
		}
		if !info.Reconnect {
			return
		}
		time.Sleep(2 * time.Second)
		fmt.Fprintln(os.Stderr, "e5-tap: reconnecting...")
	}
}

func (tm *TapManager) Disconnect() {
	// Bağlantıyı kesmek için gerekli işlemleri burada yapabilirsiniz.
	// Örneğin, bir bağlantı nesnesi varsa onu kapatabilirsiniz.
}

func (tm *TapManager) HandleEvent(ev TapEvent) {
	// Burada gelen olayları işleyebilirsiniz. Örneğin, konsola yazdırabilirsiniz.
	fmt.Printf("[%s] %s: %s\n", ev.Time.Format(time.RFC3339), ev.Level, ev.MessageId)
}

func dial(addr string, useTLS, insecure bool, caFile string) (net.Conn, error) {
	if !useTLS {
		return net.DialTimeout("tcp", addr, 10*time.Second)
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
	return tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsConf)
}

func stream(addr, key string, useTLS, insecure bool, caFile string, raw bool, cb func(TapEvent)) error {
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
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
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
		// fmt.Println(format(ev))
		cb(ev)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("connection lost: %w", err)
	}
	return fmt.Errorf("connection closed by server")
}
