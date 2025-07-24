package main

import (
	"bufio"
	"fmt"
	"net"

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

	// aynı olan instance idleri
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
		connCl.Listen(pl.Subject)
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
			connCl.connection.Write(append(v, 4))
			ct++
			// println("Write count: (" + connCl.instanceName + ") " + strconv.Itoa(ct))
		}
	}
	// pl := connCl.readPayload()

}

func (connCl *ConnectedClient) ReaderLoop() {
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
	connCl.listeningSubjects[subjectName] = true
}

func (connCl *ConnectedClient) IsListening(subjectName string) bool {

	var hasSubject, hasKey = connCl.listeningSubjects[subjectName]
	return hasKey && hasSubject
}

func (connCl *ConnectedClient) Die() {
	if connCl.connection != nil && !connCl.died {

		defer connCl.connection.Close()
		defer connCl.operator.removeConnectedClient(connCl.instanceName)
		connCl.died = true
		fmt.Println("Client " + connCl.instanceName + " has been closed")
	}
}
