package server

const (
	MsgOpCreated    = "CREATED"
	MsgOpProcessing = "PROCESSING"
	MsgOpOk         = "OK"
)

type Message struct {
	content                 string
	id                      string
	targetSubjectName       string
	targetInstanceGroupName string
	commandType             string
	ResponseOfMessageId     string
}

func MessageFromPayload(pl Payload) Message {
	// todo: eğer id yoksa hata fırlat
	return Message{
		content:                 pl.Content,
		id:                      pl.MessageId,
		targetSubjectName:       pl.Subject,
		targetInstanceGroupName: pl.InstanceGroup,
		commandType:             pl.Command,
		ResponseOfMessageId:     pl.ResponseOfMessageId,
	}
}
