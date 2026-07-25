package main

import (
	"engine5/internal/aes"
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
	tapManager := aes.NewTapManager(func(ev aes.TapEvent) {
		fmt.Printf("[%s] %s: %s | %s | %s | %s | %s | %s | %s | %s\n", ev.Time.Format("2006-01-02 15:04:05"), ev.Kind, ev.Group, ev.Instance, ev.Subject, ev.MessageId, ev.Remote, ev.Content, ev.Err, ev.Msg)
		// TODO: Gelen olayları veritabanına kaydedeceğiz sonra
	})
	defer tapManager.Close()

	tapManager.Connect(aes.ConnectionInfo{
		Host:      tapConfig.Host,
		Port:      tapConfig.Port,
		Key:       tapConfig.Key,
		UseTLS:    tapConfig.UseTLS,
		CAFile:    tapConfig.CAFile,
		Reconnect: tapConfig.Reconnect,
	})
}
