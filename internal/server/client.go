package server

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/google/uuid"
)

type ConnectedClient struct {
	instanceName      string
	connection        net.Conn
	died              bool
	listeningSubjects map[string]bool
	operator          *MessageOperator
	writing           bool
	writeQueue        chan []byte
	instanceGroup     string
	authClient        *AuthenticatedClient
}

func (connCl *ConnectedClient) SetOperator(operator *MessageOperator) {
	connCl.operator = operator

}

func (connCl *ConnectedClient) SetConnection(conn net.Conn) {
	connCl.connection = conn
	connCl.writing = false
	connCl.listeningSubjects = map[string]bool{}

}

func (connCl *ConnectedClient) BeSureConnection(payload Payload) {
	if connCl == nil {
		return
	}

	if connCl.authClient == nil {
		connCl.authClient = &AuthenticatedClient{ConnectedClient: connCl}
	}

	// Check authentication if required
	if connCl.operator != nil && connCl.operator.authConfig != nil && connCl.operator.authConfig.RequireAuth {
		if payload.AuthKey == "" {
			connCl.Write(Payload{
				Command: CtConnectError,
				Content: "Authentication required: auth key missing",
			})
			if connCl.operator != nil && connCl.operator.exhaust != nil {
				connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindAuthRejected, Msg: "Connection rejected: auth key missing"})
			}
			connCl.Die()
			return
		}

		// Determine client ID
		clientID := payload.InstanceId
		if clientID == "" {
			clientID = "default"
		}

		// Validate auth key
		permissions, err := connCl.operator.authConfig.ValidateAuthKey(payload.AuthKey, clientID)
		if err != nil {
			connCl.Write(Payload{
				Command: CtConnectError,
				Content: "Authentication failed: " + err.Error(),
			})
			if connCl.operator != nil && connCl.operator.exhaust != nil {
				connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindAuthRejected, Instance: clientID, Msg: "Connection rejected", Err: err.Error()})
			}
			connCl.Die()
			return
		}

		// Set up authenticated client
		now := time.Now()
		connCl.authClient.IsAuth = true
		connCl.authClient.Token = &AuthToken{
			ClientID:    clientID,
			Permissions: permissions,
			IssuedAt:    now,
			ExpiresAt:   now.Add(connCl.operator.authConfig.TokenExpiry),
		}
		connCl.authClient.RateLimiter = NewRateLimiter(permissions.RateLimit)
		if connCl.operator != nil && connCl.operator.exhaust != nil {
			connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindAuthOk, Instance: clientID, Msg: "Client authenticated successfully"})
		}
	}

	// aynı olan instance idleri
	if CtConnect == payload.Command && payload.InstanceId != "" {
		connCl.instanceName = payload.InstanceId
	} else {
		connCl.instanceName = uuid.NewString()
	}

	if payload.InstanceGroup == "" {
		connCl.instanceGroup = payload.InstanceId
	} else {
		connCl.instanceGroup = payload.InstanceGroup
	}
	backPayload := Payload{Command: CtConnectSuccess, InstanceId: connCl.instanceName, InstanceGroup: payload.InstanceGroup}
	connCl.died = false
	connCl.Write(backPayload)
	if connCl.operator != nil && connCl.operator.exhaust != nil {
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindClientConnected, Instance: connCl.instanceName, Group: connCl.instanceGroup, Msg: "Client connected"})
	}
}

