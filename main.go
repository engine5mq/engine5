package main

import (
	"fmt"
	"net"
)

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	fmt.Println("Engine5 starting")
	fmt.Println("Listening on 8080")
	mainOperato := QueueOperator{
		instances: []*ConnectedClient{},
		sent:      []*Message{},
		waiting:   []*Message{},
	}
	go mainOperato.LoopMessages()
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Connection Error: ", err)
			continue
		}

		go handleConnection(conn, &mainOperato)
	}
}

func handleConnection(conn net.Conn, op *QueueOperator) {
	var connCl = ConnectedClient{}
	// defer connCl.Die()
	connCl.SetConnection(conn)
	op.addConnectedClient(&connCl)
	go connCl.MainLoop()

}
