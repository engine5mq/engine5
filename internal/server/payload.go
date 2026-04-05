package server

import (
	"encoding/binary"

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
	CtResponse                = "RESPONSE"
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

// Payload, TCP üzerinden iletilirken MessagePack ile kodlanır ve 4 byte big-endian length-prefix ile çerçevelenir.
type Payload struct {
	Command string `msgpack:"command"`
	// JSON / text içerik
	Content string `msgpack:"content"`
	// Binary içerik (image, pdf, vb.) — base64 dönüşümü gerektirmez
	ContentBinary       []byte `msgpack:"contentBinary,omitempty"`
	Subject             string `msgpack:"subject"`
	InstanceId          string `msgpack:"instanceId"`
	MessageId           string `msgpack:"messageId"`
	ResponseOfMessageId string `msgpack:"responseOfMessageId"`
	ResponseErrorSide   string `msgpack:"responseErrorSide,omitempty"`
	AuthKey             string `msgpack:"authKey,omitempty"`
	Completed           bool   `msgpack:"completed,omitempty"`
	InstanceGroup       string `msgpack:"instance_group,omitempty"`
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

	// Length-prefixed protocol: 4 byte uzunluk bilgisi + msgpack verisi
	lengthPrefix := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, uint32(len(data)))

	return append(lengthPrefix, data...), nil
}
