package main

import (
	"reflect"
)

type OngoingRequest struct {
	targetInstance *ConnectedClient
	requestMessage *Message
}

type QueueOperator struct {
	instances       []*ConnectedClient
	waiting         []*Message
	intermediate    []*Message
	isWorking       bool
	ongoingRequests map[string]*OngoingRequest
}

func (op *QueueOperator) waitForFinish() {
	for {

		if !op.isWorking {
			break
		}
	}
}

func (op *QueueOperator) hold() {
	op.waitForFinish()

	op.isWorking = true
}

func (op *QueueOperator) addRequest(message *Message, clientRequesting *ConnectedClient) {

	op.ongoingRequests[message.id] = &OngoingRequest{targetInstance: clientRequesting, requestMessage: message}
	go op.LoopRequests()
}

func (op *QueueOperator) respondRequest(messageIncoming *Message) {

	if op.ongoingRequests[messageIncoming.ResponseOfMessageId] != nil {
		ongoingReq := op.ongoingRequests[messageIncoming.ResponseOfMessageId]
		ongoingReq.targetInstance.Write(Payload{
			Command:             CtResponse,
			Content:             messageIncoming.content,
			Subject:             messageIncoming.targetSubjectName,
			ResponseOfMessageId: messageIncoming.ResponseOfMessageId,
		})
		op.ongoingRequests[messageIncoming.ResponseOfMessageId] = nil
	}

}

func (op *QueueOperator) release() {
	op.isWorking = false
}

func (op *QueueOperator) addConnectedClient(client *ConnectedClient) {
	op.hold()
	op.instances = append(op.instances, client)
	client.SetOperator(op)
	op.release()
}

func (op *QueueOperator) findConnectedClient(client *ConnectedClient) {
	// op.hold()
	// op.instances = append(op.instances, client)
	// client.SetOperator(op)
	// op.release()
}

func (op *QueueOperator) removeConnectedClient(clientId string) {
	op.hold()
	var instances []*ConnectedClient = []*ConnectedClient{}
	for i := 0; i < len(op.instances); i++ {
		if op.instances[i].instanceName != clientId {
			instances = append(instances, op.instances[0])
		}
	}
	op.instances = instances
	op.release()
}

func (op *QueueOperator) addEvent(msg *Message) {
	op.hold()
	op.waiting = append(op.waiting, msg)
	op.release()
	go op.LoopMessages()
}

// func (op *QueueOperator) loopRequests() {

// 	requestWaitingMessageIds := reflect.ValueOf(op.ongoingRequests).MapKeys()

// 	for i := 0; i < len(requestWaitingMessageIds); i++ {
// 		requestMessageId := requestWaitingMessageIds[i]
// 		orq := op.ongoingRequests[requestMessageId.String()]

// 	}
// }

func (op *QueueOperator) LoopRequests() {
	for {
		op.hold()
		messageIds := reflect.ValueOf(op.ongoingRequests).MapKeys()
		if len(messageIds) > 0 {

			for i := 0; i < len(messageIds); i++ {
				or := op.ongoingRequests[messageIds[i].String()]
				if or != nil {
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
							break

						}
					}
				}

			}

		} else {
			break
		}
		op.release()
	}

}

func (op *QueueOperator) LoopMessages() {

	op.hold()

	waitingLength := len(op.waiting)
	if waitingLength == 0 {
		op.release()

	} else {
		oldWaitingListindp := make([]*Message, waitingLength)
		copy(oldWaitingListindp, op.waiting)
		op.release()

		sentNow := []*Message{}
		failed := []*Message{}

		for messageIndex := 0; messageIndex < len(oldWaitingListindp); messageIndex++ {
			msg := oldWaitingListindp[messageIndex]
			if msg.commandType == CtEvent {
				op.PublishEventMessage(msg)
			}

			sentNow = append(sentNow, msg)
		}
		op.hold()
		op.intermediate = sentNow
		op.waiting = failed
		op.release()
	}

}

func (op *QueueOperator) PublishEventMessage(msg *Message) {
	op.hold()
	for instanceIndex := 0; instanceIndex < len(op.instances); instanceIndex++ {
		instance := op.instances[instanceIndex]
		hasSubject := instance.IsListening(msg.targetSubjectName)
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
	op.release()
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
