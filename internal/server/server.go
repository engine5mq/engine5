package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"
)

func Run() {
	exhaust := NewExhaustFromEnv()
	fmt.Println("Engine5 Alpha - (c) 2026 - Tetakent (H.C.G)")

	port := os.Getenv("E5_PORT")
	if port == "" {
		port = "3535"
	}

	// Load security configurations
	tlsConfig := LoadTLSConfig()
	authConfig := LoadAuthConfig()
	enableTLS := getEnvWithDefault("ENABLE_TLS", "true") == "true"

	var ln net.Listener
	var err error
	var serverTLSConf *tls.Config

	if enableTLS {
		serverTLSConf, err = tlsConfig.CreateTLSConfig()
		if err != nil {
			log.Fatalf("Failed to create TLS config: %v", err)
		}
		ln, err = tls.Listen("tcp", ":"+port, serverTLSConf)
		if err != nil {
			log.Fatalf("Failed to listen on TLS: %v", err)
		}
		exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindServerStart, Msg: "Engine5 is starting with TLS enabled"})
	} else {
		ln, err = net.Listen("tcp", ":"+port)
		if err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
		exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindServerStart, Msg: "Engine5 is starting WITHOUT TLS - not recommended for production"})
	}

	exhaust.Emit(ExhaustEvent{
		Level: slog.LevelInfo, Kind: KindServerStart,
		Msg: fmt.Sprintf("Listening on port %s (TLS: %v, Auth: %v)", port, enableTLS, authConfig.RequireAuth),
	})

	// Egzoz çıkışı (Yol B): etkinse ayrı portta tap sunucusunu başlat.
	var tapTLS *tls.Config
	if getEnvWithDefault("E5_EXHAUST_TLS", strconv.FormatBool(enableTLS)) == "true" {
		tapTLS = serverTLSConf
	}
	exhaust.StartTap(tapTLS)

	mainOperator := MessageOperator{
		instances:                     []*ConnectedClient{},
		waiting:                       make(chan Message),
		ongoingRequests:               make(map[string]*OngoingRequest),
		requestGate:                   make(chan *RequestGateObject, 100),
		instanceGroupSelectionIndexes: make(map[string]*InstanceGroupIndexSelection),
		clientConnectionQueue:         NewTaskQueue(1),
		authConfig:                    authConfig,
		haveNewRequests:               make(chan struct{}, 1),
		exhaust:                       exhaust,
	}

	go mainOperator.LoopMessages()
	go mainOperator.LoopRequests()

	// Connection timeout and limits
	maxConnections := getEnvWithDefaultInt("MAX_CONNECTIONS", 1000)
	connectionTimeout := time.Duration(getEnvWithDefaultInt("CONNECTION_TIMEOUT", 86400)) * time.Second
	activeConnections := 0
	activeConnectionsMutex := make(chan struct {
		isIncreasing bool
	}, 1)

	go func() {
		select {
		case isIncreasing := <-activeConnectionsMutex:
			if isIncreasing.isIncreasing {
				activeConnections++
			} else {
				activeConnections--
			}
		default:
			// No update, just continue
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			exhaust.Emit(ExhaustEvent{Level: slog.LevelError, Kind: KindServerError, Msg: "Connection accept error", Err: err.Error()})
			continue
		}

		// Check connection limits
		if activeConnections >= maxConnections {
			exhaust.Emit(ExhaustEvent{Level: slog.LevelWarn, Kind: KindServerError, Msg: "Maximum connections reached, rejecting new connection"})
			conn.Close()
			continue
		}

		// Set connection timeout
		conn.SetDeadline(time.Now().Add(connectionTimeout))

		exhaust.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindClientConnecting, Remote: conn.RemoteAddr().String(), Msg: "Incoming connection"})
		go func() {
			activeConnectionsMutex <- struct{ isIncreasing bool }{isIncreasing: true}
		}()

		go func() {
			defer func() {
				activeConnectionsMutex <- struct{ isIncreasing bool }{isIncreasing: false}
			}()
			handleConnection(conn, &mainOperator)
		}()
	}
}

func handleConnection(conn net.Conn, op *MessageOperator) {
	var connCl = ConnectedClient{
		died:       true,
		writeQueue: make(chan []byte, 100),
		authClient: nil, // Sonradan atanacak
	}

	// Create authenticated wrapper
	authClient := &AuthenticatedClient{
		ConnectedClient: &connCl,
		IsAuth:          false,
	}

	if op.authConfig.RequireAuth {
		authClient.RateLimiter = NewRateLimiter(60) // Default rate limit
	}

	connCl.authClient = authClient
	connCl.SetConnection(conn)
	op.addConnectedClient(&connCl)

	go connCl.ReaderLoop()
	go connCl.WriterLoop()
}

func getEnvWithDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
