package main

import (
	"fmt"
	"net"
)

func main() {
	// a, b := "anan", "anan"
	// if a == b {
	// 	println("eşitler")
	// } else {
	// 	println("eşit değiller")

	// }
	// return
	// TCP sunucusunu 8080 portunda başlat
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	fmt.Println("Engine5 starting")
	fmt.Println("Listening on 8080")

	mainOperato := QueueOperator{
		instances: []*ConnectedClient{},
		messages:  []*Message{},
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
	defer connCl.Die()
	connCl.SetConnection(conn)
	op.addConnectedClient(&connCl)
	go connCl.MainLoop()
	// defer connCl.Die()

	// Bağlantı kapatılmalı – defer kullanıyoruz
	// defer conn.Close()
	// reader := bufio.NewReader(conn)
	// message, err := reader.ReadString('\n')

	// if err != nil {
	// 	fmt.Println("zort")
	// 	return
	// }

	// // fmt.Printf("Received: %s", string(message))

	// // request := string(buffer[:n]) + string(buffer[:n])
	// fmt.Println("Gelen istek:\n", message)

	// // Sadece GET isteği ise cevap verelim (isteğe bağlı filtre)
	// if strings.HasPrefix(message, "GET") {
	// 	response := "HTTP/1.1 200 OK\r\n" +
	// 		"Content-Type: text/html\r\n" +
	// 		"\r\n" +
	// 		"<html><body><img src='https://a1cf74336522e87f135f-2f21ace9a6cf0052456644b80fa06d4f.ssl.cf2.rackcdn.com/images/characters/large/800/Kyle-Broflovski.South-Park.webp'/></body></html>"
	// 	conn.Write([]byte(response))
	// }
}
