package entity

import (
	"engine5/internal/database"
	"time"
)

type AesEventTap struct {
	Id        string    `json:"id"`
	Time      time.Time `json:"time"`
	Level     string    `json:"level"`
	Kind      string    `json:"kind"`
	Instance  string    `json:"instance,omitempty"`
	Group     string    `json:"group,omitempty"`
	Subject   string    `json:"subject,omitempty"`
	MessageId string    `json:"messageId,omitempty"`
	Remote    string    `json:"remote,omitempty"`
	// Content hassas veri içerebilir; varsayılan olarak maskelenir.
	// Yalnızca E5_EXHAUST_INCLUDE_CONTENT=true iken doldurulur.
	Content string `json:"content,omitempty"`
	Err     string `json:"err,omitempty"`
	// Msg, olaya eşlik eden okunabilir kısa açıklamadır.
	Msg string `json:"msg,omitempty"`
}

func (AesEventTap) TableDefinition() database.TableDefinition {
	return AesEventTapTableDefinition()
}

func (aet AesEventTap) KeyValuePairs() []database.KeyValuePair {
	return []database.KeyValuePair{
		{
			Key:           "id",
			ValueSafeFunc: "DEFAULT_VALUE",
			Value:         aet.Id,
		},
		{Key: "time", Value: aet.Time},
		{Key: "level", Value: aet.Level},
		{Key: "kind", Value: aet.Kind},
		{Key: "instance", Value: aet.Instance},
		{Key: "instance_group", Value: aet.Group},
		{Key: "subject", Value: aet.Subject},
		{Key: "message_id", Value: aet.MessageId},
		{Key: "remote", Value: aet.Remote},
		{Key: "content", Value: aet.Content},
		{Key: "err", Value: aet.Err},
		{Key: "msg", Value: aet.Msg},
	}
}

func AesEventTapTableName() string {
	return "aes_event_tap"
}

func AesEventTapTableDefinition() database.TableDefinition {
	return database.TableDefinition{
		Name: AesEventTapTableName(),
		Columns: []database.ColumnDefinition{
			{
				Name:         "id",
				Type:         "varchar",
				IsPrimaryKey: true,
				Length:       36,
				SafeFunc:     "UUID",
			},
			{
				Name:      "time",
				Type:      "timestamp",
				IsNotNull: true,
				SafeFunc:  "CURRENT_TIMESTAMP",
			},
			{
				Name:      "level",
				Type:      "varchar",
				Length:    32,
				IsNotNull: true,
			},
			{
				Name:      "kind",
				Type:      "varchar",
				IsNotNull: true,
				Length:    64,
			},
			{
				Name:   "instance",
				Type:   "varchar",
				Length: 128,
			},
			{
				Name:   "instance_group",
				Type:   "varchar",
				Length: 128,
			},
			{
				Name:   "subject",
				Type:   "varchar",
				Length: 255,
			},
			{
				Name:   "message_id",
				Type:   "varchar",
				Length: 128,
			},
			{
				Name:   "remote",
				Type:   "varchar",
				Length: 255,
			},
			{
				Name: "content",
				Type: "longtext",
			},
			{
				Name: "err",
				Type: "mediumtext",
			},
			{
				Name: "msg",
				Type: "mediumtext",
			},
		},
	}
}
