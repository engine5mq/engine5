package database

import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

type KeyValuePair struct {
	Key      string
	Value    interface{}
	Operator string // Opsiyonel: "=", "<", ">", "<=", ">=", "<>", "!=" gibi operatörler için kullanılabilir. Varsayılan olarak "=" kabul edilir.
}

// Structtan where clause oluşturmak için bir yardımcı fonksiyon
func StructToWhereClause(keyValuePairs []KeyValuePair) (string, []interface{}) {
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
		whereClause += kv.Key + " " + operator + " ?"
		args = append(args, kv.Value)
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
