package aes

import (
	"engine5/internal/common"
	"engine5/internal/database"
	"log/slog"
	"time"
)

type AesEventTap struct {
	Id        string     `json:"id" db:"id" type:"varchar" size:"36" primary_key:"true" safe_func:"UUID"`
	Time      time.Time  `json:"time" db:"time" type:"timestamp" not_null:"true" safe_func:"CURRENT_TIMESTAMP"`
	Level     slog.Level `json:"level" db:"level" type:"int" not_null:"true"`
	Kind      string     `json:"kind" db:"kind" type:"varchar" size:"64" not_null:"true"`
	Instance  string     `json:"instance,omitempty" db:"instance" type:"varchar" size:"128"`
	Group     string     `json:"group,omitempty" db:"instance_group" type:"varchar" size:"128"`
	Subject   string     `json:"subject,omitempty" db:"subject" type:"varchar" size:"255"`
	MessageId string     `json:"messageId,omitempty" db:"message_id" type:"varchar" size:"128"`
	Remote    string     `json:"remote,omitempty" db:"remote" type:"varchar" size:"255"`
	// Content hassas veri içerebilir; varsayılan olarak maskelenir.
	// Yalnızca E5_EXHAUST_INCLUDE_CONTENT=true iken doldurulur.
	Content string `json:"content,omitempty" db:"content" type:"longtext"`
	Err     string `json:"err,omitempty" db:"err" type:"mediumtext"`
	// Msg, olaya eşlik eden okunabilir kısa açıklamadır.
	Msg string `json:"msg,omitempty" db:"msg" type:"mediumtext"`
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
	dbManager.SetShowQueries(config.DBShowQueries)
	if config.DBGenerateIfNotExist {
		// Assuming you have a struct representing your table, e.g., `MyTableStruct`
		err = dbManager.CreateTableFromStruct(AesEventTap{})
		if err != nil {
			return nil, err
		}
	}
	return dbManager, err
}
