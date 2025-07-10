package main

import (
	"bufio"
	"fmt"
	"net"
)

// type ConnectedClient interface {
// 	End()
// 	Send()
// }

type ConnectedClient struct {
	instanceName      string
	connection        net.Conn
	died              bool
	listeningSubjects []string
}

func (connCl *ConnectedClient) SetConnection(conn net.Conn) {
	connCl.connection = conn
	connCl.died = false
	instanceName := waitAndRead(connCl.connection)
	if instanceName == "" {
		instanceName = uu
	}
}

func (connCl *ConnectedClient) Listen(subjectName string) {
	connCl.listeningSubjects = append(connCl.listeningSubjects, subjectName)
}

func (connCl ConnectedClient) Read() string {
	if connCl.connection != nil && !connCl.died {
		return waitAndRead(connCl.connection)

	}
	return "connCl.connection != nil && !connCl.died SAĞLANMIYOR"
}

func waitAndRead(connCl net.Conn) string {
	reader := bufio.NewReader(connCl)
	message, err := reader.ReadString('\n')

	if err != nil {
		fmt.Println("zort")
		return "HATA"
	}

	return string(message)

}

func (connCl ConnectedClient) Write(str string) {
	if connCl.connection != nil && !connCl.died {
		connCl.connection.Write([]byte(str))
	}
}

func (connCl *ConnectedClient) Die() {
	if connCl.connection != nil && !connCl.died {
		defer connCl.connection.Close()
		connCl.died = true
	}
}
