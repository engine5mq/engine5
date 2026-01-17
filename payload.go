package main

import (
	"github.com/shamaton/msgpack"
)

/** DİKKAT: EĞER BU KODDA İNGLİZCE TERİMLERE TAKILIP PLAZA AĞIZI
MUHABBETİ YAPARSANIZ TUVALET TERLİĞİNİZE SU DÖKERİM! */

const (
	//ilk bağlandığında "connect" ile TCP bağlantısından emin olunur.
	CtConnect = "CONNECT"
	//"CONNECT" işlemi başarılı olursa "instanceId" ile "CONNECT_SUCCESS" dönecektir
	CtConnectSuccess = "CONNECT_SUCCESS"
	CtConnectError   = "CONNECT_ERROR"
	/* Client MQS'ye
	`Content` ve `Subject` ile "EVENT" gönderir.
	Bu, bütün Subject'i dinleyen Client'lara MessageId ile gönderilir */
	CtEvent = "EVENT"
	/* Event MQS tarafından alındıysa, Eventi üreten Client'a Subject ve `MessageId` ile "RECIEVED" gönderilecektir*/
	CtRecieved = "RECIEVED"
	// Eventi alan client Received dönecektir
	CtApplied = "APPLIED"
	/* Client eğer istek atacaksa Subject, Content, MessageId, InstanceId(eğer spesifik bir client'a gönderilecekse)
	ile "REQUEST" gönderir. Aynı payload sağlanan bilgilerle sadece bir tane ilgili client'a gönderecektir*/
	CtRequest = "REQUEST"
	// Client "content" ve "id" ile istek gönderir
	CtResponse = "RESPONSE"
	// Herhangi bir "event" ya da "request" dinleneceği zaman `Subject` ile "LISTEN" gönderilir
	CtResponseError           = "RESPONSE_ERROR"
	CtResponseErrorSideE5     = "E5"
	CtResponseErrorSideClient = "CLIENT"
	// Herhangi bir "event" ya da "request" dinleneceği zaman `Subject` ile "LISTEN" gönderilir
	CtListen = "LISTEN"
	// Client kapanacağı anda "CLOSE" gönderir
	CtClose = "CLOSE"
	// Herhangi bir hata durumunda çift durumda belli koşullarla
	CtError = "ERROR"
	// Authentication commands
	CtAuth         = "AUTH"
	CtAuthSuccess  = "AUTH_SUCCESS"
	CtAuthError    = "AUTH_ERROR"
	CtUnauthorized = "UNAUTHORIZED"
)

/*
*

	Tcp iletişimi sağlandığında bu payload kullanılacaktır.
	Payload, gönderilmeden önce MessagePack ile byte dizisi haline getirilir, ve ayırımı kolay olması açısından sonuna 0x04

	DİKKAT: SAYI TİPİ(INTEGER) YERİNE STRİNG KULLANIN. 0x04 BYTE İLE AYRILIYOR ANCAK PAYLOADIN İÇİNDE 4 SAYISI OLMASI (INTEGER - FIXINT) YANLIŞ KESİLMESİNE NEDEN OLUYOR
	USE STRING INSTEAD OF NUMBER TYPE. IT IS SEPARATED BY '4' BUT THE PRESENCE OF THE NUMBER 4 (INTEGER - FIXINT) IN THE PAYLOAD CAUSES IT TO BE TRUNCTURED WRONGLY
*/
type Payload struct {
	Command string `json:"command"`
	// Event, Request, Response, Connecton Error
	Content string `json:"content"`
	// Event, Request, Response, Connecton Error
	Subject string `json:"subject"`
	// Connect
	InstanceId string `json:"instanceId"`
	// Event, Request
	MessageId string `json:"messageId"`
	// Response
	ResponseOfMessageId string `json:"responseOfMessageId"`
	ResponseErrorSide   string

	// Not using yet
	Completed     bool   `json:"completed"`
	InstanceGroup string `json:"instance_group"`
}

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
