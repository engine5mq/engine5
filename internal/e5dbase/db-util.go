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

func hasUsableDefaultValue(defaultValue interface{}) bool {
	if defaultValue == nil {
		return false
	}

	if defaultAsString, ok := defaultValue.(string); ok {
		return strings.TrimSpace(defaultAsString) != ""
	}

	return true
}

func operatorOrDefault(operator string) string {
	if strings.TrimSpace(operator) == "" {
		return "="
	}
	return operator
}

func StructToWhereClause(dbDriverName string, keyValuePairs []KeyValuePair) (string, []interface{}) {
	var whereBuilder strings.Builder
	args := make([]interface{}, 0, len(keyValuePairs))

	for index, kv := range keyValuePairs {
		if index > 0 {
			whereBuilder.WriteString(" AND ")
		}

		operator := operatorOrDefault(kv.Operator)
		whereBuilder.WriteString(kv.Key)
		whereBuilder.WriteString(" ")
		whereBuilder.WriteString(operator)
		whereBuilder.WriteString(" ")

		if kv.ValueSafeFunc != "" {
			if safeFuncSQL, exists := resolveSafeFuncSQL(dbDriverName, kv.ValueSafeFunc); exists {
				whereBuilder.WriteString(safeFuncSQL)
				continue
			}
		}

		whereBuilder.WriteString("?")
		args = append(args, kv.Value)
	}

	return whereBuilder.String(), args
}

func StructToSetClause(keyValuePairs []KeyValuePair) (string, []interface{}) {
	var setBuilder strings.Builder
	args := make([]interface{}, 0, len(keyValuePairs))
	hasAnyAssignment := false

	for _, kv := range keyValuePairs {
		if kv.ValueSafeFunc == "DEFAULT_VALUE" && kv.Value == "" {
			continue
		}

		if hasAnyAssignment {
			setBuilder.WriteString(", ")
		}
		setBuilder.WriteString(kv.Key)
		setBuilder.WriteString(" = ?")
		args = append(args, kv.Value)
		hasAnyAssignment = true
	}

	return setBuilder.String(), args
}

func TableCreationQueryFromDefinition(dbDriverName string, tableDefinition TableDefinition) string {
	tableName := tableDefinition.Name
	columns := tableDefinition.Columns

	var columnDefsBuilder strings.Builder
	for i, col := range columns {
		if i > 0 {
			columnDefsBuilder.WriteString(", ")
		}

		columnType := resolveColumnType(dbDriverName, col.Type)
		if col.Length > 0 && !strings.Contains(columnType, "(") {
			columnType = fmt.Sprintf("%s(%d)", columnType, col.Length)
		}

		columnDefsBuilder.WriteString(fmt.Sprintf("%s %s", col.Name, columnType))
		if col.IsPrimaryKey {
			columnDefsBuilder.WriteString(" PRIMARY KEY")
		}
		if col.IsAutoIncrement {
			columnDefsBuilder.WriteString(" AUTOINCREMENT")
		}
		if col.IsNotNull {
			columnDefsBuilder.WriteString(" NOT NULL")
		}
		if col.IsUnique {
			columnDefsBuilder.WriteString(" UNIQUE")
		}

		if col.SafeFunc != "" {
			if safeFuncSQL, exists := resolveSafeFuncSQL(dbDriverName, col.SafeFunc); exists {
				columnDefsBuilder.WriteString(fmt.Sprintf(" DEFAULT %s", safeFuncSQL))
			}
		} else if hasUsableDefaultValue(col.DefaultValue) {
			columnDefsBuilder.WriteString(fmt.Sprintf(" DEFAULT %s", formatDefaultValue(col.DefaultValue)))
		}
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, columnDefsBuilder.String())
}
