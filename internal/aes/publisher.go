package aes

import (
	"bytes"
	"context"
	"encoding/binary"
	"engine5/internal/server"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/shamaton/msgpack"
)

type BrokerPublisher struct {
	info     ConnectionInfo
	mu       sync.Mutex
	conn     net.Conn
	instance string
}

func NewBrokerPublisher(info ConnectionInfo) *BrokerPublisher {
	return &BrokerPublisher{info: info, instance: "aes-" + uuid.NewString()}
}

func (p *BrokerPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn == nil {
		return nil
	}
	err := p.conn.Close()
	p.conn = nil
	return err
}

func (p *BrokerPublisher) Publish(ctx context.Context, message RetryMessage) (result error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()
	fresh := p.conn == nil
	if p.conn == nil {
		conn, err := dialContext(ctx, net.JoinHostPort(p.info.Host, p.info.Port), p.info.UseTLS, false, p.info.CAFile)
		if err != nil {
			// No bytes sent: unlike a broken connection after write, this is unambiguous.
			return fmt.Errorf("%w: connect: %v", ErrRetryRejected, err)
		}
		p.conn = conn
	}
	conn := p.conn
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	defer func() {
		if result != nil {
			conn.Close()
			p.conn = nil
		}
	}()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("%w: set deadline: %v", ErrRetryRejected, err)
	}
	if fresh {
		if err := writePayload(conn, server.Payload{Command: server.CtConnect, InstanceId: p.instance,
			InstanceGroup: p.instance, AuthKey: p.info.Key}); err != nil {
			return fmt.Errorf("%w: handshake: %v", ErrRetryRejected, err)
		}
		reply, err := readPayload(conn)
		if err != nil {
			return fmt.Errorf("%w: handshake: %v", ErrRetryRejected, err)
		}
		if reply.Command != server.CtConnectSuccess {
			return fmt.Errorf("%w: %s: %s", ErrRetryRejected, reply.Command, reply.Content)
		}
	}
	if err := writePayload(conn, server.Payload{
		Command: server.CtRetryEvent, MessageId: message.MessageID,
		OriginalMessageId: message.OriginalMessageID, Subject: message.Subject,
		InstanceGroup: message.Group, Content: message.Content,
	}); err != nil {
		return err
	}
	reply, err := readPayload(conn)
	if err != nil {
		return err
	}
	if reply.MessageId != message.MessageID || reply.Subject != message.Subject {
		return fmt.Errorf("unexpected retry reply correlation")
	}
	switch reply.Command {
	case server.CtRetryAccepted:
		return nil
	case server.CtRetryNoTarget:
		return ErrNoTarget
	case server.CtRetryError, server.CtUnauthorized:
		return fmt.Errorf("%w: %s", ErrRetryRejected, reply.Content)
	default:
		return fmt.Errorf("unexpected retry reply command %q", reply.Command)
	}
}

func writePayload(writer io.Writer, payload server.Payload) error {
	data, err := msgpack.Marshal(payload)
	if err != nil {
		return err
	}
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	_, err = io.Copy(writer, bytes.NewReader(append(header, data...)))
	return err
}

func readPayload(reader io.Reader) (server.Payload, error) {
	var payload server.Payload
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return payload, err
	}
	length := binary.BigEndian.Uint32(header[:])
	if length == 0 || length > maxScannerBuffer {
		return payload, fmt.Errorf("invalid broker frame length %d", length)
	}
	data := make([]byte, int(length))
	if _, err := io.ReadFull(reader, data); err != nil {
		return payload, err
	}
	err := msgpack.Unmarshal(data, &payload)
	return payload, err
}
