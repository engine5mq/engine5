package aes

import (
	"engine5/internal/aes/entity"
	"engine5/internal/common"
	"engine5/internal/database"
)

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
		err = dbManager.CreateTable(entity.AesEventTapTableDefinition())
		if err != nil {
			return nil, err
		}
	}
	return dbManager, err
}
