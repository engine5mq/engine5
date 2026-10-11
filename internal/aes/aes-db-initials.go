package aes

import (
	"context"
	"engine5/internal/common"
	"engine5/internal/e5dbase"
	"fmt"
)

func validateDBDriver(dbDriver string) error {
	switch dbDriver {
	case "mysql":
		return nil
	case "":
		return fmt.Errorf("database driver is not specified in the configuration")
	default:
		return fmt.Errorf("unsupported AES database driver %q (schema v1 requires mysql)", dbDriver)
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
	if err := dbManager.GetDB().Ping(); err != nil {
		dbManager.Close()
		return nil, fmt.Errorf("connect AES database: %w", err)
	}
	store := NewStore(dbManager)
	if config.DBGenerateIfNotExist {
		if err := store.Init(context.Background()); err != nil {
			dbManager.Close()
			return nil, err
		}
	} else if err := store.CheckVersion(context.Background()); err != nil {
		dbManager.Close()
		return nil, err
	}

	return dbManager, nil
}
