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
	connection net.Conn
	died       bool
}

func (connCl *ConnectedClient) SetConnection(conn net.Conn) {
	connCl.connection = conn
	connCl.died = false
}

func (connCl ConnectedClient) Read() string {
	if connCl.connection != nil && !connCl.died {
		reader := bufio.NewReader(connCl.connection)
		message, err := reader.ReadString('\n')

		if err != nil {
			fmt.Println("zort")
			return "HATA"
		}

		return string(message)
	}
	return "connCl.connection != nil && !connCl.died SAĞLANMIYOR"
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
