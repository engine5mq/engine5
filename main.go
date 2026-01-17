package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
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

	if enableTLS {
		tlsConf, err := tlsConfig.CreateTLSConfig()
		if err != nil {
			log.Fatalf("Failed to create TLS config: %v", err)
		}
		ln, err = tls.Listen("tcp", ":"+port, tlsConf)
		if err != nil {
			log.Fatalf("Failed to listen on TLS: %v", err)
		}
		fmt.Println("Engine5 is starting with TLS enabled")
	} else {
		ln, err = net.Listen("tcp", ":"+port)
		if err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
		fmt.Println("WARNING: Engine5 is starting WITHOUT TLS - not recommended for production")
	}

	fmt.Println("Engine5 is being started")
	fmt.Printf("Listening on port %s (TLS: %v, Auth: %v)\n", port, enableTLS, authConfig.RequireAuth)

	mainOperator := MessageOperator{
		instances:                     []*ConnectedClient{},
		waiting:                       make(chan Message),
		ongoingRequests:               make(map[string]*OngoingRequest),
		requestGate:                   make(chan *RequestGateObject),
		instanceGroupSelectionIndexes: make(map[string]*InstanceGroupIndexSelection),
		clientConnectionQueue:         NewTaskQueue(1),
		authConfig:                    authConfig,
	}

	go mainOperator.LoopMessages()
	go mainOperator.LoopRequests()

	// Connection timeout and limits
	maxConnections := getEnvWithDefaultInt("MAX_CONNECTIONS", 1000)
	connectionTimeout := time.Duration(getEnvWithDefaultInt("CONNECTION_TIMEOUT", 30)) * time.Second
	activeConnections := 0

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Connection Error: %v\n", err)
			continue
		}

		// Check connection limits
		if activeConnections >= maxConnections {
			fmt.Println("Maximum connections reached, rejecting new connection")
			conn.Close()
			continue
		}

		// Set connection timeout
		conn.SetDeadline(time.Now().Add(connectionTimeout))

		fmt.Printf("Incoming connection from %s\n", conn.RemoteAddr().String())
		activeConnections++

		go func() {
			defer func() {
				activeConnections--
			}()
			handleConnection(conn, &mainOperator)
		}()
	}
}

func handleConnection(conn net.Conn, op *MessageOperator) {
	var connCl = ConnectedClient{
		died:       true,
		writeQueue: make(chan []byte, 100),
		authClient: &AuthenticatedClient{
			IsAuth: false,
		},
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
