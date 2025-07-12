package main

import "slices"

type QueueOperator struct {
	instances []*ConnectedClient
	messages  []*Message
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
	op.messages = append(op.messages, msg)
}

func (op *QueueOperator) LoopMessages() {
	for {
		for messageIndex := 0; messageIndex < len(op.messages); messageIndex++ {
			msg := op.messages[messageIndex]
			if msg.status != MsgOpOk {
				for instanceIndex := 0; instanceIndex < len(op.instances); instanceIndex++ {
					instance := op.instances[instanceIndex]
					hasSubject := slices.Contains(instance.listeningSubjects, msg.targetSubjectName)
					if hasSubject {
						pl := Payload{
							command:   msg.commandType,
							content:   msg.content,
							subject:   msg.targetSubjectName,
							messageId: msg.id,
						}
						instance.Write(pl)
					}
				}
				msg.status = MsgOpOk
			}
		}
	}
}
