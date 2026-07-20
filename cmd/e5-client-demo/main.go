package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	engine5client "engine5/clients/go"
)

func main() {
	host := flag.String("host", envOr("E5_HOST", "localhost"), "Engine5 host")
	port := flag.Int("port", envIntOr("E5_PORT", 3535), "Engine5 port")
	instanceID := flag.String("instance-id", envOr("E5_INSTANCE_ID", "demo-client"), "client instance id")
	instanceGroup := flag.String("instance-group", envOr("E5_INSTANCE_GROUP", "demo-group"), "client instance group")
	subject := flag.String("subject", envOr("E5_SUBJECT", "demo.subject"), "subject to listen/send")
	tlsEnabled := flag.Bool("tls", envBoolOr("E5_TLS", false), "enable TLS")
	insecure := flag.Bool("insecure", false, "skip TLS certificate verification")
	requestDemo := flag.Bool("request", false, "send a demo request after connect")
	flag.Parse()

	var tlsConfig *tls.Config
	if *tlsEnabled {
		tlsConfig = &tls.Config{InsecureSkipVerify: *insecure} // #nosec G402 -- dev demo flag
	}

	client := engine5client.NewClient(engine5client.Options{
		Host:          *host,
		Port:          *port,
		InstanceID:    *instanceID,
		InstanceGroup: *instanceGroup,
		TLSConfig:     tlsConfig,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := client.Connect(ctx); err != nil {
		fatalf("connect: %v", err)
	}
	defer client.Close()

	if err := client.Listen(ctx, *subject, func(data any) any {
		fmt.Printf("event received on %s: %#v\n", *subject, data)
		return nil
	}); err != nil {
		fatalf("listen: %v", err)
	}

	if err := client.SendEvent(ctx, *subject, map[string]any{
		"kind":      "demo-event",
		"message":   "hello from Go client",
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "send event failed: %v\n", err)
	}

	if *requestDemo {
		response, err := client.Request(ctx, *subject, map[string]any{
			"kind": "demo-request",
			"ping": true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
		} else {
			fmt.Printf("request response: %#v\n", response)
		}
	}

	fmt.Printf("connected as %s in group %s; waiting for events on %s\n", client.InstanceID(), client.InstanceGroup(), *subject)
	<-ctx.Done()
	fmt.Println("shutting down")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var parsed int
		if _, err := fmt.Sscanf(v, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return def
}

func envBoolOr(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch v {
		case "1", "true", "TRUE", "yes", "YES", "on", "ON":
			return true
		case "0", "false", "FALSE", "no", "NO", "off", "OFF":
			return false
		}
	}
	return def
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
