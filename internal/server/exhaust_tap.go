package server

import (
	"bufio"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"time"
)

/*
TapSink, "egzoz çıkışı"dır (Yol B). ExhaustEvent'leri ayrı bir TCP portu
üzerinden bağlı dinleyicilere (harici uygulamalar / e5-tap) yayınlar.

Yayın formatı: NDJSON — her olay tek satır JSON + '\n'. Bu sayede herhangi
bir dilde (jq dahil) kolayca tüketilebilir.

Güvenlik:
  - Ortak anahtar (E5_EXHAUST_KEY) ayarlıysa, dinleyici bağlandığında ilk
    satır olarak anahtarı göndermek zorundadır; sabit-zamanlı karşılaştırma
    ile doğrulanır, aksi halde bağlantı kapatılır.
  - TLS, ana sunucuyla aynı sertifika üzerinden kullanılabilir.

Backpressure: Her dinleyicinin kendi tamponlu kuyruğu vardır. Yavaş tüketici
kuyruğu doldurursa o dinleyici için olaylar düşürülür; broker etkilenmez.
*/
type TapSink struct {
	mu          sync.RWMutex
	subscribers map[*tapSubscriber]struct{}
	authKey     string
	port        string
}

type tapSubscriber struct {
	conn  net.Conn
	queue chan []byte
}

func NewTapSink(port, authKey string) *TapSink {
	return &TapSink{
		subscribers: make(map[*tapSubscriber]struct{}),
		authKey:     authKey,
		port:        port,
	}
}

// Write, olayı NDJSON olarak tüm bağlı dinleyicilere yayınlar (non-blocking).
func (t *TapSink) Write(ev ExhaustEvent) {
	line, err := json.Marshal(ev)
	if err != nil {
		return
	}
	line = append(line, '\n')

	t.mu.RLock()
	for sub := range t.subscribers {
		select {
		case sub.queue <- line:
		default:
			// Yavaş tüketici: bu dinleyici için olayı düşür.
		}
	}
	t.mu.RUnlock()
}

// Serve, tap portunu dinlemeye başlar. tlsConf nil ise düz TCP kullanılır.
func (t *TapSink) Serve(tlsConf *tls.Config) error {
	var ln net.Listener
	var err error
	if tlsConf != nil {
		ln, err = tls.Listen("tcp", ":"+t.port, tlsConf)
	} else {
		ln, err = net.Listen("tcp", ":"+t.port)
	}
	if err != nil {
		return err
	}
	go t.acceptLoop(ln)
	return nil
}

func (t *TapSink) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go t.handleSubscriber(conn)
	}
}

func (t *TapSink) handleSubscriber(conn net.Conn) {
	// Anahtar doğrulaması (varsa).
	if t.authKey != "" {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return
		}
		provided := strings.TrimRight(line, "\r\n")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(t.authKey)) != 1 {
			conn.Close()
			return
		}
		conn.SetReadDeadline(time.Time{})
	}

	sub := &tapSubscriber{conn: conn, queue: make(chan []byte, 256)}
	t.mu.Lock()
	t.subscribers[sub] = struct{}{}
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.subscribers, sub)
		t.mu.Unlock()
		conn.Close()
	}()

	// Bağlantı kopuşunu erken yakalamak için okuma goroutine'i.
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 256)
		for {
			if _, err := conn.Read(buf); err != nil {
				close(done)
				return
			}
		}
	}()

	for {
		select {
		case line := <-sub.queue:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := conn.Write(line); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
