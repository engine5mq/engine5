package aes

import (
	"engine5/internal/common"
	"engine5/internal/database"
	"log/slog"
	"time"
)

type AesEventTap struct {
	Id        string     `json:"id" db:"id" type:"varchar(36)" primary_key:"true" safe_func:"UUID"`
	Time      time.Time  `json:"time" db:"time" type:"timestamp"`
	Level     slog.Level `json:"level"`
	Kind      string     `json:"kind"`
	Instance  string     `json:"instance,omitempty"`
	Group     string     `json:"group,omitempty"`
	Subject   string     `json:"subject,omitempty"`
	MessageId string     `json:"messageId,omitempty"`
	Remote    string     `json:"remote,omitempty"`
	// Content hassas veri içerebilir; varsayılan olarak maskelenir.
	// Yalnızca E5_EXHAUST_INCLUDE_CONTENT=true iken doldurulur.
	Content string `json:"content,omitempty"`
	Err     string `json:"err,omitempty"`
	// Msg, olaya eşlik eden okunabilir kısa açıklamadır.
	Msg string `json:"msg,omitempty"`
}

func ConnectToDatabase() (*database.DatabaseManager, error) {
	config := common.GetAesConnectionConfig()
	if config.DBDriver == "" {
		panic("Database driver is not specified in the configuration.")
	}
	if config.DBDriver != "mysql" && config.DBDriver != "postgres" {
		panic("Unsupported database driver specified in the configuration.")
	}
	dbManager, err := database.NewDatabaseWithParameters(
		config.DBDriver,
		config.DBUser,
		config.DBPassword,
		config.DBHost,
		config.DBPort,
		config.DBName,
	)
	if err != nil {
		return nil, err
	}
	if config.DBGenerateIfNotExist {
		// Assuming you have a struct representing your table, e.g., `MyTableStruct`
		err = dbManager.CreateTableFromStruct(MyTableStruct{})
		if err != nil {
			return nil, err
		}
	}
	return dbManager, err
}
