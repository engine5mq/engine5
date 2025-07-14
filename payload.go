package main

import (
	"github.com/shamaton/msgpack"
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

/**
* DİKKAT: SAYI TİPİ(INTEGER) YERİNE STRİNG KULLANIN. '4' BYTE İLE AYRILIYOR ANCAK PAYLOADIN İÇİNDE 4 SAYISI OLMASI (INTEGER - FIXINT) YANLIŞ KESİLMESİNE NEDEN OLUYOR
* USE STRING INSTEAD OF NUMBER TYPE. IT IS SEPARATED BY '4' BUT THE PRESENCE OF THE NUMBER 4 (INTEGER - FIXINT) IN THE PAYLOAD CAUSES IT TO BE TRUNCTURED WRONGLY
 */
type Payload struct {
	Command           string `json:"command"`
	Content           string `json:"content"`
	Subject           string `json:"subject"`
	InstanceId        string `json:"instanceId"`
	MessageId         string `json:"messageId"`
	CreatedTime       string `json:"createdTime"`
	LastOperationTime string `json:"lastOperationTime"`
	// Number            string `json:"number"`
}

// func (p Payload) toJson() (string, error) {
// 	jsonBytes, err := json.Marshal(&p)
// 	if err != nil {
// 		return "", err
// 	}
// 	return string(jsonBytes[:]), err
// }

// func parsePayload(jsonString string) (p Payload, e error) {
// 	var person Payload

// 	err := json.Unmarshal([]byte(jsonString), &person)
// 	if err != nil {
// 		return person, err
// 	}

// 	return person, nil
// }

func parsePayloadMsgPack(msgpak []byte) (p Payload, e error) {
	if len(msgpak) > 0 {
		var person Payload

		err := msgpack.Unmarshal(msgpak, &person)
		if err != nil {
			return person, err
		}

		return person, nil
	} else {
		println("Cannot read data. Connection is about to be closed")
		return Payload{
			Command: CtClose,
		}, nil
	}

}

func (person *Payload) toMsgPak() (p []byte, e error) {

	data, err := msgpack.Marshal(person)
	if err != nil {
		return nil, err
	}

	return data, nil
}
