package server

import (
	"log/slog"
	"time"
)

/*
ExhaustEvent, Engine5 içindeki tüm önemli olayların yapısal temsilidir.
Serbest metin yerine bu yapı kullanılır; böylece hem konsola okunabilir
biçimde basılabilir hem de ileride "egzoz çıkışı" (tap) ile dışarıdaki
bir uygulamaya makine tarafından okunabilir şekilde yayınlanabilir.
*/
type ExhaustEvent struct {
	Time              time.Time  `json:"time"`
	Level             slog.Level `json:"level"`
	Kind              string     `json:"kind"`
	Instance          string     `json:"instance,omitempty"`
	Group             string     `json:"group,omitempty"`
	Subject           string     `json:"subject,omitempty"`
	MessageId         string     `json:"messageId,omitempty"`
	OriginalMessageId string     `json:"originalMessageId,omitempty"`
	Remote            string     `json:"remote,omitempty"`
	// Content tap'a orijinal haliyle gider; yalnızca konsol çıktısında maskelenir.
	Content string `json:"content,omitempty"`
	Err     string `json:"err,omitempty"`
	// Msg, olaya eşlik eden okunabilir kısa açıklamadır.
	Msg string `json:"msg,omitempty"`
	// IsError, RESPONSE olaylarında yanıtın hata olarak işaretlendiğini belirtir.
	IsError bool `json:"isError,omitempty"`
}

// Exhaust olay türleri (Kind). Dışarıdaki dinleyiciler bu sabitlere göre
// filtreleme yapabilir.
const (
	KindServerStart = "SERVER_START"
	KindServerError = "SERVER_ERROR"

	KindClientConnecting = "CLIENT_CONNECTING"
	KindClientConnected  = "CLIENT_CONNECTED"
	KindClientClosing    = "CLIENT_CLOSING"
	KindClientClosed     = "CLIENT_CLOSED"
	KindClientRenamed    = "CLIENT_RENAMED"

	KindAuthOk       = "AUTH_OK"
	KindAuthRejected = "AUTH_REJECTED"

	KindClientListen = "CLIENT_LISTEN"

	KindEventReceived   = "EVENT_RECEIVED"
	KindEventDelivered  = "EVENT_DELIVERED"
	KindEventNoListener = "EVENT_NO_LISTENER"

	KindRequestReceived = "REQUEST_RECEIVED"
	KindRequestRouted   = "REQUEST_ROUTED"
	KindRequestNoTarget = "REQUEST_NO_TARGET"

	KindResponseReceived  = "RESPONSE_RECEIVED"
	KindResponseDelivered = "RESPONSE_DELIVERED"

	KindProtocolError    = "PROTOCOL_ERROR"
	KindParseError       = "PARSE_ERROR"
	KindInternalError    = "INTERNAL_ERROR"
	KindConsumingSuccess = "CONSUMING_SUCCESS"
	KindConsumingError   = "CONSUMING_ERROR"
)
