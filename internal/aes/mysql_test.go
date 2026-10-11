package aes

import (
	"context"
	"engine5/internal/e5dbase"
	"os"
	"strings"
	"testing"
	"time"
)

// Use only a disposable, empty database: this test creates and deletes AES records.
func TestMySQLDeliveryLifecycle(t *testing.T) {
	dsn := os.Getenv("E5_AES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("E5_AES_TEST_MYSQL_DSN not set")
	}
	manager, err := e5dbase.NewDatabaseWithMysqlDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if err := manager.GetDB().Ping(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(manager)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Init(ctx); err != nil {
		t.Fatal("schema init not idempotent:", err)
	}
	rules, err := ReadRules(strings.NewReader(`{"rules":[{"event_subject_key":"order.created",
		"try_after":1,"try_count":2,"remove_successful_timeout":1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, store: store, publisher: &testPublisher{}, now: time.Now().UTC()}
	f.processor = NewProcessor(store, rules, f.publisher)
	f.event("EVENT_RECEIVED", "mysql-original", "")
	f.event("EVENT_DELIVERED", "mysql-original", "orders")
	f.event("CONSUMING_ERROR", "mysql-original", "orders")
	f.tick(1)
	if len(f.publisher.messages) != 1 {
		t.Fatal("MySQL retry not scheduled")
	}
	f.event("CONSUMING_SUCCESS", f.publisher.messages[0].MessageID, "orders")
	if d := f.delivery("mysql-original", "orders"); d == nil || d.State != StateSucceeded || d.RetryCount != 1 {
		t.Fatalf("MySQL delivery state incorrect: %+v", d)
	}
	f.tick(1)
	if f.delivery("mysql-original", "orders") != nil || f.count("aes_event_tap") != 0 || f.count("aes_attempt") != 0 {
		t.Fatal("MySQL cleanup failed")
	}
}
