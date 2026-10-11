package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"testing"

	"github.com/shamaton/msgpack"
)

func retryTestClient(t *testing.T, group string) *ConnectedClient {
	t.Helper()
	conn, peer := net.Pipe()
	t.Cleanup(func() { conn.Close(); peer.Close() })
	return &ConnectedClient{connection: conn, instanceName: group + "-1", instanceGroup: group,
		writeQueue: make(chan []byte, 4), listeningSubjects: map[string]bool{"orders": true}}
}

func queuedPayload(t *testing.T, client *ConnectedClient) Payload {
	t.Helper()
	select {
	case data := <-client.writeQueue:
		var payload Payload
		if err := msgpack.Unmarshal(data[4:], &payload); err != nil {
			t.Fatal(err)
		}
		return payload
	default:
		t.Fatal("expected queued payload")
		return Payload{}
	}
}

func TestRetryTargetsExactlyOneConsumerInFailedGroup(t *testing.T) {
	first := retryTestClient(t, "failed")
	second := retryTestClient(t, "failed")
	healthy := retryTestClient(t, "healthy")
	sender := retryTestClient(t, "aes")
	op := &MessageOperator{instances: []*ConnectedClient{healthy, first, second},
		exhaust: &Exhaust{queue: make(chan ExhaustEvent, 10)}}
	msg := MessageFromPayload(Payload{Command: CtRetryEvent, MessageId: "attempt", OriginalMessageId: "original",
		Subject: "orders", Content: "body", InstanceGroup: "failed"})
	msg.retrySender = sender
	op.PublishEventMessage(msg)
	payload := queuedPayload(t, first)
	if payload.Command != CtEvent || payload.MessageId != "attempt" ||
		payload.OriginalMessageId != "original" || payload.Content != "body" {
		t.Fatalf("invalid consumer payload: %+v", payload)
	}
	if len(second.writeQueue) != 0 || len(healthy.writeQueue) != 0 {
		t.Fatal("retry broadcast outside its single target")
	}
	if reply := queuedPayload(t, sender); reply.Command != CtRetryAccepted {
		t.Fatalf("invalid acceptance: %+v", reply)
	}
}

func TestRetryNoTargetAndFullQueue(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-target", true: "queue-full"}[full], func(t *testing.T) {
			sender := retryTestClient(t, "aes")
			op := &MessageOperator{exhaust: &Exhaust{queue: make(chan ExhaustEvent, 10)}}
			want := CtRetryNoTarget
			if full {
				client := retryTestClient(t, "failed")
				client.writeQueue = make(chan []byte)
				op.instances = []*ConnectedClient{client}
				want = CtRetryError
			}
			op.PublishEventMessage(Message{commandType: CtRetryEvent, id: "attempt", targetSubjectName: "orders",
				targetInstanceGroupName: "failed", retrySender: sender})
			if reply := queuedPayload(t, sender); reply.Command != want {
				t.Fatalf("expected %s, got %+v", want, reply)
			}
		})
	}
}

func TestNormalEventStillBroadcastsToEachGroup(t *testing.T) {
	first := retryTestClient(t, "first")
	second := retryTestClient(t, "second")
	op := &MessageOperator{instances: []*ConnectedClient{first, second},
		exhaust: &Exhaust{queue: make(chan ExhaustEvent, 10)}}
	op.PublishEventMessage(Message{commandType: CtEvent, id: "original", targetSubjectName: "orders"})
	if len(first.writeQueue) != 1 || len(second.writeQueue) != 1 {
		t.Fatal("normal event behavior changed")
	}
}

func TestConsumptionEventsUseConnectionGroup(t *testing.T) {
	for _, command := range []string{CtConsumingSuccess, CtConsumingError} {
		client := retryTestClient(t, "actual")
		ex := &Exhaust{queue: make(chan ExhaustEvent, 10)}
		client.operator = &MessageOperator{exhaust: ex}
		client.ReviewPayload(Payload{Command: command, Subject: "orders", MessageId: "original", InstanceGroup: "spoofed"})
		ev := <-ex.queue
		if ev.Group != "actual" || ev.Instance != "actual-1" || ev.MessageId != "original" {
			t.Fatalf("wrong consumption correlation: %+v", ev)
		}
	}
}

func TestRetryRequiresPublishPermission(t *testing.T) {
	client := retryTestClient(t, "aes")
	client.operator = &MessageOperator{authConfig: &AuthConfig{RequireAuth: true}}
	if err := client.AuthorizePayload(Payload{Command: CtRetryEvent, Subject: "orders"}); err == nil {
		t.Fatal("unauthenticated retry allowed")
	}
	if commandToAction(CtRetryEvent) != "publish" {
		t.Fatal("retry not mapped to publish permissions")
	}
}

func TestTapBodyUnmaskedAndConsoleBodyMasked(t *testing.T) {
	for _, include := range []bool{false, true} {
		var output bytes.Buffer
		console := &ConsoleSink{logger: slog.New(slog.NewJSONHandler(&output, nil)), minLevel: slog.LevelInfo,
			includeContent: include}
		ex := &Exhaust{queue: make(chan ExhaustEvent, 1)}
		ex.Emit(ExhaustEvent{Level: slog.LevelInfo, Kind: KindEventDelivered, Content: "original-body"})
		ev := <-ex.queue
		console.Write(ev)
		var printed struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(output.Bytes(), &printed); err != nil {
			t.Fatal(err)
		}
		want := exhaustContentMasked
		if include {
			want = "original-body"
		}
		if printed.Content != want {
			t.Fatalf("console body = %q, want %q", printed.Content, want)
		}
		sub := &tapSubscriber{queue: make(chan []byte, 1)}
		tap := &TapSink{subscribers: map[*tapSubscriber]struct{}{sub: {}}}
		tap.Write(ev)
		var tapped struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(<-sub.queue, &tapped); err != nil {
			t.Fatal(err)
		}
		if tapped.Content != "original-body" {
			t.Fatalf("tap body was masked: %q", tapped.Content)
		}
	}
}
