package database

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

type ColumnDefinition struct {
	Name            string
	Type            string
	IsPrimaryKey    bool
	IsAutoIncrement bool
	IsUnique        bool
	IsNotNull       bool
	DefaultValue    interface{}
	Length          int
	SafeFunc        string // Opsiyonel: "UUID", "NOW", "CURRENT_TIMESTAMP" gibi güvenli fonksiyonlar için kullanılabilir.
}

type TableDefinition struct {
	Name    string
	Columns []ColumnDefinition
}

type KeyValuePair struct {
	Key           string
	Value         interface{}
	ValueSafeFunc string // Opsiyonel: "UUID", "NOW", "CURRENT_TIMESTAMP" gibi güvenli fonksiyonlar için kullanılabilir.
	Operator      string // Opsiyonel: "=", "<", ">", "<=", ">=", "<>", "!=" gibi operatörler için kullanılabilir. Varsayılan olarak "=" kabul edilir.
}

// Structtan where clause oluşturmak için bir yardımcı fonksiyon
func StructToWhereClause(dbname string, keyValuePairs []KeyValuePair) (string, []interface{}) {
	whereClause := ""
	args := []interface{}{}

	for _, kv := range keyValuePairs {
		if whereClause != "" {
			whereClause += " AND "
		}
		operator := kv.Operator
		if operator == "" {
			operator = "="
		}
		if kv.ValueSafeFunc != "" {
			whereClause += kv.Key + " " + operator + " " + safeFuncs[kv.ValueSafeFunc][dbname]
		} else {
			whereClause += kv.Key + " " + operator + " ?"
			args = append(args, kv.Value)
		}
		// whereClause += kv.Key + " " + operator + " ?"
		// args = append(args, kv.Value)
	}

	return whereClause, args
}

// Structtan set clause oluşturmak için bir yardımcı fonksiyon.
func StructToSetClause(keyValuePairs []KeyValuePair) (string, []interface{}) {
	setClause := ""
	args := []interface{}{}

	for _, kv := range keyValuePairs {
		if setClause != "" {
			setClause += ", "
		}
		setClause += kv.Key + " = ?"
		args = append(args, kv.Value)
	}

	return setClause, args
}

func TableCreationQueryFromDefinition(dbDriverName string, tableDefinition TableDefinition) string {
	tableName := tableDefinition.Name
	columns := tableDefinition.Columns

	columnDefs := ""
	for i, col := range columns {
		if i > 0 {
			columnDefs += ", "
		}
		columnDefs += fmt.Sprintf("%s %s", col.Name, col.Type)
		if col.Length > 0 {
			columnDefs += fmt.Sprintf("(%d)", col.Length)
		}
		if col.IsPrimaryKey {
			columnDefs += " PRIMARY KEY"
		}
		if col.IsAutoIncrement {
			columnDefs += " AUTOINCREMENT"
		}
		if col.IsNotNull {
			columnDefs += " NOT NULL"
		}
		if col.IsUnique {
			columnDefs += " UNIQUE"
		}
		if col.SafeFunc != "" {
			if safeFuncMap, exists := safeFuncs[col.SafeFunc]; exists {
				if safeFunc, exists := safeFuncMap[dbDriverName]; exists {
					columnDefs += fmt.Sprintf(" DEFAULT %s", safeFunc)
				}
			}
		} else if col.DefaultValue != "" && col.DefaultValue != nil {
			columnDefs += fmt.Sprintf(" DEFAULT '%s'", col.DefaultValue)
		}
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, columnDefs)
	return query
}
