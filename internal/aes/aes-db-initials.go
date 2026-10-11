package aes

import (
	"engine5/internal/aes/entity"
	"engine5/internal/common"
	"engine5/internal/e5dbase"
	"fmt"
)

func validateDBDriver(dbDriver string) error {
	switch dbDriver {
	case "mysql", "postgres":
		return nil
	case "":
		return fmt.Errorf("database driver is not specified in the configuration")
	default:
		return fmt.Errorf("unsupported database driver %q", dbDriver)
	}
}

func ConnectToDatabase() (*e5dbase.DatabaseManager, error) {
	config := common.GetAesConnectionConfig()
	if err := validateDBDriver(config.DBDriver); err != nil {
		return nil, err
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
		if err := dbManager.CreateTable(entity.AesEventTapTableDefinition()); err != nil {
			return nil, err
		}
	}

	return dbManager, nil
}
