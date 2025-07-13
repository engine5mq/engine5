package main

import (
	"slices"
	"time"
)

type QueueOperator struct {
	instances []*ConnectedClient
	waiting   []*Message
	sent      []*Message
}

func (op *QueueOperator) addConnectedClient(client *ConnectedClient) {
	op.instances = append(op.instances, client)
	client.SetOperator(op)
}

func (op *QueueOperator) removeConnectedClient(clientId string) {
	// currentIndex := slices.Index(op.instances, op)
	var instances []*ConnectedClient = []*ConnectedClient{}
	for i := 0; i < len(op.instances); i++ {
		if op.instances[i].instanceName != clientId {
			instances = append(instances, op.instances[0])
		}
	}
	op.instances = instances
}

func (op *QueueOperator) addMessage(msg *Message) {
	op.waiting = append(op.waiting, msg)
	go op.LoopMessages()
}

func (op *QueueOperator) LoopMessages() {
	for {

		waitingLength := len(op.waiting)
		if waitingLength == 0 {
			break
		}
		oldWaitingListindp := make([]*Message, waitingLength)
		copy(oldWaitingListindp, op.waiting)

		sent := []*Message{}
		failed := []*Message{}
		for messageIndex := 0; messageIndex < len(oldWaitingListindp); messageIndex++ {
			msg := oldWaitingListindp[messageIndex]
			if msg.status != MsgOpOk {
				for instanceIndex := 0; instanceIndex < len(op.instances); instanceIndex++ {
					instance := op.instances[instanceIndex]
					hasSubject := slices.Contains(instance.listeningSubjects, msg.targetSubjectName)
					if hasSubject {
						pl := Payload{
							Command:           msg.commandType,
							Content:           msg.content,
							Subject:           msg.targetSubjectName,
							MessageId:         msg.id,
							CreatedTime:       msg.createdTime.String(),
							LastOperationTime: msg.lastOperationTime.String(),
						}
						instance.Write(pl)
					}
				}
				msg.status = MsgOpOk
				msg.lastOperationTime = time.Now()
				sent = append(sent, msg)
			}
		}
		op.sent = sent
		op.waiting = failed
	}
}
