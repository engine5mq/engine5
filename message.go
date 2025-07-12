package main

import "github.com/google/uuid"

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
	return &Message{
		content:           pl.content,
		id:                uuid.NewString(),
		targetSubjectName: pl.subject,
		commandType:       pl.command,
		status:            MsgOpCreated,
	}
}
