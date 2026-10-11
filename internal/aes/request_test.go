package aes

import (
	"context"
	"engine5/internal/e5dbase"
	"strings"
	"sync"
	"testing"
	"time"
)

func setRequestRules(t *testing.T, f *fixture, requests string) {
	t.Helper()
	rules, err := ReadRules(strings.NewReader(`{"rules":[{"event_subject_key":"order.created",
		"remove_successful_timeout":1}],"request_rules":` + requests + `}`))
	if err != nil {
		t.Fatal(err)
	}
	f.processor = NewProcessor(f.store, rules, f.publisher)
}

func (f *fixture) requestEvent(kind, id, subject string) {
	f.t.Helper()
	if err := f.processor.Handle(context.Background(), TapEvent{
		Kind: kind, MessageId: id, Subject: subject, Content: "request-or-response-body", Level: "DEBUG",
	}, f.now); err != nil {
		f.t.Fatal(err)
	}
}

func TestRequestLogsDiscardedByDefault(t *testing.T) {
	f := newFixture(t, "")
	for _, kind := range []string{"REQUEST_RECEIVED", "REQUEST_ROUTED", "REQUEST_NO_TARGET", "RESPONSE_RECEIVED", "RESPONSE_DELIVERED"} {
		f.requestEvent(kind, "request", "postral/payment-channel/init")
	}
	f.requestEvent("RESPONSE_RECEIVED", "request", "")
	if f.count("aes_event_tap") != 0 || f.count("aes_request") != 0 {
		t.Fatal("default request path persisted a body or retention record")
	}
	f.event("EVENT_RECEIVED", "event", "")
	f.event("SERVER_START", "", "")
	if f.count("aes_event_tap") != 2 {
		t.Fatal("default discard affected non-request logs")
	}
	f.tick(3600)
	if len(f.publisher.messages) != 0 {
		t.Fatal("request logs triggered retry")
	}
}

