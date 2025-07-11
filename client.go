package main

import (
	"bufio"
	"errors"
	"net"

	"github.com/google/uuid"
)

type ConnectedClient struct {
	instanceName      string
	connection        net.Conn
	died              bool
	listeningSubjects []string
	operator          *QueueOperator
}

func (connCl *ConnectedClient) SetOperator(operator *QueueOperator) {
	connCl.operator = operator

}

func (connCl *ConnectedClient) SetConnection(conn net.Conn) {
	connCl.connection = conn
	connCl.died = false
	payload := connCl.readPayload()
	if CtConnect == payload.Command && payload.InstanceId != "" {
		connCl.instanceName = payload.InstanceId
	} else {
		connCl.instanceName = uuid.NewString()
	}

	backPayload := Payload{Command: CtConnectSuccess, InstanceId: connCl.instanceName}
	js, _ := backPayload.toJson()
	connCl.WriteStr(js)
}

func (connCl *ConnectedClient) readPayload() Payload {
	jsonStr, error := waitAndReadString(connCl.connection)
	if error != nil {
		println("Hata: ", error)
	}

	payload, _ := parsePayload(jsonStr)
	return payload
}

func (connCl *ConnectedClient) MainLoop() {
	defer connCl.Die()
	for {
		pl := connCl.readPayload()
		switch pl.Command {
		case CtClose:
			connCl.Die()
		case CtListen:
			connCl.listeningSubjects = append(connCl.listeningSubjects, pl.Subject)
		case CtEvent:
			connCl.operator.addMessage(MessageFromPayload(pl))
			// case CtRequest:
			// 	// todo
			// case CtResponse:
			// 	// todo:
			// case CtRecieved:
			// 	//todo:
		}
	}

}

func (connCl *ConnectedClient) Listen(subjectName string) {
	connCl.listeningSubjects = append(connCl.listeningSubjects, subjectName)
}

func (connCl ConnectedClient) Read() (string, error) {
	if connCl.connection != nil && !connCl.died {
		return waitAndReadString(connCl.connection)

	}
	return "", errors.New("connCl.connection != nil && !connCl.died SAĞLANMIYOR")
}

func waitAndReadString(connCl net.Conn) (string, error) {
	reader := bufio.NewReader(connCl)
	message, err := reader.ReadString('\n')

	if err != nil {
		return "", err
	}

	return string(message), nil

}

func (connCl ConnectedClient) WriteStr(str string) {
	if connCl.connection != nil && !connCl.died {
		connCl.connection.Write([]byte(str))
	}
}

func (connCl ConnectedClient) Write(pl Payload) {
	if connCl.connection != nil && !connCl.died {
		json, err := pl.toJson()
		if err != nil {
			println("HATA ", err)
		}
		connCl.WriteStr(json)
	}
}

func (connCl *ConnectedClient) Die() {
	if connCl.connection != nil && !connCl.died {
		defer connCl.connection.Close()
		defer connCl.operator.removeConnectedClient(connCl.instanceName)
		connCl.died = true
	}
}
