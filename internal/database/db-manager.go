package database

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

type DatabaseManager struct {
	db                      *sql.DB
	dbDriverName            string
	activeTransaction       *sql.Tx
	originalDatabaseManager *DatabaseManager
}

func (dm *DatabaseManager) GetDB() *sql.DB {
	return dm.db
}

func (dm *DatabaseManager) Execute(query string, args ...interface{}) (sql.Result, error) {
	if dm.activeTransaction != nil {
		return dm.activeTransaction.Exec(query, args...)
	}
	return dm.db.Exec(query, args...)
}

func (dm *DatabaseManager) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if dm.activeTransaction != nil {
		return dm.activeTransaction.Query(query, args...)
	}
	return dm.db.Query(query, args...)
}

/**
* Yeni bir transaction başlatır ve yeni bir DatabaseManager döndürür.
* Eğer zaten bir transaction içindeyse, nil ve nil döndürür.
 */
func (dm *DatabaseManager) BeginTransaction() (*DatabaseManager, error) {
	if dm.activeTransaction != nil {
		return nil, fmt.Errorf("Already in a transaction")
	}

	tx, err := dm.db.Begin()
	if err != nil {
		return nil, err
	}
	return &DatabaseManager{
		db:                      dm.db,
		activeTransaction:       tx,
		originalDatabaseManager: dm,
	}, nil
	// dm.activeTransaction = tx
	// return dm, nil
}

func (dm *DatabaseManager) CommitTransaction() (*DatabaseManager, error) {
	if dm.activeTransaction == nil {
		return nil, nil // Not in a transaction
	}

	err := dm.activeTransaction.Commit()
	dm.activeTransaction = nil
	if dm.originalDatabaseManager != nil {
		dm.originalDatabaseManager.activeTransaction = nil
	}
	return dm.originalDatabaseManager, err
}

func (dm *DatabaseManager) RollbackTransaction() (*DatabaseManager, error) {
	if dm.activeTransaction == nil {
		return nil, nil // Not in a transaction
	}

	err := dm.activeTransaction.Rollback()
	dm.activeTransaction = nil
	if dm.originalDatabaseManager != nil {
		dm.originalDatabaseManager.activeTransaction = nil
	}
	return dm.originalDatabaseManager, err
}

func (dm *DatabaseManager) Close() error {
	return dm.db.Close()
}

func (dm *DatabaseManager) IsInTransaction() bool {
	return dm.activeTransaction != nil
}

// Tablo ekleme metodu
func (dm *DatabaseManager) CreateTable(tableDefinition TableDefinition) error {
	query := TableCreationQueryFromDefinition(dm.dbDriverName, tableDefinition)
	_, err := dm.Execute(query)
	return err
}

func (dm *DatabaseManager) CreateTableFromStruct(structType interface{}) error {
	tableDefinition := GetTableDefinitionFromStruct(structType)
	return dm.CreateTable(tableDefinition)
}

// Crud metotlarını ekleyebilirsiniz. Örneğin:
func (dm *DatabaseManager) Insert(table string, keyValuePairs []KeyValuePair) (sql.Result, error) {
	setClause, args := StructToSetClause(keyValuePairs)
	query := fmt.Sprintf("INSERT INTO %s SET %s", table, setClause)
	return dm.Execute(query, args...)
}

func (dm *DatabaseManager) Update(table string, keyValuePairs []KeyValuePair, whereClauses []KeyValuePair) (sql.Result, error) {
	setClause, setArgs := StructToSetClause(keyValuePairs)
	whereClause, whereArgs := StructToWhereClause(dm.dbDriverName, whereClauses)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s", table, setClause, whereClause)
	args := append(setArgs, whereArgs...)
	return dm.Execute(query, args...)
}

func (dm *DatabaseManager) Delete(table string, whereClauses []KeyValuePair) (sql.Result, error) {
	whereClause, args := StructToWhereClause(dm.dbDriverName, whereClauses)
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", table, whereClause)
	return dm.Execute(query, args...)
}

func NewDatabaseWithSqlitePath(dbPath string) (*DatabaseManager, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	return &DatabaseManager{db: db, dbDriverName: "sqlite3"}, nil
}

func NewDatabaseWithParameters(dbDriver, dbUser, dbPassword, dbHost string, dbPort int, dbName string) (*DatabaseManager, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", dbUser, dbPassword, dbHost, dbPort, dbName)
	db, err := sql.Open(dbDriver, dsn)
	if err != nil {
		return nil, err
	}

	return &DatabaseManager{db: db, dbDriverName: dbDriver}, nil
}

func NewDatabaseWithMysqlDSN(dsn string) (*DatabaseManager, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	return &DatabaseManager{db: db, dbDriverName: "mysql"}, nil
}
