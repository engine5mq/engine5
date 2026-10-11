package aes

import (
	"bytes"
	"context"
	"encoding/binary"
	"engine5/internal/server"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestBrokerPublisherHandshakeCorrelationAndReconnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	done := make(chan error, 1)
	go func() {
		commands := []string{server.CtRetryAccepted, server.CtRetryNoTarget, server.CtRetryAccepted}
		position := 0
		for position < len(commands) {
			conn, err := listener.Accept()
			if err != nil {
				done <- err
				return
			}
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			connect, err := readPayload(conn)
			if err != nil || connect.Command != server.CtConnect || connect.AuthKey != "test-key" {
				conn.Close()
				done <- fmt.Errorf("invalid handshake: %+v, %v", connect, err)
				return
			}
			if err := writePayload(conn, server.Payload{Command: server.CtConnectSuccess}); err != nil {
				conn.Close()
				done <- err
				return
			}
			for position < len(commands) {
				payload, err := readPayload(conn)
				if err != nil || payload.Command != server.CtRetryEvent || payload.Content != "body" ||
					payload.OriginalMessageId != "original" || payload.InstanceGroup != "orders" {
					conn.Close()
					done <- fmt.Errorf("invalid retry: %+v, %v", payload, err)
					return
				}
				command := commands[position]
				position++
				if err := writePayload(conn, server.Payload{Command: command, MessageId: payload.MessageId, Subject: payload.Subject}); err != nil {
					conn.Close()
					done <- err
					return
				}
				if command == server.CtRetryNoTarget {
					break
				}
			}
			conn.Close()
		}
		done <- nil
	}()
	publisher := NewBrokerPublisher(ConnectionInfo{Host: host, Port: port, Key: "test-key"})
	defer publisher.Close()
	for i := 0; i < 3; i++ {
		err := publisher.Publish(context.Background(), RetryMessage{MessageID: fmt.Sprint(i), OriginalMessageID: "original",
			Subject: "order.created", Group: "orders", Content: "body"})
		if i == 1 {
			if !errors.Is(err, ErrNoTarget) {
				t.Fatalf("expected no target, got %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPayloadFramesAndSizeLimit(t *testing.T) {
	var buffer bytes.Buffer
	want := server.Payload{Command: server.CtEvent, Subject: "x", Content: "body\x04with delimiter", MessageId: "id"}
	if err := writePayload(&buffer, want); err != nil {
		t.Fatal(err)
	}
	got, err := readPayload(&buffer)
	if err != nil || got != want {
		t.Fatalf("frame roundtrip: %+v, %v", got, err)
	}
	for _, length := range []uint32{0, maxScannerBuffer + 1} {
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], length)
		if _, err := readPayload(bytes.NewReader(header[:])); err == nil {
			t.Fatal("invalid frame size accepted")
		}
	}
}

func TestTapCloseInterruptsIdleRead(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	manager := NewTapManager(nil)
	done := make(chan struct{})
	go func() {
		manager.Connect(ConnectionInfo{Host: host, Port: port})
		close(done)
	}()
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	manager.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("tap close did not interrupt idle connection")
	}
}
