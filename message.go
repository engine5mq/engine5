package main

const (
	MsgOpCreated    = "CREATED"
	MsgOpProcessing = "PROCESSING"
	MsgOpOk         = "OK"
)

type Message struct {
	content           string
	id                string
	targetSubjectName string
	commandType       string
	status            string
	fromInstanceName  string
	toInstanceName    string
}

func MessageFromPayload(pl Payload, cc ConnectedClient) *Message {
	// todo: eğer id yoksa hata fırlat
	return &Message{
		content:           pl.Content,
		id:                pl.MessageId,
		targetSubjectName: pl.Subject,
		commandType:       pl.Command,
		status:            MsgOpCreated,
		fromInstanceName:  cc.instanceName,
		toInstanceName:    cc.instanceName,
	}
}
