package main

import (
	"time"

	"github.com/google/uuid"
)

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
	createdTime       time.Time
	lastOperationTime time.Time
}

func MessageFromPayload(pl Payload) *Message {
	return &Message{
		content:           pl.Content,
		id:                uuid.NewString(),
		targetSubjectName: pl.Subject,
		commandType:       pl.Command,
		status:            MsgOpCreated,
		createdTime:       time.Now(),
		lastOperationTime: time.Now(),
	}
}
