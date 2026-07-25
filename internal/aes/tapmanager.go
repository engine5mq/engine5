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

// NewTapManager, verilen olay işleyici (callback) ile yeni bir TapManager oluşturur.
// onTapEvent alanı paket dışından erişilemediği için diğer paketler bu constructor'ı kullanmalıdır.
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

func (tm *TapManager) Connect(info ConnectionInfo) {
	addr := net.JoinHostPort(info.Host, info.Port)
	for {
		if err := stream(addr, info.Key, info.UseTLS, false, info.CAFile, false, tm.HandleEvent, func() bool { return tm.willClose }); err != nil {
			fmt.Fprintf(os.Stderr, "e5-tap: %v\n", err)
		}
		if !info.Reconnect {
			return
		}
		time.Sleep(2 * time.Second)
		fmt.Fprintln(os.Stderr, "e5-tap: reconnecting...")
	}
}

/**
* Verilen adres ve TLS ayarları ile bir TCP bağlantısı kurar.
* Eğer TLS kullanımı istenmiyorsa, normal bir TCP bağlantısı kurar.
* TLS kullanımı isteniyorsa, gerekli sertifika doğrulama ayarlarını yapar.
* @param addr Bağlanılacak adres (host:port formatında)
* @param useTLS TLS kullanılıp kullanılmayacağını belirten boolean değer
* @param insecure Sertifika doğrulamasını atlamak için boolean değer
* @param caFile Özel CA sertifikası dosyasının yolu (boşsa varsayılan CA'lar kullanılır)
* @return Kurulan net.Conn nesnesi ve olası hata
 */
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

/**
* Verilen adres ve TLS ayarları ile bir TCP bağlantısı kurar ve gelen NDJSON olaylarını işler.
* Eğer ortak anahtar verilmişse, bağlantı kurulduktan sonra ilk satır olarak gönderilir.
* @param addr Bağlanılacak adres (host:port formatında)
* @param key Ortak anahtar (boşsa gönderilmez)
* @param useTLS TLS kullanılıp kullanılmayacağını belirten boolean değer
* @param insecure Sertifika doğrulamasını atlamak için boolean değer
* @param caFile Özel CA sertifikası dosyasının yolu (boşsa varsayılan CA'lar kullanılır)
* @param raw Ham JSON satırlarını yazdırmak için boolean değer
* @param cb Gelen TapEvent olaylarını işlemek için callback fonksiyonu
* @param cbCloseRequest bu callback fonksiyonu boolean döner ve bağlantının kapatılması gerektiğini belirtir
* @return Olası hata
 */
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
		if cbCloseRequest != nil && cbCloseRequest() {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("connection lost: %w", err)
	}
	return fmt.Errorf("connection closed by server")
}
