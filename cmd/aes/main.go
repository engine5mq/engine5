package main

import (
	"engine5/internal/aes"
	"engine5/internal/aes/entity"
	"engine5/internal/common"
	"fmt"
)

// import "engine5/internal/server"

func main() {
	// server.Run()
	// println("AES main function executed")
	fmt.Println("Engine5 AES - (c) 2026 - Tetakent (H.C.G)")
	fmt.Println("Starting Database connection")

	dbManager, err := aes.ConnectToDatabase()
	if err != nil {
		fmt.Println("Failed to connect to database:", err)
		return
	}
	defer dbManager.Close()

	fmt.Println("Connecting e5 server as exhaustive client.")
	// TODO: E5'e tap client olarak bağlanmak için gerekli kodu buraya ekle.
	tapConfig := common.GetTapConfig()
	tapManager := aes.NewTapManager(func(ev aes.TapEvent) {
		// fmt.Printf("[%s] %s: %s | %s | %s | %s | %s | %s | %s | %s\n", ev.Time.Format("2006-01-02 15:04:05"), ev.Kind, ev.Group, ev.Instance, ev.Subject, ev.MessageId, ev.Remote, ev.Content, ev.Err, ev.Msg)
		// TODO: Gelen olayları veritabanına kaydedeceğiz sonra
		insertion, err := dbManager.Insert(
			entity.AesEventTapTableName(),
			(entity.AesEventTap{
				Time:      ev.Time,
				Level:     ev.Level,
				Kind:      ev.Kind,
				Instance:  ev.Instance,
				Group:     ev.Group,
				Subject:   ev.Subject,
				MessageId: ev.MessageId,
				Remote:    ev.Remote,
				Content:   ev.Content,
				Err:       ev.Err,
				Msg:       ev.Msg,
			}).KeyValuePairs(),
		)

		if err != nil {
			fmt.Printf("Failed to insert event into database: %v\n", err)
		} else {
			lastInsertID, lastInsertErr := insertion.LastInsertId()
			if lastInsertErr != nil {
				fmt.Printf("Inserted event into database (ID unavailable): %v\n", lastInsertErr)
			} else {
				fmt.Printf("Inserted event into database with ID: %v\n", lastInsertID)
			}
		}
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
