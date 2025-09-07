package main

import (
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
	index int
}

type MessageOperator struct {
	instances                     []*ConnectedClient
	waiting                       chan Message
	requestGate                   chan *RequestGateObject
	ongoingRequests               map[string]*OngoingRequest
	instanceGroupSelectionIndexes map[string]*InstanceGroupIndexSelection
}

// LoopMessages listens for event messages and publishes them.
func (op *MessageOperator) LoopMessages() {
	for eventMsg := range op.waiting {
		op.PublishEventMessage(eventMsg)
	}
}

// LoopRequests handles distributing responses and received requests.
func (op *MessageOperator) LoopRequests() {
	for {
		op.DistrubuteResponses()
		op.DistrubuteReceivedRequests()
	}
}

// DistrubuteResponses processes incoming requests and responses.
func (op *MessageOperator) DistrubuteResponses() {
	select {
	case incomingMessage, ok := <-op.requestGate:
		if !ok || incomingMessage == nil {
			return
		}
		if incomingMessage.requestMessage != nil {
			op.ongoingRequests[incomingMessage.requestMessage.id] = &OngoingRequest{
				targetInstance: incomingMessage.by,
				requestMessage: incomingMessage.requestMessage,
			}
		} else if incomingMessage.responseMessage != nil {
			messageIncoming := incomingMessage.responseMessage
			ongoingReq, exists := op.ongoingRequests[messageIncoming.ResponseOfMessageId]
			if exists && ongoingReq.targetInstance != nil {
				ongoingReq.targetInstance.Write(Payload{
					Command:             CtResponse,
					Content:             messageIncoming.content,
					Subject:             messageIncoming.targetSubjectName,
					ResponseOfMessageId: messageIncoming.ResponseOfMessageId,
				})
				delete(op.ongoingRequests, messageIncoming.ResponseOfMessageId)
			}
		}
	default:
		// Non-blocking select
	}
}

// DistrubuteReceivedRequests sends requests to appropriate clients.
func (op *MessageOperator) DistrubuteReceivedRequests() {
	for id, or := range op.ongoingRequests {
		if or == nil || or.targetInstance == nil || or.sent {
			continue
		}
		message := or.requestMessage
		pl := Payload{
			Command:   CtRequest,
			Content:   message.content,
			MessageId: message.id,
			Subject:   message.targetSubjectName,
		}
		iSelectionMappingKey := message.targetSubjectName + "_" + message.targetInstanceGroupName

		var relatedInstances []*ConnectedClient
		for _, instance := range op.instances {
			if instance == nil {
				continue
			}
			hasSubject := instance.IsListening(message.targetSubjectName)
			filteringInstanceGroup := (message.targetInstanceGroupName == "") || message.targetInstanceGroupName == instance.instanceGroup
			if hasSubject && filteringInstanceGroup {
				relatedInstances = append(relatedInstances, instance)
			}
		}

		if len(relatedInstances) > 0 {
			isi := op.SelectIndex(iSelectionMappingKey, len(relatedInstances))
			instance := relatedInstances[isi]
			instance.Write(pl)
			or.sent = true
		} else {
			or.targetInstance.Write(Payload{
				Command:           CtResponseError,
				Content:           "No clients matching the criteria were found.",
				ResponseErrorSide: CtResponseErrorSideClient,
			})
			delete(op.ongoingRequests, id)
		}
	}
}

// SelectIndex returns the next index for round-robin selection.
func (op *MessageOperator) SelectIndex(mappingName string, maxLength int) int {
	indexInfo, hasIndex := op.instanceGroupSelectionIndexes[mappingName]
	if !hasIndex {
		op.instanceGroupSelectionIndexes[mappingName] = &InstanceGroupIndexSelection{index: 0}
		return 0
	}
	newIndex := (indexInfo.index + 1) % maxLength
	op.instanceGroupSelectionIndexes[mappingName].index = newIndex
	return newIndex
}

// addRequest queues a new request.
func (op *MessageOperator) addRequest(message Message, clientRequesting *ConnectedClient) {
	op.requestGate <- &RequestGateObject{by: clientRequesting, requestMessage: &message}
}

// respondRequest queues a response to a request.
func (op *MessageOperator) respondRequest(messageIncoming Message) {
	op.requestGate <- &RequestGateObject{responseMessage: &messageIncoming}
}

// addConnectedClient adds a new client, ensuring unique instance names.
func (op *MessageOperator) addConnectedClient(client *ConnectedClient) {
	for _, existInstance := range op.instances {
		if client.instanceName == existInstance.instanceName {
			println("Has a client name that same instance name. Renaming...")
			client.instanceName = client.instanceName + uuid.NewString()
			println("Renamed to " + client.instanceName)
		}
	}
	op.instances = append(op.instances, client)
	client.SetOperator(op)
	client.writeQueue = make(chan []byte)
}

// removeConnectedClient removes a client by its instance name.
func (op *MessageOperator) removeConnectedClient(clientId string) {
	var instances []*ConnectedClient
	for _, inst := range op.instances {
		if inst.instanceName != clientId {
			instances = append(instances, inst)
		}
	}
	op.instances = instances
}

// addEvent queues an event message.
func (op *MessageOperator) addEvent(msg Message) {
	op.waiting <- msg
}

// PublishEventMessage sends an event message to all relevant clients.
func (op *MessageOperator) PublishEventMessage(msg Message) {
	sentGroups := make(map[string]bool)
	for _, instance := range op.instances {
		if instance == nil {
			continue
		}
		if instance.instanceGroup != "" {
			if sentGroups[instance.instanceGroup] {
				continue
			}
		}
		if instance.IsListening(msg.targetSubjectName) {
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
