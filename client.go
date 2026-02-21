package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
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
			fmt.Println("Connection rejected: auth key missing")
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
			fmt.Printf("Connection rejected for client %s: %v\n", clientID, err)
			connCl.Die()
			return
		}

		// Set up authenticated client
		connCl.authClient.IsAuth = true
		connCl.authClient.Token = &AuthToken{
			ClientID:    clientID,
			Permissions: permissions,
			IssuedAt:    time.Now(),
		}
		connCl.authClient.RateLimiter = NewRateLimiter(permissions.RateLimit)
		fmt.Printf("Client %s authenticated successfully\n", clientID)
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
	fmt.Println("Connected client's instance name is: " + connCl.instanceName)
}

func (connCl *ConnectedClient) ReviewPayload(pl Payload) {
	switch pl.Command {
	case CtConnect:
		connCl.BeSureConnection(pl)
	case CtClose:
		fmt.Println("Client " + connCl.instanceName + " is closing")
		connCl.Die()
	case CtListen:
		fmt.Println("Client " + connCl.instanceName + " is listening '" + pl.Subject + "' subject")
		connCl.Listen(pl.Subject)
		connCl.Write(Payload{Command: CtRecieved, Subject: pl.Subject})
		go connCl.operator.rescanRequestsForClient(connCl)
	case CtEvent:
		fmt.Println("Client " + connCl.instanceName + " sent a event. " + " content: " + pl.Content + ", id: " + pl.MessageId)
		msg := MessageFromPayload(pl)
		connCl.operator.addEvent(msg)
		connCl.Write(Payload{Command: CtRecieved, MessageId: msg.id, Subject: msg.targetSubjectName})

	case CtRequest:
		fmt.Println("Client " + connCl.instanceName + " sent a request. " + " content: " + pl.Content + ", id: " + pl.MessageId + ", subject " + pl.Subject)
		msg := MessageFromPayload(pl)
		connCl.operator.addRequest(msg, connCl)
		connCl.Write(Payload{Command: CtRecieved, MessageId: msg.id, Subject: msg.targetSubjectName})

	case CtResponse:
		fmt.Println("Client " + connCl.instanceName + " responsed a request. " + " content: " + pl.Content + ", responseOf: " + pl.ResponseOfMessageId)
		msg := MessageFromPayload(pl)
		connCl.operator.respondRequest(msg)
		connCl.Write(Payload{Command: CtRecieved, MessageId: msg.id, Subject: msg.targetSubjectName})
	}

}

func (connCl *ConnectedClient) Write(pl Payload) {
	if connCl.connection != nil && !connCl.died {
		json, err := pl.toMsgPak()
		if err != nil {
			println("HATA ", err)
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
					println("Connection closed by client: ", connCl.instanceName)
					connCl.Die()
					return
				}
				println("Error reading length prefix: ", err.Error())
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
				println("Error reading msgpack data: ", err)
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
			println("Error while parsing payload: ", err2)
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
		fmt.Println("Client " + connCl.instanceName + " has been closed")
	}
}
