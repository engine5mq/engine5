package common

// Bu dosya, .env üzerinden okunan tüm ortam değişkeni (environment variable)
// isimlerini tek bir yerde toplar. Amaç, kod içinde dağınık şekilde geçen
// hardcoded string'leri önleyip hangi ayarın hangi env key'ine karşılık
// geldiğini kolayca takip edebilmektir.
const (
	// Genel (General) ayarlar
	EnvHost           = "E5_HOST"
	EnvEventPort      = "E5_EVENT_PORT"
	EnvExhaustivePort = "E5_EXHAUSTIVE_PORT"
	EnvInstanceID     = "E5_INSTANCE_ID"
	EnvInstanceGroup  = "E5_INSTANCE_GROUP"
	EnvActiveRulePath = "E5_ACTIVE_RULE_PATH"

	// Client ayarları
	EnvPort      = "E5_PORT"
	EnvSubject   = "E5_SUBJECT"
	EnvClientTLS = "E5_TLS"

	// Server ayarları
	EnvEnableTLS         = "ENABLE_TLS"
	EnvMaxConnections    = "MAX_CONNECTIONS"
	EnvConnectionTimeout = "CONNECTION_TIMEOUT"

	// Server TLS ayarları
	EnvTLSCertFile          = "TLS_CERT_FILE"
	EnvTLSKeyFile           = "TLS_KEY_FILE"
	EnvTLSCAFile            = "TLS_CA_FILE"
	EnvTLSRequireClientAuth = "TLS_REQUIRE_CLIENT_AUTH"
	EnvTLSServerName        = "TLS_SERVER_NAME"

	// Server Auth ayarları
	EnvAuthSecret        = "AUTH_SECRET"
	EnvRequireAuth       = "REQUIRE_AUTH"
	EnvClientPermissions = "CLIENT_PERMISSIONS"

	// Exhaust (server tarafı) ayarları
	EnvExhaustEnv            = "E5_ENV"
	EnvExhaustLogLevel       = "E5_LOG_LEVEL"
	EnvExhaustLogFormat      = "E5_LOG_FORMAT"
	EnvExhaustIncludeContent = "E5_EXHAUST_INCLUDE_CONTENT"
	EnvExhaustEnable         = "E5_EXHAUST_ENABLE"
	EnvExhaustPort           = "E5_EXHAUST_PORT"
	EnvExhaustKey            = "E5_EXHAUST_KEY"
	EnvExhaustTLS            = "E5_EXHAUST_TLS"

	// Tap (exhaust client tarafı) ayarları
	EnvExhaustHost      = "E5_EXHAUST_HOST"
	EnvExhaustCAFile    = "E5_EXHAUST_CA_FILE"
	EnvExhaustReconnect = "E5_EXHAUST_RECONNECT"
)
