package aes

import (
	"context"
	"database/sql"
	"engine5/internal/aes/entity"
	"engine5/internal/e5dbase"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testPublisher struct {
	messages  []RetryMessage
	err       error
	onPublish func(RetryMessage)
}

func (p *testPublisher) Publish(_ context.Context, message RetryMessage) error {
	p.messages = append(p.messages, message)
	if p.onPublish != nil {
		p.onPublish(message)
	}
	return p.err
}

type fixture struct {
	t         *testing.T
	store     *Store
	processor *Processor
	publisher *testPublisher
	now       time.Time
	manager   *e5dbase.DatabaseManager
	path      string
}

func newFixture(t *testing.T, fields string) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aes.db")
	manager, err := e5dbase.NewDatabaseWithSqlitePath(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close() })
	manager.GetDB().SetMaxOpenConns(1)
	store := NewStore(manager)
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	rules, err := ReadRules(strings.NewReader(`{"rules":[{"event_subject_key":"order.created"` + fields + `}]}`))
	if err != nil {
		t.Fatal(err)
	}
	pub := &testPublisher{}
	return &fixture{t: t, store: store, processor: NewProcessor(store, rules, pub),
		publisher: pub, manager: manager, path: path, now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (f *fixture) event(kind, id, group string) {
	f.t.Helper()
	err := f.processor.Handle(context.Background(), TapEvent{
		Kind: kind, MessageId: id, Subject: "order.created", Group: group,
		Instance: group + "-1", Content: `{"order":42}`, Msg: "test", Level: "INFO",
	}, f.now)
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) tick(seconds int) {
	f.t.Helper()
	f.now = f.now.Add(time.Duration(seconds) * time.Second)
	if err := f.processor.Tick(context.Background(), f.now); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) delivery(id, group string) *entity.AesDelivery {
	f.t.Helper()
	tx, err := f.store.db.Begin()
	if err != nil {
		f.t.Fatal(err)
	}
	defer tx.Rollback()
	d, err := f.store.load(context.Background(), tx, recordKey(id, "order.created", group))
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}

func (f *fixture) count(table string) int {
	f.t.Helper()
	var count int
	if err := f.store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		f.t.Fatal(err)
	}
	return count
}

func TestRetryLimitAndFailedCleanup(t *testing.T) {
	f := newFixture(t, `,"try_after":10,"try_count":2,"remove_failed_timeout":5`)
	f.event("EVENT_DELIVERED", "original", "orders")
	f.event("CONSUMING_ERROR", "original", "orders")
	f.event("CONSUMING_ERROR", "original", "orders")
	f.tick(9)
	if len(f.publisher.messages) != 0 {
		t.Fatal("retry before try_after")
	}
	f.tick(1)
	first := f.publisher.messages[0]
	if first.Content != `{"order":42}` || first.Group != "orders" || first.OriginalMessageID != "original" ||
		first.MessageID == "original" {
		t.Fatalf("incorrect retry payload: %+v", first)
	}
	f.event("CONSUMING_ERROR", first.MessageID, "orders")
	f.tick(10)
	second := f.publisher.messages[1]
	f.tick(100)
	d := f.delivery("original", "orders")
	if d == nil || d.State != StateAwaiting || d.RetryCount != 2 {
		t.Fatalf("last retry removed before consumption result: %+v", d)
	}
	f.event("CONSUMING_ERROR", first.MessageID, "orders")
	if f.delivery("original", "orders").State != StateAwaiting {
		t.Fatal("stale failure changed current attempt")
	}
	f.event("CONSUMING_ERROR", second.MessageID, "orders")
	f.tick(4)
	if d := f.delivery("original", "orders"); d == nil || d.State != StateExhausted {
		t.Fatalf("expected retained exhausted record: %+v", d)
	}
	f.tick(1)
	if f.delivery("original", "orders") != nil || f.count("aes_attempt") != 0 || f.count("aes_event_tap") != 0 {
		t.Fatal("failed retention did not clean related records")
	}
	if len(f.publisher.messages) != 2 {
		t.Fatal("retry limit exceeded")
	}
}

