package aes

import (
	"engine5/internal/aes/entity"
	"engine5/internal/common"
	"engine5/internal/e5dbase"
)

func ConnectToDatabase() (*e5dbase.DatabaseManager, error) {
	config := common.GetAesConnectionConfig()
	if config.DBDriver == "" {
		panic("Database driver is not specified in the configuration.")
	}
	if config.DBDriver != "mysql" && config.DBDriver != "postgres" {
		panic("Unsupported database driver specified in the configuration.")
	}
	dbManager, err := e5dbase.NewDatabaseWithParameters(
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
		err = dbManager.CreateTable(entity.AesEventTapTableDefinition())
		if err != nil {
			return nil, err
		}
	}
	return dbManager, err
}
