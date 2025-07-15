package main

type OngoingRequest struct {
	targetInstance *ConnectedClient
}

type QueueOperator struct {
	instances      []*ConnectedClient
	waiting        []*Message
	intermediate   []*Message
	isWorking      bool
	ongoingRequest map[string]*OngoingRequest
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
	op.hold()
	op.instances = append(op.instances, client)
	client.SetOperator(op)
	op.release()
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

func (op *QueueOperator) addMessage(msg *Message) {
	op.hold()
	op.waiting = append(op.waiting, msg)
	op.release()
	go op.LoopMessages()
}

func (op *QueueOperator) LoopMessages() {
	for {
		op.hold()

		waitingLength := len(op.waiting)
		if waitingLength == 0 {
			op.release()
			break
		}

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
			if msg.commandType == CtRequest {
				// if op.ongoingRequest == nil {
				// 	op.ongoingRequest = make(map[string]*OngoingRequest)
				// }
				// op.ongoingRequest[msg.id] = &OngoingRequest{targetInstance: }
			}
			if msg.commandType == CtResponse {

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

func (op *QueueOperator) SendRequestToClient(msg *Message) {
	op.hold()
	if op.ongoingRequest == nil {
		op.ongoingRequest = make(map[string]*OngoingRequest)
	}
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
			op.ongoingRequest[msg.id] = &OngoingRequest{targetInstance: insta}
			instance.Write(pl)
		}
	}
	op.release()
}
