package main

import (
	"context"
	"engine5/internal/aes"
	"engine5/internal/common"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("AES stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("Engine5 AES - (c) 2026 - Tetakent (H.C.G)")
	config := common.GetAesConnectionConfig()
	rules, err := aes.LoadRules(config.RulesFile)
	if err != nil {
		return err
	}
	dbManager, err := aes.ConnectToDatabase()
	if err != nil {
		return err
	}
	defer dbManager.Close()

	clientConfig := common.GetClientConfig()
	tapConfig := common.GetTapConfig()
	publisher := aes.NewBrokerPublisher(aes.ConnectionInfo{
		Host: clientConfig.Host, Port: strconv.Itoa(clientConfig.Port),
		Key: config.AuthKey, UseTLS: clientConfig.TLSEnabled, CAFile: tapConfig.CAFile,
	})
	defer publisher.Close()
	processor := aes.NewProcessor(aes.NewStore(dbManager), rules, publisher)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	tapManager := aes.NewTapManager(func(ev aes.TapEvent) {
		if err := processor.Handle(ctx, ev, time.Now()); err != nil && ctx.Err() == nil {
			slog.Error("AES could not persist tap event", "kind", ev.Kind, "messageId", ev.MessageId, "error", err)
		}
	})
	defer tapManager.Close()

	schedulerDone := make(chan struct{})
	go func() {
		defer close(schedulerDone)
		processor.Run(ctx)
	}()
	cancelTap := context.AfterFunc(ctx, tapManager.Close)
	defer cancelTap()
	tapManager.Connect(aes.ConnectionInfo{
		Host: tapConfig.Host, Port: tapConfig.Port, Key: tapConfig.Key,
		UseTLS: tapConfig.UseTLS, CAFile: tapConfig.CAFile, Reconnect: tapConfig.Reconnect,
	})
	stop()
	<-schedulerDone
	return nil
}