func TestEarlySuccessAndGroupIsolation(t *testing.T) {
	f := newFixture(t, `,"try_after":5,"try_count":5,"remove_successful_timeout":10`)
	f.event("EVENT_RECEIVED", "original", "")
	f.event("EVENT_DELIVERED", "original", "orders")
	f.event("EVENT_DELIVERED", "original", "billing")
	f.event("CONSUMING_SUCCESS", "original", "billing")
	f.event("CONSUMING_ERROR", "original", "orders")
	f.tick(5)
	retry := f.publisher.messages[0]
	f.event("CONSUMING_SUCCESS", retry.MessageID, "orders")
	f.event("CONSUMING_ERROR", retry.MessageID, "orders")
	if d := f.delivery("original", "orders"); d.State != StateSucceeded || d.RetryCount != 1 {
		t.Fatalf("success not terminal: %+v", d)
	}
	f.tick(5)
	if f.delivery("original", "billing") != nil || f.delivery("original", "orders") == nil {
		t.Fatal("group retention was not independent")
	}
	var originalLogs int
	if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM aes_event_tap WHERE message_id = 'original'`).Scan(&originalLogs); err != nil {
		t.Fatal(err)
	}
	if originalLogs == 0 {
		t.Fatal("shared original logs deleted too early")
	}
	f.tick(5)
	if f.count("aes_delivery") != 0 || f.count("aes_event_tap") != 0 || len(f.publisher.messages) != 1 {
		t.Fatal("successful cleanup or retry cancellation failed")
	}
}

func TestUnlimitedRetryAndNoRemovalWhilePending(t *testing.T) {
	f := newFixture(t, `,"try_after":1,"try_count":0,"remove_failed_timeout":1`)
	f.event("EVENT_DELIVERED", "original", "orders")
	id := "original"
	for i := 0; i < 10; i++ {
		f.event("CONSUMING_ERROR", id, "orders")
		f.tick(1)
		id = f.publisher.messages[i].MessageID
	}
	d := f.delivery("original", "orders")
	if d == nil || d.RetryCount != 10 || d.State != StateAwaiting {
		t.Fatalf("unlimited retry stopped: %+v", d)
	}
}

func TestTimeoutActions(t *testing.T) {
	for _, action := range []string{"keep", "try", "remove"} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t, `,"try_after":3,"consumption_timeout":5,"on_consumption_timeout":"`+action+`"`)
			f.event("EVENT_DELIVERED", "original", "orders")
			f.tick(4)
			if f.delivery("original", "orders").State != StateAwaiting {
				t.Fatal("timeout fired early")
			}
			f.tick(1)
			d := f.delivery("original", "orders")
			switch action {
			case "keep":
				if d == nil || d.State != StateKept || !d.NextActionAt.IsZero() {
					t.Fatalf("keep failed: %+v", d)
				}
				f.event("CONSUMING_SUCCESS", "original", "orders")
				if f.delivery("original", "orders").State != StateSucceeded {
					t.Fatal("late success not applied")
				}
			case "try":
				if d == nil || d.State != StatePending {
					t.Fatalf("try failed: %+v", d)
				}
				f.tick(3)
				if len(f.publisher.messages) != 1 {
					t.Fatal("timeout did not schedule retry")
				}
			case "remove":
				if d != nil || f.count("aes_attempt") != 0 || f.count("aes_event_tap") != 0 {
					t.Fatal("remove failed")
				}
			}
		})
	}
}

func TestDisabledRetryAndZeroRetention(t *testing.T) {
	f := newFixture(t, `,"remove_successful_timeout":0`)
	f.event("EVENT_DELIVERED", "success", "orders")
	f.event("CONSUMING_SUCCESS", "success", "orders")
	f.event("EVENT_DELIVERED", "failure", "orders")
	f.event("CONSUMING_ERROR", "failure", "orders")
	f.tick(7200)
	if f.delivery("success", "orders") == nil || f.delivery("failure", "orders") == nil ||
		len(f.publisher.messages) != 0 {
		t.Fatal("zero retry/retention semantics lost")
	}
}

func TestPublisherFailureSemantics(t *testing.T) {
	for _, publishErr := range []error{ErrNoTarget, ErrRetryRejected, errors.New("connection lost after send")} {
		t.Run(publishErr.Error(), func(t *testing.T) {
			f := newFixture(t, `,"try_after":2,"try_count":1`)
			f.publisher.err = publishErr
			f.event("EVENT_DELIVERED", "original", "orders")
			f.event("CONSUMING_ERROR", "original", "orders")
			f.tick(2)
			d := f.delivery("original", "orders")
			if publishErr == ErrNoTarget || publishErr == ErrRetryRejected {
				if d.RetryCount != 0 || d.State != StatePending {
					t.Fatalf("definite rejection consumed retry limit: %+v", d)
				}
			} else if d.RetryCount != 1 || d.State != StateAwaiting {
				t.Fatalf("ambiguous failure retried unsafely: %+v", d)
			}
		})
	}
}

func TestRestartPreservesPolicyAndDeadline(t *testing.T) {
	f := newFixture(t, `,"try_after":5,"try_count":2`)
	f.event("EVENT_DELIVERED", "original", "orders")
	f.event("CONSUMING_ERROR", "original", "orders")
	if err := f.manager.Close(); err != nil {
		t.Fatal(err)
	}
	manager, err := e5dbase.NewDatabaseWithSqlitePath(f.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close() })
	manager.GetDB().SetMaxOpenConns(1)
	f.manager = manager
	f.store = NewStore(manager)
	if err := f.store.CheckVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.processor = NewProcessor(f.store, &Rules{}, f.publisher)
	f.tick(5)
	if len(f.publisher.messages) != 1 || f.delivery("original", "orders").RetryCount != 1 {
		t.Fatal("restart reset persisted policy/state")
	}
}

func TestOrphanRetryDeliveryDoesNotStartNewRetryChain(t *testing.T) {
	f := newFixture(t, `,"try_after":1`)
	if err := f.processor.Handle(context.Background(), TapEvent{Kind: "EVENT_DELIVERED", MessageId: "orphan-attempt",
		OriginalMessageId: "removed-original", Subject: "order.created", Group: "orders", Instance: "orders-1",
		Content: "body", Level: "INFO"}, f.now); err != nil {
		t.Fatal(err)
	}
	if f.count("aes_delivery") != 0 || f.count("aes_event_tap") != 1 {
		t.Fatal("orphan retry started a new delivery")
	}
}

func TestEmptyBodyIsRetriedWithoutSubstitution(t *testing.T) {
	f := newFixture(t, `,"try_after":1`)
	if err := f.processor.Handle(context.Background(), TapEvent{Kind: "EVENT_DELIVERED", MessageId: "empty",
		Subject: "order.created", Group: "orders", Level: "INFO"}, f.now); err != nil {
		t.Fatal(err)
	}
	f.event("CONSUMING_ERROR", "empty", "orders")
	f.tick(1)
	if len(f.publisher.messages) != 1 || f.publisher.messages[0].Content != "" {
		t.Fatal("empty body changed on retry")
	}
}

func TestResultDuringPublishDoesNotGetOverwritten(t *testing.T) {
	f := newFixture(t, `,"try_after":1`)
	f.event("EVENT_DELIVERED", "original", "orders")
	f.event("CONSUMING_ERROR", "original", "orders")
	f.publisher.onPublish = func(message RetryMessage) {
		f.event("CONSUMING_SUCCESS", message.MessageID, "orders")
	}
	f.tick(1)
	if f.delivery("original", "orders").State != StateSucceeded {
		t.Fatal("concurrent result overwritten")
	}
}

func TestSchemaVersion(t *testing.T) {
	f := newFixture(t, "")
	if err := f.store.Init(context.Background()); err != nil {
		t.Fatal("schema init not idempotent:", err)
	}
	if f.count("aes_schema_version") != 1 {
		t.Fatal("expected one version record")
	}
	if _, err := f.store.db.Exec(`UPDATE aes_schema_version SET version = 2`); err != nil {
		t.Fatal(err)
	}
	if err := f.store.CheckVersion(context.Background()); err == nil {
		t.Fatal("unsupported schema version accepted")
	}
	if err := f.store.Init(context.Background()); err == nil {
		t.Fatal("schema init accepted unsupported version")
	}
}

func TestCleanupPreservesUnmatchedGroups(t *testing.T) {
	f := newFixture(t, `,"instance_group":"^orders$","remove_successful_timeout":1`)
	f.event("EVENT_DELIVERED", "original", "orders")
	f.event("EVENT_DELIVERED", "original", "unmanaged")
	f.event("CONSUMING_SUCCESS", "original", "orders")
	f.tick(1)
	if f.count("aes_delivery") != 0 || f.count("aes_event_tap") != 1 {
		t.Fatal("cleanup removed unrelated unmatched group logs")
	}
}

func TestLastRetryTimeoutUsesFailedRetention(t *testing.T) {
	f := newFixture(t, `,"try_after":1,"try_count":1,"consumption_timeout":2,
		"on_consumption_timeout":"try","remove_failed_timeout":3`)
	f.event("EVENT_DELIVERED", "original", "orders")
	f.event("CONSUMING_ERROR", "original", "orders")
	f.tick(1)
	f.tick(2)
	if d := f.delivery("original", "orders"); d == nil || d.State != StateExhausted {
		t.Fatalf("last attempt timeout not exhausted: %+v", d)
	}
	f.tick(2)
	if f.delivery("original", "orders") == nil {
		t.Fatal("failed retention fired early")
	}
	f.tick(1)
	if f.delivery("original", "orders") != nil || len(f.publisher.messages) != 1 {
		t.Fatal("failed retention or retry limit incorrect after timeout")
	}
}
