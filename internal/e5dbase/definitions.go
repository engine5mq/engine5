package e5dbase

type ColumnDefinition struct {
	Name            string
	Type            string
	IsPrimaryKey    bool
	IsAutoIncrement bool
	IsUnique        bool
	IsNotNull       bool
	DefaultValue    interface{}
	Length          int
	SafeFunc        string                        // Opsiyonel: "UUID", "NOW", "CURRENT_TIMESTAMP" gibi güvenli fonksiyonlar için kullanılabilir.
	GetValueFunc    func(interface{}) interface{} // Opsiyonel: Kolon için özel bir değer alma fonksiyonu. Eğer belirtilirse, DefaultValue yerine bu fonksiyon kullanılabilir.
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
