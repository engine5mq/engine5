package main

import (
	"fmt"
	"net"
)

// defer en son çalıştırılır, LIFO
// go ise arkaplanda çalıştırılır

func main() {
	// Listen for incoming connections
	listener, err := net.Listen("tcp", "0.0.0.0:8080")
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
	// defer
	defer conn.Close()

	conn.Write([]byte("HTTP/1.1 200 OK\n\n" +
		"<html><body><img src='https://upload.wikimedia.org/wikipedia/tr/7/72/Kylesapkasiz.jpg'/></body></html>"))
	// if err != nil {
	// 	fmt.Printf("err.Error(): %v\n", err.Error())

	// }
	// conn.Close()

	// Read and process data from the client
	// ...

	// Write data back to the client
	// ...
}
