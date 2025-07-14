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
}

func MessageFromPayload(pl Payload) *Message {
	// todo: eğer id yoksa hata fırlat
	return &Message{
		content:           pl.Content,
		id:                pl.MessageId,
		targetSubjectName: pl.Subject,
		commandType:       pl.Command,
		status:            MsgOpCreated,
	}
}
