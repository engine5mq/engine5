package main

import (
	"fmt"
	"net"
	"os"
)

func main() {

	port := os.Getenv("E5_PORT")
	if port == "" {
		port = "3535"
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}
	fmt.Println("Engine5 is being started")
	fmt.Println("Listening on " + port)
	mainOperato := MessageOperator{
		instances:                     []*ConnectedClient{},
		waiting:                       make(chan Message),
		ongoingRequests:               make(map[string]*OngoingRequest),
		requestGate:                   make(chan *RequestGateObject),
		instanceGroupSelectionIndexes: make(map[string]*InstanceGroupIndexSelection),
		clientConnectionQueue:         NewTaskQueue(1),
	}
	go mainOperato.LoopMessages()
	go mainOperato.LoopRequests()
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Connection Error: ", err)
			continue
		}
		fmt.Println("Incoming connection")
		go handleConnection(conn, &mainOperato)
	}
}

func handleConnection(conn net.Conn, op *MessageOperator) {
	var connCl = ConnectedClient{died: true, writeQueue: make(chan []byte, 100)}
	// defer connCl.Die()
	connCl.SetConnection(conn)
	op.addConnectedClient(&connCl)
	go connCl.ReaderLoop()
	go connCl.WriterLoop()

}
