package main

import (
	"engine5/internal/common"
	"fmt"
)

// import "engine5/internal/server"

func main() {
	// server.Run()
	// println("AES main function executed")
	fmt.Println("Engine5 AES - (c) 2026 - Tetakent (H.C.G)")
	fmt.Println("Connecting e5 server as exhaustive client.")
	// TODO: E5'e tap client olarak bağlanmak için gerekli kodu buraya ekle.
	tapConfig := common.GetTapConfig()

	fmt.Println("Connecting e5 server as a client.")
	// TODO: E5'e client olarak bağlanmak için gerekli kodu buraya ekle.
	fmt.Println("Loading rules")

}
