package main

import (
	"fmt"
	"net"
)

// defer en son çalıştırılır, LIFO
// go ise arkaplanda çalıştırılır

func main() {
	// Listen for incoming connections
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Server is listening on port 8080")

	for {
		// Accept incoming connections
		conn, err := listener.Accept()
		fmt.Println("Yeni bağlantı")

		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		// Handle client connection in a goroutine
		go handleClient(conn)
	}
}

func handleClient(conn net.Conn) {
	defer conn.Close()

	go conn.Write([]byte("HTTP/1.1 200 OK\n" +
		"Date: Sun, 29 Apr 2024 12:00:00 GMT\n" +
		"Server: Saygex\n" +
		"Content-Type: text/html; charset=UTF-8\n" +
		"Content-Length: 48\n" +
		"Set-Cookie: sessionToken=abc123; Expires=Wed, 09 Jun 2024 10:18:14 GMT; HttpOnly\n" +
		"Connection: close\n" +
		"anan"))
	// conn.Close()

	// Read and process data from the client
	// ...

	// Write data back to the client
	// ...
}
