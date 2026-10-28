package server

import (
	"testing"

	"github.com/shamaton/msgpack"
)

func TestDistributeResponsesPreservesIsError(t *testing.T) {
	for _, name := range []string{"error", "success", "omitted"} {
		t.Run(name, func(t *testing.T) {
			incoming := map[string]interface{}{
				"Command":             CtResponse,
				"Content":             "response body",
				"Subject":             "orders",
				"ResponseOfMessageId": "request-1",
			}
			if name != "omitted" {
				incoming["IsError"] = name == "error"
			}
			data, err := msgpack.Marshal(incoming)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := parsePayloadMsgPack(data)
			if err != nil {
				t.Fatal(err)
			}

			requester := retryTestClient(t, "requester")
			responder := retryTestClient(t, "responder")
			op := &MessageOperator{
				exhaust:     &Exhaust{queue: make(chan ExhaustEvent, 10)},
				requestGate: make(chan *RequestGateObject, 1),
				ongoingRequests: map[string]*OngoingRequest{
					"request-1": {targetInstance: requester},
				},
			}
			responder.operator = op
			responder.ReviewPayload(payload)
			op.DistrubuteResponses()

			got := queuedPayload(t, requester)
			if got.Command != CtResponse || got.IsError != (name == "error") ||
				got.Content != payload.Content || got.Subject != payload.Subject ||
				got.ResponseOfMessageId != payload.ResponseOfMessageId {
				t.Fatalf("unexpected response: %+v", got)
			}
			for _, kind := range []string{KindResponseReceived, KindResponseDelivered} {
				ev := <-op.exhaust.queue
				if ev.Kind != kind || ev.IsError != (name == "error") {
					t.Fatalf("unexpected exhaust event: %+v", ev)
				}
			}
			if len(op.ongoingRequests) != 0 {
				t.Fatal("completed request was not removed")
			}
			if len(requester.writeQueue) != 0 {
				t.Fatal("response was delivered more than once")
			}
		})
	}
}
