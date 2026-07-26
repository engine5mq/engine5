package database

import (
	. "engine5/internal/common"
	"reflect"
)

const (
	TAGS_DB_KEY         = "db"
	TAGS_TYPE_KEY       = "type"
	TAGS_PRIMARY_KEY    = "primary_key"
	TAGS_AUTO_INCREMENT = "auto_increment"
	TAGS_UNIQUE         = "unique"
	TAGS_NOT_NULL       = "not_null"
	TAGS_DEFAULT        = "default"
	TAGS_SAFE_FUNC      = "safe_func"
	TAGS_SIZE_KEY       = "size"
	TAGS_IGNORE         = "db_ignore"
)

var tableDefinitionsFromStructs = map[string]TableDefinition{}

func CamelCaseToSnakeCase(str string) string {
	runes := []rune(str)
	length := len(runes)
	var result []rune

	for i, r := range runes {
		if i > 0 && isUpper(r) && (isLower(runes[i-1]) || (i+1 < length && isLower(runes[i+1]))) {
			result = append(result, '_')
		}
		result = append(result, toLower(r))
	}

	return string(result)
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isLower(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func toLower(r rune) rune {
	if isUpper(r) {
		return r + ('a' - 'A')
	}
	return r
}

// get struct table definition. if not in the map, create it and return it.
func GetTableDefinitionFromStruct(structType interface{}) TableDefinition {
	tableName := reflect.TypeOf(structType).Name()

	if tableDef, exists := tableDefinitionsFromStructs[tableName]; exists {
		return tableDef
	}

	// Eğer tablo tanımı yoksa, struct'tan tablo tanımını oluştur.
	tableDef := CreateTableDefinitionFromStruct(tableName, structType)
	tableDefinitionsFromStructs[tableName] = tableDef
	return tableDef
}

func CreateTableDefinitionFromStruct(tableName string, structType interface{}) TableDefinition {
	tableDefFromMap, tableDefExist := tableDefinitionsFromStructs[tableName]
	if tableDefExist {
		return tableDefFromMap
	}

	tableDef := TableDefinition{
		Name:    tableName,
		Columns: []ColumnDefinition{},
	}

	val := reflect.ValueOf(structType)
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		isIgnored := BoolStringOrDefault(field.Tag.Get(TAGS_IGNORE), false)
		if isIgnored {
			continue
		}

		columnName := StringOrDefault(field.Tag.Get(TAGS_DB_KEY), CamelCaseToSnakeCase(field.Name)) // Eğer db tag yoksa, Go struct alan adı kullanılacak.
		columnType := StringOrDefault(field.Tag.Get(TAGS_TYPE_KEY), field.Type.Name())              // Basit tip adı. Daha karmaşık tipler için ek işleme gerekebilir.
		isPrimaryKey := BoolStringOrDefault(field.Tag.Get(TAGS_PRIMARY_KEY), false)
		isAutoIncrement := BoolStringOrDefault(field.Tag.Get(TAGS_AUTO_INCREMENT), false)
		isUnique := BoolStringOrDefault(field.Tag.Get(TAGS_UNIQUE), false)
		isNotNull := BoolStringOrDefault(field.Tag.Get(TAGS_NOT_NULL), false)
		safeFunc := field.Tag.Get(TAGS_SAFE_FUNC)
		defaultValue := field.Tag.Get(TAGS_DEFAULT)

		columnDef := ColumnDefinition{
			Name:            columnName,
			Type:            columnType, // Eğer type tag yoksa, Go tip adı kullanılacak.
			IsPrimaryKey:    isPrimaryKey,
			IsAutoIncrement: isAutoIncrement,
			IsUnique:        isUnique,
			IsNotNull:       isNotNull,
			SafeFunc:        safeFunc,
			// Diğer özellikler (IsPrimaryKey, IsAutoIncrement, vb.) için ek tag'ler kullanılabilir.
			DefaultValue: defaultValue,
			Length:       StringToIntOrDefault(field.Tag.Get(TAGS_SIZE_KEY), 0),
		}
		tableDef.Columns = append(tableDef.Columns, columnDef)
	}
	tableDefinitionsFromStructs[tableName] = tableDef
	return tableDef
}
