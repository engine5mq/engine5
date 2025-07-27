package main

import (
	"reflect"

	"github.com/google/uuid"
)

type OngoingRequest struct {
	targetInstance *ConnectedClient
	requestMessage *Message
	sent           bool
}

type MessageOperator struct {
	instances       map[string]*ConnectedClient
	waiting         chan Message
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
		messageIds := []reflect.Value{}
		messageIdsLength := 0
		addAndWaitToGlobalTaskQueue(func() {
			messageIds = reflect.ValueOf(op.ongoingRequests).MapKeys()
			messageIdsLength = len(messageIds)
		})
		if messageIdsLength > 0 {

			for i := 0; i < messageIdsLength; i++ {
				var or *OngoingRequest = nil
				addAndWaitToGlobalTaskQueue(func() {
					or = op.ongoingRequests[messageIds[i].String()]
				})
				if or != nil && or.targetInstance != nil && !or.sent {
					message := or.requestMessage
					for instanceIndex := range op.instances {
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
	addToGlobalTaskQueue(func() {
		op.ongoingRequests[message.id] = &OngoingRequest{targetInstance: clientRequesting, requestMessage: &message}
	})
}

func (op *MessageOperator) respondRequest(messageIncoming Message) {
	var ongoingReq *OngoingRequest = nil
	addAndWaitToGlobalTaskQueue(func() {
		ongoingReq = op.ongoingRequests[messageIncoming.ResponseOfMessageId]
	})
	if ongoingReq != nil {
		ongoingReq.targetInstance.Write(Payload{
			Command:             CtResponse,
			Content:             messageIncoming.content,
			Subject:             messageIncoming.targetSubjectName,
			ResponseOfMessageId: messageIncoming.ResponseOfMessageId,
		})
		addAndWaitToGlobalTaskQueue(func() {
			delete(op.ongoingRequests, messageIncoming.ResponseOfMessageId)
		})
	}

}

func (op *MessageOperator) addConnectedClient(client *ConnectedClient) {
	addToGlobalTaskQueue(func() {
		if op.instances[client.instanceName] != nil {
			println("Has a client name that same instance name. Renaming...")
			client.instanceName = client.instanceName + uuid.NewString()
			println("Renamed to " + client.instanceName)
		}
		op.instances[client.instanceName] = client
	})

	client.SetOperator(op)
	client.writeQueue = make(chan []byte)
}

func (op *MessageOperator) removeConnectedClient(clientId string) {

	addAndWaitToGlobalTaskQueue(func() {
		delete(op.instances, clientId)
	})

	// op.instances = instances

}

func (op *MessageOperator) addEvent(msg Message) {
	op.waiting <- msg
}

func (op *MessageOperator) PublishEventMessage(msg Message) {
	// instanceCount := 0
	// instanceCount = len(op.instances)
	sentGroups := make(map[string]bool)

	for instanceId := range op.instances {
		instance := op.instances[instanceId]
		var sentGroupVal, sentGroupExist = false, false

		if instance.instanceGroup != "" {
			sentGroupVal, sentGroupExist = sentGroups[instance.instanceGroup]
		}

		if instance != nil && (!sentGroupExist || !sentGroupVal) {
			hasSubject := instance.IsListening(msg.targetSubjectName)

			if hasSubject {
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

	// for instanceIndex := 0; instanceIndex < instanceCount; instanceIndex++ {

	// }
}
