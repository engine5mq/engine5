package main

import (
	"fmt"
	"net"
	"strings"
	"unicode"
)

func main() {
	// TCP sunucusunu 8080 portunda başlat
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	fmt.Println("Sunucu 8080 portunda dinleniyor...")

	// Sonsuz döngüde gelen bağlantıları dinle
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Bağlantı hatası:", err)
			continue
		}

		// Her bağlantıyı ayrı bir goroutine'de işle
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	var connCl = ConnectedClient{}
	connCl.SetConnection(conn)
	defer connCl.Die()

	for {
		gelen1 := connCl.Read()
		if strings.HasPrefix(strings.ToUpperSpecial(unicode.TurkishCase, gelen1), "GEBER") {
			connCl.Die()
			break
		} else {
			connCl.Write("Asıl sana " + gelen1)
		}
	}

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
