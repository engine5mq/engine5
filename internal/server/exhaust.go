package server

import (
	"context"
	"crypto/tls"
	"log/slog"
	"os"
	"strings"
	"time"
)

/*
Sink, bir exhaust olayının nereye yazılacağını tanımlar.
Şu an yalnızca ConsoleSink var; ileride "egzoz çıkışı" için ayrı bir port
üzerinden yayın yapan bir TapSink eklenebilir (aynı arayüzü uygular).
*/
type Sink interface {
	Write(ev ExhaustEvent)
}

/*
Exhaust, Engine5'in merkezi olay/log çıkışıdır.
Tüm sunucu kodu buraya Emit() yapar; Exhaust olayı kayıtlı tüm sink'lere
dağıtır. Dağıtım ayrı bir goroutine'de yapılır ve kuyruk dolduğunda olay
DÜŞÜRÜLÜR; böylece loglama, mesaj broker'ın hot path'ini asla bloklamaz.
*/
type Exhaust struct {
	sinks          []Sink
	queue          chan ExhaustEvent
	includeContent bool
	tap            *TapSink
}

const exhaustContentMasked = "[hidden]"

// NewExhaustFromEnv, ortam değişkenlerine göre Exhaust'ı yapılandırır:
//
//	E5_ENV                       development | production  (varsayılan: development)
//	E5_LOG_LEVEL                 DEBUG | INFO | WARN | ERROR | OFF
//	                             (varsayılan: dev=DEBUG, prod=OFF — prod'da konsola basmaz)
//	E5_LOG_FORMAT                text | json  (varsayılan: dev=text, prod=json)
//	E5_EXHAUST_INCLUDE_CONTENT   true | false (varsayılan: false — hassas veri maskelenir)
//	E5_EXHAUST_ENABLE            true | false (varsayılan: false — egzoz tap portunu açar)
//	E5_EXHAUST_PORT              tap portu     (varsayılan: 3536)
//	E5_EXHAUST_KEY               tap ortak anahtarı (boşsa anahtar doğrulaması yok)
func NewExhaustFromEnv() *Exhaust {
	isProd := strings.EqualFold(getEnvWithDefault("E5_ENV", "development"), "production")

	defaultLevel := "DEBUG"
	defaultFormat := "text"
	if isProd {
		defaultLevel = "OFF" // Production'da konsola basma
		defaultFormat = "json"
	}

	ex := &Exhaust{
		queue:          make(chan ExhaustEvent, 1024),
		includeContent: strings.EqualFold(getEnvWithDefault("E5_EXHAUST_INCLUDE_CONTENT", "false"), "true"),
	}

	levelStr := strings.ToUpper(getEnvWithDefault("E5_LOG_LEVEL", defaultLevel))
	if levelStr != "OFF" {
		format := strings.ToLower(getEnvWithDefault("E5_LOG_FORMAT", defaultFormat))
		ex.sinks = append(ex.sinks, newConsoleSink(parseLevel(levelStr), format))
	}

	// Egzoz çıkışı (Yol B): etkinse TapSink oluştur. Sunucu, Serve'i
	// StartTap ile (TLS yapılandırmasıyla birlikte) başlatır.
	if strings.EqualFold(getEnvWithDefault("E5_EXHAUST_ENABLE", "false"), "true") {
		tapPort := getEnvWithDefault("E5_EXHAUST_PORT", "3536")
		tapKey := os.Getenv("E5_EXHAUST_KEY")
		ex.tap = NewTapSink(tapPort, tapKey)
		ex.sinks = append(ex.sinks, ex.tap)
	}

	go ex.run()
	return ex
}

// StartTap, etkinse egzoz tap sunucusunu dinlemeye başlatır.
// tlsConf nil ise tap portu düz TCP kullanır.
func (e *Exhaust) StartTap(tlsConf *tls.Config) {
	if e == nil || e.tap == nil {
		return
	}
	if err := e.tap.Serve(tlsConf); err != nil {
		e.Emit(ExhaustEvent{Level: slog.LevelError, Kind: KindServerError, Msg: "Failed to start exhaust tap", Err: err.Error()})
		return
	}
	secured := "no"
	if tlsConf != nil {
		secured = "yes"
	}
	e.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindServerStart, Msg: "Exhaust tap listening on port " + e.tap.port + " (tls: " + secured + ")"})
}

// Emit, bir olayı çıkışa gönderir. Asla bloklamaz: kuyruk doluysa olay düşer.
func (e *Exhaust) Emit(ev ExhaustEvent) {
	if e == nil {
		return
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	if !e.includeContent && ev.Content != "" {
		ev.Content = exhaustContentMasked
	}
	select {
	case e.queue <- ev:
	default:
		// Kuyruk dolu: olayı düşür, hot path'i bloklama.
	}
}

func (e *Exhaust) run() {
	for ev := range e.queue {
		for _, s := range e.sinks {
			s.Write(ev)
		}
	}
}

func parseLevel(s string) slog.Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

/*
ConsoleSink, olayları slog ile stdout'a yazar. Kendi minimum seviyesine
göre filtreler.
*/
type ConsoleSink struct {
	logger   *slog.Logger
	minLevel slog.Level
}

func newConsoleSink(minLevel slog.Level, format string) *ConsoleSink {
	opts := &slog.HandlerOptions{Level: minLevel}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return &ConsoleSink{logger: slog.New(handler), minLevel: minLevel}
}

func (c *ConsoleSink) Write(ev ExhaustEvent) {
	if ev.Level < c.minLevel {
		return
	}
	attrs := make([]any, 0, 12)
	attrs = append(attrs, "kind", ev.Kind)
	if ev.Instance != "" {
		attrs = append(attrs, "instance", ev.Instance)
	}
	if ev.Group != "" {
		attrs = append(attrs, "group", ev.Group)
	}
	if ev.Subject != "" {
		attrs = append(attrs, "subject", ev.Subject)
	}
	if ev.MessageId != "" {
		attrs = append(attrs, "messageId", ev.MessageId)
	}
	if ev.Remote != "" {
		attrs = append(attrs, "remote", ev.Remote)
	}
	if ev.Content != "" {
		attrs = append(attrs, "content", ev.Content)
	}
	if ev.Err != "" {
		attrs = append(attrs, "err", ev.Err)
	}
	c.logger.Log(context.Background(), ev.Level, ev.Msg, attrs...)
}
