package main

import (
	"reflect"
)

type OngoingRequest struct {
	targetInstance *ConnectedClient
	requestMessage *Message
	sent           bool
}

type MessageOperator struct {
	instances    []*ConnectedClient
	waiting      []*Message
	intermediate []*Message
	// isWorking       bool
	ongoingRequests map[string]*OngoingRequest
}

// func (op *QueueOperator) waitForFinish() {
// 	for {

// 		if !op.isWorking {
// 			break
// 		}
// 	}
// }

func (op *MessageOperator) LoopRequests() {

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

func (op *MessageOperator) addRequest(message *Message, clientRequesting *ConnectedClient) {

	op.ongoingRequests[message.id] = &OngoingRequest{targetInstance: clientRequesting, requestMessage: message}
	go op.LoopRequests()
}

func (op *MessageOperator) respondRequest(messageIncoming *Message) {
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
		addToGlobalTaskQueue(func() {
			delete(op.ongoingRequests, messageIncoming.ResponseOfMessageId)

		})
	}

}

func (op *MessageOperator) addConnectedClient(client *ConnectedClient) {

	addToGlobalTaskQueue(
		func() {
			op.instances = append(op.instances, client)
		})
	client.SetOperator(op)

}

func (op *MessageOperator) removeConnectedClient(clientId string) {
	var instances []*ConnectedClient = []*ConnectedClient{}
	for i := 0; i < len(op.instances); i++ {
		if op.instances[i].instanceName != clientId {
			instances = append(instances, op.instances[0])
		}
	}
	addToGlobalTaskQueue(
		func() {
			op.instances = instances
		})
}

func (op *MessageOperator) addEvent(msg *Message) {
	addToGlobalTaskQueue(func() {
		op.waiting = append(op.waiting, msg)
	})
	go op.LoopMessages()

}

// func (op *QueueOperator) loopRequests() {

// 	requestWaitingMessageIds := reflect.ValueOf(op.ongoingRequests).MapKeys()

// 	for i := 0; i < len(requestWaitingMessageIds); i++ {
// 		requestMessageId := requestWaitingMessageIds[i]
// 		orq := op.ongoingRequests[requestMessageId.String()]

// 	}
// }

func (op *MessageOperator) LoopAll() {

}

func (op *MessageOperator) LoopMessages() {

	waitingLength := 0
	addAndWaitToGlobalTaskQueue(func() {
		waitingLength = len(op.waiting)
	})
	if waitingLength > 0 {
		oldWaitingListindp := make([]*Message, waitingLength)

		addAndWaitToGlobalTaskQueue(func() {
			copy(oldWaitingListindp, op.waiting)
		})

		sentNow := []*Message{}
		failed := []*Message{}

		for messageIndex := 0; messageIndex < len(oldWaitingListindp); messageIndex++ {
			msg := oldWaitingListindp[messageIndex]
			// if msg.commandType == CtEvent {
			// }
			op.PublishEventMessage(msg)

			sentNow = append(sentNow, msg)

		}
		addToGlobalTaskQueue(func() {
			op.intermediate = sentNow
			op.waiting = failed
		})
	}

}

func (op *MessageOperator) PublishEventMessage(msg *Message) {
	instanceCount := 0
	addAndWaitToGlobalTaskQueue(func() {
		instanceCount = len(op.instances)
	})
	for instanceIndex := 0; instanceIndex < instanceCount; instanceIndex++ {
		var instance *ConnectedClient = nil
		var hasSubject = false
		addToGlobalTaskQueue(func() {
			instance = op.instances[instanceIndex]
			if instance != nil {
				hasSubject = instance.IsListening(msg.targetSubjectName)
			}
		})

		if hasSubject {
			pl := Payload{
				Command:   msg.commandType,
				Content:   msg.content,
				Subject:   msg.targetSubjectName,
				MessageId: msg.id,
			}
			instance.Write(pl)
		}

	}
}

// func (op *QueueOperator) SendRequestToClient(msg *Message) {
// 	op.hold()
// 	if op.ongoingRequest == nil {
// 		op.ongoingRequest = make(map[string]*OngoingRequest)
// 	}
// 	for instanceIndex := 0; instanceIndex < len(op.instances); instanceIndex++ {
// 		instance := op.instances[instanceIndex]
// 		hasSubject := instance.IsListening(msg.targetSubjectName)
// 		if hasSubject {
// 			pl := Payload{
// 				Command:   msg.commandType,
// 				Content:   msg.content,
// 				Subject:   msg.targetSubjectName,
// 				MessageId: msg.id,
// 			}
// 			op.ongoingRequest[msg.id] = &OngoingRequest{targetInstance: insta}
// 			instance.Write(pl)
// 		}
// 	}
// 	op.release()
// }
