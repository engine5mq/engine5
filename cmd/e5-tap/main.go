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
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type tapEvent struct {
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	host := flag.String("host", envOr("E5_EXHAUST_HOST", "localhost"), "exhaust tap host")
	port := flag.String("port", envOr("E5_EXHAUST_PORT", "3536"), "exhaust tap port")
	key := flag.String("key", os.Getenv("E5_EXHAUST_KEY"), "exhaust shared key (E5_EXHAUST_KEY)")
	useTLS := flag.Bool("tls", envOr("E5_EXHAUST_TLS", "true") == "true", "connect with TLS")
	insecure := flag.Bool("insecure", false, "skip TLS certificate verification (dev / self-signed)")
	caFile := flag.String("ca", os.Getenv("E5_EXHAUST_CA_FILE"), "CA certificate file to verify the server")
	raw := flag.Bool("raw", false, "print raw NDJSON lines instead of formatted output")
	reconnect := flag.Bool("reconnect", true, "automatically reconnect on disconnect")
	flag.Parse()

	addr := net.JoinHostPort(*host, *port)

	for {
		if err := stream(addr, *key, *useTLS, *insecure, *caFile, *raw); err != nil {
			fmt.Fprintf(os.Stderr, "e5-tap: %v\n", err)
		}
		if !*reconnect {
			return
		}
		time.Sleep(2 * time.Second)
		fmt.Fprintln(os.Stderr, "e5-tap: reconnecting...")
	}
}

func stream(addr, key string, useTLS, insecure bool, caFile string, raw bool) error {
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
		var ev tapEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			fmt.Println(string(line)) // parse edilemiyorsa ham bas
			continue
		}
		fmt.Println(format(ev))
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("connection lost: %w", err)
	}
	return fmt.Errorf("connection closed by server")
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

func format(ev tapEvent) string {
	var b strings.Builder
	ts := ev.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	b.WriteString(ts.Format("15:04:05.000"))
	b.WriteString("  ")
	b.WriteString(fmt.Sprintf("%-5s", ev.Level))
	b.WriteString("  ")
	b.WriteString(fmt.Sprintf("%-18s", ev.Kind))

	writeKV(&b, "instance", ev.Instance)
	writeKV(&b, "group", ev.Group)
	writeKV(&b, "subject", ev.Subject)
	writeKV(&b, "messageId", ev.MessageId)
	writeKV(&b, "remote", ev.Remote)
	writeKV(&b, "content", ev.Content)
	writeKV(&b, "err", ev.Err)

	if ev.Msg != "" {
		b.WriteString("  ")
		b.WriteString(ev.Msg)
	}
	return b.String()
}

func writeKV(b *strings.Builder, k, v string) {
	if v == "" {
		return
	}
	b.WriteString(" ")
	b.WriteString(k)
	b.WriteString("=")
	b.WriteString(v)
}
