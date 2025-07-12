package main

import (
	"bufio"
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
	if CtConnect == payload.command && payload.instanceId != "" {
		connCl.instanceName = payload.instanceId
	} else {
		connCl.instanceName = uuid.NewString()
	}

	backPayload := Payload{command: CtConnectSuccess, instanceId: connCl.instanceName}
	connCl.Write(backPayload)
}

func (connCl *ConnectedClient) readPayload() Payload {
	byteLs, error := waitAndRead(connCl.connection)
	if error != nil {
		println("Hata: ", error)
	}

	payload, _ := parsePayloadMsgPack(byteLs)
	return payload
}

func (connCl *ConnectedClient) MainLoop() {
	defer connCl.Die()
	for {
		pl := connCl.readPayload()
		switch pl.command {
		case CtClose:
			connCl.Die()
		case CtListen:
			connCl.listeningSubjects = append(connCl.listeningSubjects, pl.subject)
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
	// 			panic(err)
	// 		}
	// 	}

	// }
	bytels, err := reader.ReadBytes(4)
	if err != nil {
		panic(err)
	}
	return bytels, nil

}

func (connCl ConnectedClient) WriteStr(str string) {
	if connCl.connection != nil && !connCl.died {
		connCl.connection.Write([]byte(str))
	}
}

func (connCl ConnectedClient) Write(pl Payload) {
	if connCl.connection != nil && !connCl.died {
		json, err := pl.toMsgPak()
		if err != nil {
			println("HATA ", err)
		}
		connCl.connection.Write(json)
	}
}

func (connCl *ConnectedClient) Die() {
	if connCl.connection != nil && !connCl.died {
		defer connCl.connection.Close()
		defer connCl.operator.removeConnectedClient(connCl.instanceName)
		connCl.died = true
	}
}
