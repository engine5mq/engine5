package server

import (
	"bufio"
	"encoding/binary"
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
	// Check authentication if required
	if connCl.operator.authConfig.RequireAuth {
		if payload.AuthKey == "" {
			connCl.Write(Payload{
				Command: CtConnectError,
				Content: "Authentication required: auth key missing",
			})
			connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindAuthRejected, Msg: "Connection rejected: auth key missing"})
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
			connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindAuthRejected, Instance: clientID, Msg: "Connection rejected", Err: err.Error()})
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
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindAuthOk, Instance: clientID, Msg: "Client authenticated successfully"})
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
	connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindClientConnected, Instance: connCl.instanceName, Group: connCl.instanceGroup, Msg: "Client connected"})
}

func (connCl *ConnectedClient) ReviewPayload(pl Payload) {
	if err := connCl.AuthorizePayload(pl); err != nil {
		connCl.Write(Payload{
			Command: CtUnauthorized,
			Content: err.Error(),
			Subject: pl.Subject,
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

	case CtRequest:
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelDebug, Kind: KindRequestReceived, Instance: connCl.instanceName, Subject: pl.Subject, MessageId: pl.MessageId, Content: pl.Content, Msg: "Client sent a request"})
		msg := MessageFromPayload(pl)
		connCl.operator.addRequest(msg, connCl)
		connCl.Write(Payload{Command: CtRecieved, MessageId: msg.id, Subject: msg.targetSubjectName})

	case CtResponse:
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelDebug, Kind: KindResponseReceived, Instance: connCl.instanceName, MessageId: pl.ResponseOfMessageId, Content: pl.Content, Msg: "Client responded a request"})
		msg := MessageFromPayload(pl)
		connCl.operator.respondRequest(msg)
		connCl.Write(Payload{Command: CtRecieved, MessageId: msg.id, Subject: msg.targetSubjectName})
	}

}

func (connCl *ConnectedClient) Write(pl Payload) {
	if connCl.connection != nil && !connCl.died {
		json, err := pl.toMsgPak()
		if err != nil {
			connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelError, Kind: KindInternalError, Instance: connCl.instanceName, Msg: "Failed to encode payload", Err: err.Error()})
		}
		connCl.writeQueue <- json

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
	if connCl.connection != nil && !connCl.died {
		defer connCl.operator.removeConnectedClient(connCl.instanceName)
		defer connCl.connection.Close()
		connCl.died = true
		connCl.operator.exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindClientClosed, Instance: connCl.instanceName, Msg: "Client has been closed"})
	}
}
