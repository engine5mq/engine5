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

}

func (connCl *ConnectedClient) ReviewPayload(pl Payload) {
	switch pl.Command {
	case CtConnect:
		connCl.BeSureConnection(pl)
	case CtClose:
		connCl.Die()
	case CtListen:
		connCl.listeningSubjects = append(connCl.listeningSubjects, pl.Subject)
	case CtEvent:
		connCl.operator.addMessage(MessageFromPayload(pl))

	}
}

func (connCl *ConnectedClient) MainLoop() {
	defer connCl.Die()
	reader := bufio.NewReader(connCl.connection)

	bytels := []byte{}
	for {

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
