package main

import (
	"reflect"

	"github.com/google/uuid"
)

type RequestGateObject struct {
	by              *ConnectedClient
	requestMessage  *Message
	responseMessage *Message
	rescan          bool
}

type OngoingRequest struct {
	targetInstance *ConnectedClient
	requestMessage *Message
	sent           bool
}

type MessageOperator struct {
	instances       []*ConnectedClient
	waiting         chan Message
	requestGate     chan *RequestGateObject
	ongoingRequests map[string]*OngoingRequest
}

func (op *MessageOperator) LoopMessages() {
	for {

		for eventMsg := range op.waiting {
			op.PublishEventMessage(eventMsg)
		}

	}
}

func (op *MessageOperator) LoopRequests() {

	for {

		select {
		case incomingMessage, ok := <-op.requestGate:
			if ok {
				if incomingMessage.requestMessage != nil {
					op.ongoingRequests[incomingMessage.requestMessage.id] = &OngoingRequest{
						targetInstance: incomingMessage.by,
						requestMessage: incomingMessage.requestMessage,
					}
				} else if incomingMessage.responseMessage != nil {
					messageIncoming := incomingMessage.responseMessage
					ongoingReq := op.ongoingRequests[messageIncoming.ResponseOfMessageId]

					ongoingReq.targetInstance.Write(Payload{
						Command:             CtResponse,
						Content:             messageIncoming.content,
						Subject:             messageIncoming.targetSubjectName,
						ResponseOfMessageId: messageIncoming.ResponseOfMessageId,
					})
					delete(op.ongoingRequests, messageIncoming.ResponseOfMessageId)

				}
			}
		}
		// for incomingMessage := range op.requestGate {
		//
		// }

		messageIds := []reflect.Value{}
		messageIdsLength := 0
		// addAndWaitToGlobalTaskQueue(func() {
		messageIds = reflect.ValueOf(op.ongoingRequests).MapKeys()
		messageIdsLength = len(messageIds)
		// })
		if messageIdsLength > 0 {

			for i := 0; i < messageIdsLength; i++ {
				var or *OngoingRequest = nil
				// addAndWaitToGlobalTaskQueue(func() {
				or = op.ongoingRequests[messageIds[i].String()]
				// })
				if or != nil && or.targetInstance != nil && !or.sent {
					message := or.requestMessage
					for instanceIndex := 0; instanceIndex < len(op.instances); instanceIndex++ {

						instance := op.instances[instanceIndex]
						hasSubject := instance.IsListening(message.targetSubjectName)
						if hasSubject {
							pl := Payload{
								Command:   CtRequest,
								Content:   message.content,
								MessageId: message.id,
								Subject:   message.targetSubjectName,
							}
							instance.Write(pl)
							or.sent = true
							break
						}
					}
				}

			}

		}
	}

}

func (op *MessageOperator) addRequest(message Message, clientRequesting *ConnectedClient) {
	op.requestGate <- &RequestGateObject{by: clientRequesting, requestMessage: &message}
}

func (op *MessageOperator) respondRequest(messageIncoming Message) {
	op.requestGate <- &RequestGateObject{responseMessage: &messageIncoming}

}

func (op *MessageOperator) addConnectedClient(client *ConnectedClient) {
	// addToGlobalTaskQueue(func() {
	for instanceExist := range op.instances {
		existInstanceName := op.instances[instanceExist].instanceName
		if client.instanceName == existInstanceName {
			println("Has a client name that same instance name. Renaming...")
			client.instanceName = client.instanceName + uuid.NewString()
			println("Renamed to " + client.instanceName)

		}
	}
	op.instances = append(op.instances, client)

	client.SetOperator(op)
	client.writeQueue = make(chan []byte)
	// })

}

func (op *MessageOperator) removeConnectedClient(clientId string) {
	var instances []*ConnectedClient = []*ConnectedClient{}
	var instanceSize = 0
	// addAndWaitToGlobalTaskQueue(func() {
	instanceSize = len(op.instances)

	for i := 0; i < instanceSize; i++ {
		if op.instances[i].instanceName != clientId {
			instances = append(instances, op.instances[i])
		}
	}

	// })

	op.instances = instances

}

func (op *MessageOperator) addEvent(msg Message) {
	op.waiting <- msg
}

func (op *MessageOperator) PublishEventMessage(msg Message) {
	instanceCount := 0
	instanceCount = len(op.instances)
	sentGroups := make(map[string]bool)

	for instanceIndex := 0; instanceIndex < instanceCount; instanceIndex++ {
		var instance *ConnectedClient = nil
		var hasSubject = false
		instance = op.instances[instanceIndex]
		var sentGroupVal, sentGroupExist = false, false
		if instance != nil {
			if instance.instanceGroup != "" {
				sentGroupVal, sentGroupExist = sentGroups[instance.instanceGroup]
			}

			hasSubject = instance.IsListening(msg.targetSubjectName)

			if hasSubject && (!sentGroupExist || !sentGroupVal) {
				pl := Payload{
					Command:   msg.commandType,
					Content:   msg.content,
					Subject:   msg.targetSubjectName,
					MessageId: msg.id,
				}
				instance.Write(pl)
				if instance.instanceGroup != "" {
					sentGroups[instance.instanceGroup] = true
				}
			}
		}

	}
}
