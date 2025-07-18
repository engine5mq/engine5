package main

import (
	"bufio"
	"fmt"
	"net"
	"slices"

	"github.com/google/uuid"
)

type ConnectedClient struct {
	instanceName      string
	connection        net.Conn
	died              bool
	listeningSubjects []string
	operator          *MessageOperator
	writing           bool
}

func (connCl *ConnectedClient) SetOperator(operator *MessageOperator) {
	connCl.operator = operator

}

func (connCl *ConnectedClient) SetConnection(conn net.Conn) {
	connCl.connection = conn
	connCl.writing = false
	// payload := connCl.readPayload()

}

// func (connCl *ConnectedClient) readPayload() Payload {
// 	byteLs, error := waitAndRead(connCl.connection)
// 	if error != nil {
// 		println("Hata: ", error)
// 	}

// 	payload, _ := parsePayloadMsgPack(byteLs)
// 	return payload
// }

func (connCl *ConnectedClient) BeSureConnection(payload Payload) {
	if CtConnect == payload.Command && payload.InstanceId != "" {
		connCl.instanceName = payload.InstanceId
	} else {
		connCl.instanceName = uuid.NewString()
	}

	backPayload := Payload{Command: CtConnectSuccess, InstanceId: connCl.instanceName}
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
		connCl.listeningSubjects = append(connCl.listeningSubjects, pl.Subject)
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
		// case CtRequest:
		// 	fmt.Println("Client " + connCl.instanceName + " have a request. " + " content: " + pl.Content + ", id: " + pl.MessageId)
		// 	msg := MessageFromPayload(pl)
		// 	connCl.operator.addMessage(msg)
		// 	connCl.Write(Payload{Command: CtRecieved, MessageId: msg.id, Subject: msg.targetSubjectName})
	}
}

func (connCl *ConnectedClient) MainLoop() {
	defer connCl.Die()
	reader := bufio.NewReader(connCl.connection)

	bytels := []byte{}
	for {
		// Gelen byteları sürekli okur. Taa ki 0x04'e kadar
		byteReaded, err := reader.ReadByte()
		if err == nil {
			if byteReaded == 4 {
				pl, err2 := parsePayloadMsgPack(bytels)
				if err2 != nil {
					println("Error while reading and waiting payload: ", err2)
				} else {
					connCl.ReviewPayload(pl)
					if connCl.died {
						break
					}
				}
				bytels = []byte{}
			} else {
				bytels = append(bytels, byteReaded)

			}
		} else {
			println("Error: ", err)
			break
		}

	}
	// pl := connCl.readPayload()

}

func (connCl *ConnectedClient) Listen(subjectName string) {
	connCl.listeningSubjects = append(connCl.listeningSubjects, subjectName)
}

func (connCl *ConnectedClient) IsListening(subjectName string) bool {
	hasSubject := slices.Contains(connCl.listeningSubjects, subjectName)
	return hasSubject
}

// func (connCl ConnectedClient) Read() (string, error) {
// 	if connCl.connection != nil && !connCl.died {
// 		return waitAndReadString(connCl.connection)

// 	}
// 	return "", errors.New("connCl.connection != nil && !connCl.died SAĞLANMIYOR")
// }

func waitAndRead(connCl net.Conn) ([]byte, error) {
	reader := bufio.NewReader(connCl)
	// bytels := []byte{}
	// for {
	// 	byteReaded, err := reader.ReadByte()
	// 	if err == nil {
	// 		bytels = append(bytels, byteReaded)
	// 	} else {
	// 		if err == io.EOF {
	// 			break
	// 		} else {
	// 			panic(err),,,,,,,
	// 			panic(err),,,,,,,
	// 			panic(err),,,,,,,
	// 			panic(err),,,,,,,
	// 		},,,,
	// 	}

	// }
	bytels, err := reader.ReadBytes(4)
	if err != nil {
		// panic(err)
		println("error: ", err)
	}
	return bytels, nil

}

func (connCl *ConnectedClient) WriteStr(str string) {
	if connCl.connection != nil && !connCl.died {
		hold(connCl)
		connCl.connection.Write([]byte(str))
		release(connCl)
	}
}

func release(connCl *ConnectedClient) {
	connCl.writing = false
}

func hold(connCl *ConnectedClient) {
	for {
		if !connCl.writing {
			break
		}
	}
	connCl.writing = true
}

func (connCl ConnectedClient) Write(pl Payload) {
	if connCl.connection != nil && !connCl.died {
		hold(&connCl)
		json, err := pl.toMsgPak()
		if err != nil {
			println("HATA ", err)
		}
		connCl.connection.Write(append(json, 4))
		release(&connCl)
	}
}

func (connCl *ConnectedClient) Die() {
	if connCl.connection != nil && !connCl.died {
		defer connCl.connection.Close()
		defer connCl.operator.removeConnectedClient(connCl.instanceName)
		connCl.died = true
		fmt.Println("Client " + connCl.instanceName + " has been closed")

	}
}