func TestRequestRetentionCorrelatesResponsesAndDeletesAtDeadline(t *testing.T) {
	f := newFixture(t, "")
	setRequestRules(t, f, `[{"request_subject_key":{"regex":"^postral/"},"remove_timeout":30}]`)
	f.requestEvent("REQUEST_RECEIVED", "request", "postral/payment-channel/init")
	f.requestEvent("REQUEST_ROUTED", "request", "postral/payment-channel/init")
	f.now = f.now.Add(10 * time.Second)
	f.requestEvent("RESPONSE_RECEIVED", "request", "")
	f.requestEvent("RESPONSE_DELIVERED", "request", "")
	var count int
	if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM aes_event_tap WHERE subject = ?`,
		"postral/payment-channel/init").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 || f.count("aes_request") != 1 {
		t.Fatal("subjectless responses not correlated with request")
	}
	f.tick(19)
	if f.count("aes_event_tap") != 4 {
		t.Fatal("retained logs removed before first-request deadline")
	}
	f.tick(1)
	if f.count("aes_event_tap") != 0 || f.count("aes_request") != 0 || len(f.publisher.messages) != 0 {
		t.Fatal("deadline did not clean request lifecycle independently of event retry")
	}
	f.requestEvent("RESPONSE_RECEIVED", "request", "")
	f.requestEvent("RESPONSE_DELIVERED", "request", "postral/payment-channel/init")
	if f.count("aes_event_tap") != 0 || f.count("aes_request") != 0 {
		t.Fatal("late response reopened cleaned request")
	}
}

func TestUnmatchedAndOrphanResponsesNotRetained(t *testing.T) {
	f := newFixture(t, "")
	setRequestRules(t, f, `[{"request_subject_key":"important","remove_timeout":30}]`)
	f.requestEvent("REQUEST_RECEIVED", "unmatched", "ordinary")
	f.requestEvent("REQUEST_ROUTED", "unmatched", "ordinary")
	f.requestEvent("RESPONSE_RECEIVED", "unmatched", "")
	f.requestEvent("RESPONSE_DELIVERED", "orphan", "important")
	if f.count("aes_event_tap") != 0 || f.count("aes_request") != 0 {
		t.Fatal("unmatched/orphan response retained")
	}
}

func TestRequestRetentionWithoutResponse(t *testing.T) {
	for _, kind := range []string{"REQUEST_RECEIVED", "REQUEST_ROUTED", "REQUEST_NO_TARGET"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t, "")
			setRequestRules(t, f, `[{"request_subject_key":"important","remove_timeout":30}]`)
			f.requestEvent(kind, "request", "important")
			f.tick(30)
			if f.count("aes_event_tap") != 0 || f.count("aes_request") != 0 {
				t.Fatal("request without response not cleaned")
			}
		})
	}
}

func TestRequestZeroTimeoutKeepsLogs(t *testing.T) {
	f := newFixture(t, "")
	setRequestRules(t, f, `[{"request_subject_key":"important","remove_timeout":0}]`)
	f.requestEvent("REQUEST_RECEIVED", "request", "important")
	f.tick(86400)
	f.requestEvent("RESPONSE_RECEIVED", "request", "")
	if f.count("aes_event_tap") != 2 || f.count("aes_request") != 1 {
		t.Fatal("zero timeout did not retain request/response indefinitely")
	}
}

func TestRequestPolicyAndCorrelationSurviveRestart(t *testing.T) {
	f := newFixture(t, "")
	setRequestRules(t, f, `[{"request_subject_key":"important","remove_timeout":30}]`)
	f.requestEvent("REQUEST_RECEIVED", "request", "important")
	if err := f.manager.Close(); err != nil {
		t.Fatal(err)
	}
	manager, err := e5dbase.NewDatabaseWithSqlitePath(f.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { manager.Close() })
	manager.GetDB().SetMaxOpenConns(1)
	f.store = NewStore(manager)
	f.processor = NewProcessor(f.store, &Rules{}, f.publisher)
	f.now = f.now.Add(20 * time.Second)
	f.requestEvent("RESPONSE_RECEIVED", "request", "")
	f.requestEvent("RESPONSE_DELIVERED", "request", "important")
	if f.count("aes_event_tap") != 3 {
		t.Fatal("restart or rules change lost retained request policy")
	}
	f.tick(10)
	if f.count("aes_event_tap") != 0 || f.count("aes_request") != 0 {
		t.Fatal("restart reset request retention deadline")
	}
}

func TestLateResponseAtDeadlineCleansWithoutWaitingForTick(t *testing.T) {
	f := newFixture(t, "")
	setRequestRules(t, f, `[{"request_subject_key":"important","remove_timeout":30}]`)
	f.requestEvent("REQUEST_RECEIVED", "request", "important")
	f.now = f.now.Add(30 * time.Second)
	f.requestEvent("RESPONSE_RECEIVED", "request", "")
	if f.count("aes_event_tap") != 0 || f.count("aes_request") != 0 {
		t.Fatal("expired request saved a new response")
	}
}

func TestRequestAndEventCleanupDoNotDeleteEachOther(t *testing.T) {
	for _, requestFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "request-cleanup", false: "event-cleanup"}[requestFirst], func(t *testing.T) {
			f := newFixture(t, `,"remove_successful_timeout":0`)
			setRequestRules(t, f, `[{"request_subject_key":"order.created","remove_timeout":30}]`)
			f.event("EVENT_DELIVERED", "same-id", "")
			f.requestEvent("REQUEST_RECEIVED", "same-id", "order.created")
			if requestFirst {
				f.tick(30)
				if f.count("aes_request") != 0 || f.count("aes_delivery") != 1 || f.count("aes_event_tap") != 1 {
					t.Fatal("request cleanup deleted an event with the same messageId")
				}
			} else {
				f.event("CONSUMING_SUCCESS", "same-id", "")
				f.tick(1)
				if f.count("aes_delivery") != 0 || f.count("aes_request") != 1 || f.count("aes_event_tap") != 1 {
					t.Fatal("event cleanup deleted request logs")
				}
			}
		})
	}
}

func TestRequestRetentionValidationAndExplicitErrors(t *testing.T) {
	f := newFixture(t, "")
	setRequestRules(t, f, `[{"request_subject_key":"important"}]`)
	if err := f.processor.Handle(context.Background(), TapEvent{Kind: "REQUEST_RECEIVED", Subject: "important"}, f.now); err == nil {
		t.Fatal("missing correlation ID accepted")
	}
	f.requestEvent("REQUEST_RECEIVED", "request", "important")
	if err := f.processor.Handle(context.Background(), TapEvent{Kind: "RESPONSE_DELIVERED",
		MessageId: "request", Subject: "wrong"}, f.now); err == nil {
		t.Fatal("mismatched response subject accepted")
	}
	if f.count("aes_event_tap") != 1 {
		t.Fatal("invalid response body was persisted")
	}
}

func TestRequestCleanupPreservesLegacyLogs(t *testing.T) {
	f := newFixture(t, "")
	tx, err := f.store.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := insertTap(context.Background(), tx, TapEvent{Kind: "REQUEST_RECEIVED",
		MessageId: "legacy", Subject: "important", Time: f.now, Level: "DEBUG"}); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	f.tick(86400)
	if f.count("aes_event_tap") != 1 {
		t.Fatal("request default retroactively deleted legacy logs")
	}
}

func TestConcurrentRequestIngestionAndCleanup(t *testing.T) {
	f := newFixture(t, "")
	setRequestRules(t, f, `[{"request_subject_key":"important","remove_timeout":1}]`)
	now := f.now
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			if err := f.processor.Handle(context.Background(), TapEvent{Kind: "REQUEST_RECEIVED",
				MessageId: "request", Subject: "important", Level: "DEBUG"}, now); err != nil {
				errs <- err
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			if err := f.processor.Tick(context.Background(), now); err != nil {
				errs <- err
				return
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if f.count("aes_request") != 1 || f.count("aes_event_tap") != 20 {
		t.Fatal("concurrent request handling lost logs or duplicated retention records")
	}
	f.tick(1)
	if f.count("aes_event_tap") != 0 || f.count("aes_request") != 0 {
		t.Fatal("concurrent request retention did not clean up")
	}
}
