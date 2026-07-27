package e5dbase

import (
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

func normalizeDriverName(dbname string) string {
	driver := strings.ToLower(strings.TrimSpace(dbname))
	switch driver {
	case "sqlite3":
		return "sqlite"
	default:
		return driver
	}
}

func resolveSafeFuncSQL(dbname, safeFuncName string) (string, bool) {
	if safeFuncName == "" {
		return "", false
	}

	funcName := strings.ToUpper(strings.TrimSpace(safeFuncName))
	driver := normalizeDriverName(dbname)

	safeFuncMap, exists := safeFuncs[funcName]
	if !exists {
		return "", false
	}

	safeFuncSQL, exists := safeFuncMap[driver]
	if !exists {
		return "", false
	}

	return safeFuncSQL, true
}

func mapGoTypeToSQLType(dbDriverName, rawType string) (string, bool) {
	goType := strings.ToLower(strings.TrimSpace(rawType))
	driver := normalizeDriverName(dbDriverName)

	switch goType {
	case "string":
		if driver == "sqlite" {
			return "TEXT", true
		}
		return "VARCHAR", true
	case "bool", "boolean":
		if driver == "sqlite" {
			return "INTEGER", true
		}
		return "BOOLEAN", true
	case "int", "int8", "int16", "int32", "uint", "uint8", "uint16", "uint32", "level":
		return "INTEGER", true
	case "int64", "uint64":
		return "BIGINT", true
	case "float32":
		return "FLOAT", true
	case "float64":
		return "DOUBLE", true
	case "time", "time.time":
		if driver == "sqlite" {
			return "TEXT", true
		}
		return "TIMESTAMP", true
	default:
		return "", false
	}
}

func resolveColumnType(dbDriverName, rawType string) string {
	typeName := strings.TrimSpace(rawType)
	if typeName == "" {
		if normalizeDriverName(dbDriverName) == "sqlite" {
			return "TEXT"
		}
		return "VARCHAR"
	}

	if mappedType, ok := mapGoTypeToSQLType(dbDriverName, typeName); ok {
		return mappedType
	}

	return typeName
}

func formatDefaultValue(defaultValue interface{}) string {
	defaultValueAsString := strings.ReplaceAll(fmt.Sprint(defaultValue), "'", "''")
	return fmt.Sprintf("'%s'", defaultValueAsString)
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
			if safeFuncSQL, exists := resolveSafeFuncSQL(dbname, kv.ValueSafeFunc); exists {
				whereClause += kv.Key + " " + operator + " " + safeFuncSQL
			} else {
				whereClause += kv.Key + " " + operator + " ?"
				args = append(args, kv.Value)
			}
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
		if kv.ValueSafeFunc == "DEFAULT_VALUE" && kv.Value == "" {
			continue // Skip this key-value pair for the SET clause
		}
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
		columnType := resolveColumnType(dbDriverName, col.Type)
		if col.Length > 0 && !strings.Contains(columnType, "(") {
			columnType = fmt.Sprintf("%s(%d)", columnType, col.Length)
		}
		columnDefs += fmt.Sprintf("%s %s", col.Name, columnType)
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
			if safeFuncSQL, exists := resolveSafeFuncSQL(dbDriverName, col.SafeFunc); exists {
				columnDefs += fmt.Sprintf(" DEFAULT %s", safeFuncSQL)
			}
		} else if col.DefaultValue != "" && col.DefaultValue != nil {
			columnDefs += fmt.Sprintf(" DEFAULT %s", formatDefaultValue(col.DefaultValue))
		}
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, columnDefs)
	return query
}
