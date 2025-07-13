package main

import (
	"slices"
	"time"
)

type QueueOperator struct {
	instances []*ConnectedClient
	waiting   []*Message
	sent      []*Message
	isWorking bool
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

func (op *QueueOperator) removeConnectedClient(clientId string) {
	op.hold()
	// currentIndex := slices.Index(op.instances, op)
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

		sent := []*Message{}
		failed := []*Message{}

		for messageIndex := 0; messageIndex < len(oldWaitingListindp); messageIndex++ {
			msg := oldWaitingListindp[messageIndex]
			if msg.status != MsgOpOk {
				op.hold()
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
				op.release()
				msg.status = MsgOpOk
				msg.lastOperationTime = time.Now()
				sent = append(sent, msg)
			}
		}
		op.hold()
		op.sent = sent
		op.waiting = failed
		op.release()
	}
}
