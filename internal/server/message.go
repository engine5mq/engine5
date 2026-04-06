package server

const (
	MsgOpCreated    = "CREATED"
	MsgOpProcessing = "PROCESSING"
	MsgOpOk         = "OK"
)

type Message struct {
	content                 string
	contentBinary           []byte
	id                      string
	targetSubjectName       string
	targetInstanceGroupName string
	commandType             string
	ResponseOfMessageId     string
	// nil = legacy terminal, *false = streaming chunk, *true = final chunk
	completed *bool
}

func MessageFromPayload(pl Payload) Message {
	// todo: eğer id yoksa hata fırlat
	return Message{
		content:                 pl.Content,
		contentBinary:           pl.ContentBinary,
		id:                      pl.MessageId,
		targetSubjectName:       pl.Subject,
		targetInstanceGroupName: pl.InstanceGroup,
		commandType:             pl.Command,
		ResponseOfMessageId:     pl.ResponseOfMessageId,
		completed:               pl.Completed,
	}
}
