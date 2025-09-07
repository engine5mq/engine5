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

type InstanceGroupIndexSelection struct {
	instanceGroupName string
	index             int
}

type MessageOperator struct {
	instances                     []*ConnectedClient
	waiting                       chan Message
	requestGate                   chan *RequestGateObject
	ongoingRequests               map[string]*OngoingRequest
	instanceGroupSelectionIndexes map[string]int
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
		op.DistrubuteResponses()
		op.DistrubuteReceivedRequests()
	}

}

func (op *MessageOperator) DistrubuteResponses() {
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
}

func (op *MessageOperator) DistrubuteReceivedRequests() {
	messageIds := reflect.ValueOf(op.ongoingRequests).MapKeys()
	messageIdsLength := len(messageIds)
	if messageIdsLength > 0 {

		for i := 0; i < messageIdsLength; i++ {
			or := op.ongoingRequests[messageIds[i].String()]
			if or != nil && or.targetInstance != nil && !or.sent {
				message := or.requestMessage
				pl := Payload{
					Command:   CtRequest,
					Content:   message.content,
					MessageId: message.id,
					Subject:   message.targetSubjectName,
				}
				iSelectionMappingKey := message.targetSubjectName + "_" + message.targetInstanceGroupName

				relatedInstances := []*ConnectedClient{}

				for instanceIndex := 0; instanceIndex < len(op.instances); instanceIndex++ {
					instance := op.instances[instanceIndex]
					hasSubject := instance.IsListening(message.targetSubjectName)
					filteringInstanceGroup := (message.targetSubjectName != "") || message.targetInstanceGroupName == instance.instanceGroup
					if hasSubject && filteringInstanceGroup {
						relatedInstances = append(relatedInstances, instance)
					}

				}

				instanceLength := len(relatedInstances)
				if instanceLength > 0 {
					isi := op.SelectIndex(iSelectionMappingKey, instanceLength)
					instance := relatedInstances[isi]
					instance.Write(pl)
					or.sent = true
				} else {
					or.targetInstance.Write(Payload{
						Command:           CtResponseError,
						Content:           "No clients matching the criteria were found.",
						ResponseErrorSide: CtResponseErrorSideClient,
					})
					delete(op.ongoingRequests, or.requestMessage.id)
				}
			}

		}

	}
}

func (op *MessageOperator) SelectIndex(mappingName string, maxLength int) int {
	var indexInfo, hasIndex = op.instanceGroupSelectionIndexes[mappingName]
	if !hasIndex {
		op.instanceGroupSelectionIndexes[mappingName] = 0
		return 0
	} else {
		newIndex := (indexInfo + 1) % maxLength
		op.instanceGroupSelectionIndexes[mappingName] = newIndex
		return newIndex
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