func (connCl *ConnectedClient) ReviewPayload(pl Payload) {
	if err := connCl.AuthorizePayload(pl); err != nil {
		connCl.Write(Payload{
			Command:   CtUnauthorized,
			Content:   err.Error(),
			Subject:   pl.Subject,
			MessageId: pl.MessageId,
		})
		return
	}

	switch pl.Command {
	case CtConnect:
		connCl.BeSureConnection(pl)
	case CtClose:
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindClientClosing, Instance: connCl.instanceName, Msg: "Client is closing"})
		connCl.Die()
	case CtListen:
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindClientListen, Instance: connCl.instanceName, Subject: pl.Subject, Msg: "Client is listening subject"})
		connCl.Listen(pl.Subject)
		connCl.Write(Payload{Command: CtRecieved, Subject: pl.Subject})
		go connCl.operator.rescanRequestsForClient(connCl)
	case CtEvent:
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelDebug, Kind: KindEventReceived, Instance: connCl.instanceName, Subject: pl.Subject, MessageId: pl.MessageId, Content: pl.Content, Msg: "Client sent an event"})
		msg := MessageFromPayload(pl)
		connCl.operator.addEvent(msg)
		connCl.Write(Payload{Command: CtRecieved, MessageId: msg.id, Subject: msg.targetSubjectName})
	case CtRetryEvent:
		if pl.Subject == "" || pl.MessageId == "" || pl.OriginalMessageId == "" {
			connCl.Write(Payload{Command: CtRetryError, MessageId: pl.MessageId, Subject: pl.Subject, Content: "Retry requires subject, messageId and originalMessageId"})
			return
		}
		msg := MessageFromPayload(pl)
		msg.retrySender = connCl
		connCl.operator.addEvent(msg)
	case CtConsumingSuccess:
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelDebug, Kind: KindConsumingSuccess, Instance: connCl.instanceName, Group: connCl.instanceGroup, Subject: pl.Subject, MessageId: pl.MessageId, Content: pl.Content, Msg: "Client consumed a message successfully"})
	case CtConsumingError:
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelError, Kind: KindConsumingError, Instance: connCl.instanceName, Group: connCl.instanceGroup, Subject: pl.Subject, MessageId: pl.MessageId, Content: pl.Content, Msg: "Client failed to consume a message"})
	case CtRequest:
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelDebug, Kind: KindRequestReceived, Instance: connCl.instanceName, Subject: pl.Subject, MessageId: pl.MessageId, Content: pl.Content, Msg: "Client sent a request"})
		msg := MessageFromPayload(pl)
		connCl.operator.addRequest(msg, connCl)
		connCl.Write(Payload{Command: CtRecieved, MessageId: msg.id, Subject: msg.targetSubjectName})

	case CtResponse:
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelDebug, Kind: KindResponseReceived, Instance: connCl.instanceName, MessageId: pl.ResponseOfMessageId, Content: pl.Content, IsError: pl.IsError, Msg: "Client responded a request"})
		msg := MessageFromPayload(pl)
		connCl.operator.respondRequest(msg)
		connCl.Write(Payload{Command: CtRecieved, MessageId: msg.id, Subject: msg.targetSubjectName})
	}

}

func (connCl *ConnectedClient) Write(pl Payload) {
	if err := connCl.queuePayload(pl); err != nil && connCl != nil && connCl.operator != nil && connCl.operator.exhaust != nil {
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindInternalError, Instance: connCl.instanceName, Msg: "Failed to queue payload", Err: err.Error()})
	}
}

func (connCl *ConnectedClient) queuePayload(pl Payload) error {
	if connCl == nil || connCl.connection == nil || connCl.died || connCl.writeQueue == nil {
		return fmt.Errorf("client is not connected")
	}

	json, err := pl.toMsgPak()
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	select {
	case connCl.writeQueue <- json:
		return nil
	default:
		return fmt.Errorf("write queue full")
	}
}

func (connCl *ConnectedClient) WriterLoop() {
	ct := 0
	for {
		for v := range connCl.writeQueue {
			// Length-prefix zaten toMsgPak() içinde eklendi
			connCl.connection.Write(v)
			ct++
			// println("Write count: (" + connCl.instanceName + ") " + strconv.Itoa(ct))
		}
	}
	// pl := connCl.readPayload()

}

