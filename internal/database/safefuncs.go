package database

var safeFuncs = map[string]map[string]string{
	"UUID": {
		"mysql":    "UUID()",
		"postgres": "gen_random_uuid()",
		"sqlite":   "lower(hex(randomblob(16)))",
	},
	"NOW": {
		"mysql":    "NOW()",
		"postgres": "NOW()",
		"sqlite":   "datetime('now')",
	},
	"CURRENT_TIMESTAMP": {
		"mysql":    "CURRENT_TIMESTAMP",
		"postgres": "CURRENT_TIMESTAMP",
		"sqlite":   "CURRENT_TIMESTAMP",
	},
	"CURRENT_DATE": {
		"mysql":    "CURRENT_DATE",
		"postgres": "CURRENT_DATE",
		"sqlite":   "date('now')",
	},
	"CURRENT_TIME": {
		"mysql":    "CURRENT_TIME",
		"postgres": "CURRENT_TIME",
		"sqlite":   "time('now')",
	},
	"LOCALTIME": {
		"mysql":    "LOCALTIME",
		"postgres": "LOCALTIME",
		"sqlite":   "time('now')",
	},
	"LOCALTIMESTAMP": {
		"mysql":    "LOCALTIMESTAMP",
		"postgres": "LOCALTIMESTAMP",
		"sqlite":   "datetime('now')",
	},
	"TRUE": {
		"mysql":    "TRUE",
		"postgres": "TRUE",
		"sqlite":   "1",
	},
	"FALSE": {
		"mysql":    "FALSE",
		"postgres": "FALSE",
		"sqlite":   "0",
	},
	"NULL": {
		"mysql":    "NULL",
		"postgres": "NULL",
		"sqlite":   "NULL",
	},
}
