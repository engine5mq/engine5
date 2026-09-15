package server

import "testing"

func TestConnectedClientBeSureConnection_NoPanicWhenOperatorMissing(t *testing.T) {
	client := &ConnectedClient{
		writeQueue: make(chan []byte, 10),
		authClient: &AuthenticatedClient{IsAuth: false},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BeSureConnection panicked with nil operator: %v", r)
		}
	}()

	client.BeSureConnection(Payload{Command: CtConnect, InstanceId: "client-1"})
}