func (connCl *ConnectedClient) ReaderLoop() {
	defer connCl.Die()
	reader := bufio.NewReader(connCl.connection)

	for {
		// Length-prefixed protocol: Önce 4 byte uzunluk bilgisini oku
		lengthBytesRemaining := 4
		lengthBytes := make([]byte, 4)
		// _, err := reader.Read(lengthBytes)
		// if err != nil {
		// 	println("Error reading length prefix: ", err)
		// 	break
		// }
		for lengthBytesRemaining > 0 {
			n, err := reader.Read(lengthBytes[4-lengthBytesRemaining:])

			if err != nil {
				if (err.Error() == "EOF") || (err.Error() == "read tcp "+connCl.connection.LocalAddr().String()+"->"+connCl.connection.RemoteAddr().String()+": use of closed network connection") {
					connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindClientClosing, Instance: connCl.instanceName, Msg: "Connection closed by client"})
					connCl.Die()
					return
				}
				// tls: first record does not look like a TLS handshake, bu hata genellikle TLS olmayan bir bağlantının TLS bekleyen bir sunucuya bağlanmaya çalışması durumunda ortaya çıkar, bu durumda da bağlantının kapandığını varsayıp client'ı öldürüyoruz
				if err.Error() == "tls: first record does not look like a TLS handshake" {
					connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindProtocolError, Instance: connCl.instanceName, Msg: "Non-TLS connection to TLS server, closing connection"})
					connCl.Die()
					return
				}
				// TLS bağlantısında bazen "remote error: handshake failure" hatası alınabiliyor, bu durumda da bağlantının kapandığını varsayıp client'ı öldürüyoruz
				if err.Error() == "remote error: handshake failure" {
					connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindProtocolError, Instance: connCl.instanceName, Msg: "TLS handshake failure, closing connection"})
					connCl.Die()
					return
				}

				connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelError, Kind: KindProtocolError, Instance: connCl.instanceName, Msg: "Error reading length prefix", Err: err.Error()})
				break
			}
			lengthBytesRemaining -= n
		}

		// Uzunluk bilgisini uint32'ye çevir
		messageLength := binary.BigEndian.Uint32(lengthBytes)

		// Belirtilen uzunlukta msgpack verisini oku
		remainingLength := int(messageLength)
		msgpackData := make([]byte, remainingLength)
		// _, err = reader.Read(msgpackData)
		for remainingLength > 0 {
			n, err := reader.Read(msgpackData[messageLength-uint32(remainingLength):])
			if err != nil {
				if (err.Error() == "EOF") || (err.Error() == "read tcp "+connCl.connection.LocalAddr().String()+"->"+connCl.connection.RemoteAddr().String()+": use of closed network connection") {
					connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindClientClosing, Instance: connCl.instanceName, Msg: "Connection closed by client"})
					connCl.Die()
					return
				}
				// tls: first record does not look like a TLS handshake, bu hata genellikle TLS olmayan bir bağlantının TLS bekleyen bir sunucuya bağlanmaya çalışması durumunda ortaya çıkar, bu durumda da bağlantının kapandığını varsayıp client'ı öldürüyoruz
				if err.Error() == "tls: first record does not look like a TLS handshake" {
					connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindProtocolError, Instance: connCl.instanceName, Msg: "Non-TLS connection to TLS server, closing connection"})
					connCl.Die()
					return
				}
				// TLS bağlantısında bazen "remote error: handshake failure" hatası alınabiliyor, bu durumda da bağlantının kapandığını varsayıp client'ı öldürüyoruz
				if err.Error() == "remote error: handshake failure" {
					connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindProtocolError, Instance: connCl.instanceName, Msg: "TLS handshake failure, closing connection"})
					connCl.Die()
					return
				}
				connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelError, Kind: KindProtocolError, Instance: connCl.instanceName, Msg: "Error reading msgpack data", Err: err.Error()})
				break
			}
			remainingLength -= n
		}

		// if err != nil {
		// 	println("Error reading msgpack data: ", err)
		// 	break
		// }

		// Msgpack verisini parse et
		pl, err2 := parsePayloadMsgPack(msgpackData)
		if err2 != nil {
			connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelError, Kind: KindParseError, Instance: connCl.instanceName, Msg: "Error while parsing payload", Err: err2.Error()})
		} else {
			connCl.ReviewPayload(pl)
			if connCl.died {
				break
			}
		}
	}
}

func (connCl *ConnectedClient) Listen(subjectName string) {
	connCl.listeningSubjects[subjectName] = true
}

func (connCl *ConnectedClient) IsListening(subjectName string) bool {

	var hasSubject, hasKey = connCl.listeningSubjects[subjectName]
	return hasKey && hasSubject
}

/**
* Client bağlantısını sonlandırır
 */
func (connCl *ConnectedClient) Die() {
	if connCl == nil || connCl.connection == nil || connCl.died {
		return
	}

	defer connCl.connection.Close()
	if connCl.operator != nil {
		defer connCl.operator.removeConnectedClient(connCl.instanceName)
		if connCl.operator.exhaust != nil {
			defer connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindClientClosed, Instance: connCl.instanceName, Msg: "Client has been closed"})
		}
	}
	connCl.died = true
}
