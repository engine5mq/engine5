package main

import (
	"encoding/json"
)

const (
	CtConnect        = "CONNECT"
	CtConnectSuccess = "CONNECT_SUCCESS"
	CtRecieved       = "RECIEVED"
	CtEvent          = "EVENT"
	CtRequest        = "REQUEST"
	CtResponse       = "RESPONSE"
	CtListen         = "LISTEN"
	CtClose          = "CLOSE"
)

type Payload struct {
	Command           string `json:"command"`
	Content           string `json:"content"`
	Subject           string `json:"subject"`
	InstanceId        string `json:"instanceId"`
	MessageId         string `json:"messageId"`
	CreatedTime       string `json:"createdTime"`
	LastOperationTime string `json:"lastOperationTime"`
}

func (p Payload) toJson() (string, error) {
	jsonBytes, err := json.Marshal(&p)
	if err != nil {
		return "", err
	}
	return string(jsonBytes[:]), err
}

func parsePayload(jsonString string) (p Payload, e error) {
	var person Payload

	err := json.Unmarshal([]byte(jsonString), &person)
	if err != nil {
		return person, err
	}

	return person, nil
}
