package aes

import (
	"strings"
	"testing"
)

func TestRulesDefaultsAndMatching(t *testing.T) {
	rules, err := ReadRules(strings.NewReader(`{"rules":[
		{"event_subject_key":"order.created"},
		{"event_subject_key":{"regex":"^payment\\."},"instance_group":"^pay$","instance_name":"^worker-"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	r := rules.Match("order.created", "any", "any")
	if r == nil || r.RemoveSuccessfulTimeout != 3600 || r.TryAfter != 0 || r.TryCount != 0 ||
		r.RemoveFailedTimeout != 0 || r.ConsumptionTimeout != 0 || r.OnConsumptionTimeout != "keep" {
		t.Fatalf("incorrect defaults: %+v", r)
	}
	if rules.Match("orderXcreated", "any", "any") != nil {
		t.Fatal("literal subject was interpreted as regex")
	}
	if rules.Match("payment.completed", "pay", "worker-2") == nil ||
		rules.Match("payment.completed", "other", "worker-2") != nil ||
		rules.Match("payment.completed", "pay", "other") != nil {
		t.Fatal("regex filters did not match correctly")
	}
}

func TestRulesExplicitZeroAndFirstMatch(t *testing.T) {
	rules, err := ReadRules(strings.NewReader(`{"rules":[
		{"event_subject_key":{"regex":".*"},"remove_successful_timeout":0},
		{"event_subject_key":"order.created","try_count":5}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	r := rules.Match("order.created", "", "")
	if r == nil || r.RemoveSuccessfulTimeout != 0 || r.TryCount != 0 {
		t.Fatalf("zero or precedence lost: %+v", r)
	}
}

func TestRulesValidation(t *testing.T) {
	for _, input := range []string{
		`{"rules":[{}]}`,
		`{"rules":[{"event_subject_key":null}]}`,
		`{"rules":[{"event_subject_key":""}]}`,
		`{"rules":[{"event_subject_key":{"regex":"["}}]}`,
		`{"rules":[{"event_subject_key":"x","instance_group":"["}]}`,
		`{"rules":[{"event_subject_key":"x","instance_name":""}]}`,
		`{"rules":[{"event_subject_key":"x","try_after":-1}]}`,
		`{"rules":[{"event_subject_key":"x","try_count":-1}]}`,
		`{"rules":[{"event_subject_key":"x","consumption_timeout":9223372036854775807}]}`,
		`{"rules":[{"event_subject_key":"x","on_consumption_timeout":"unknown"}]}`,
		`{"rules":[{"event_subject_key":"x","typo":1}]}`,
		`{"rules":[{"event_subject_key":{"regexp":"x"}}]}`,
		`{"rules":[]} {}`,
		`{"rules":[],"request_rules":[{}]}`,
		`{"rules":[],"request_rules":[{"request_subject_key":""}]}`,
		`{"rules":[],"request_rules":[{"request_subject_key":{"regex":"["}}]}`,
		`{"rules":[],"request_rules":[{"request_subject_key":"x","remove_timeout":-1}]}`,
		`{"rules":[],"request_rules":[{"request_subject_key":"x","remove_timeout":9223372037}]}`,
		`{"rules":[],"request_rules":[{"request_subject_key":"x","remove_timeout":0.5}]}`,
		`{"rules":[],"request_rules":[{"request_subject_key":"x","try_after":30}]}`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := ReadRules(strings.NewReader(input)); err == nil {
				t.Fatal("invalid rules accepted")
			}
		})
	}
}

func TestExampleManifest(t *testing.T) {
	rules, err := LoadRules("../../aes.rules.example.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules.Rules) != 3 {
		t.Fatalf("expected three example policies, got %d", len(rules.Rules))
	}
	order := rules.Match("order.created", "orders", "orders-worker-1")
	if order == nil || order.TryAfter != 30 || order.TryCount != 5 ||
		order.ConsumptionTimeout != 120 || order.OnConsumptionTimeout != "try" ||
		order.RemoveSuccessfulTimeout != 3600 || order.RemoveFailedTimeout != 86400 {
		t.Fatalf("order example differs from documented policy: %+v", order)
	}
	if rules.Match("order.created", "other", "orders-worker-1") != nil ||
		rules.Match("order.created", "orders", "other") != nil {
		t.Fatal("order example must filter initial consumer group and name")
	}
	payment := rules.Match("payment.completed", "any", "any")
	if payment == nil || payment.TryAfter != 60 || payment.TryCount != 0 ||
		payment.ConsumptionTimeout != 300 || payment.OnConsumptionTimeout != "keep" ||
		payment.RemoveSuccessfulTimeout != 3600 || payment.RemoveFailedTimeout != 0 {
		t.Fatalf("payment example differs from documented policy: %+v", payment)
	}
	if rules.Match("paymentXcompleted", "any", "any") != nil {
		t.Fatal("payment regex must match a literal dot")
	}
	temporary := rules.Match("temporary.notification", "any", "any")
	if temporary == nil || temporary.TryAfter != 0 || temporary.TryCount != 0 ||
		temporary.ConsumptionTimeout != 600 || temporary.OnConsumptionTimeout != "remove" ||
		temporary.RemoveSuccessfulTimeout != 60 || temporary.RemoveFailedTimeout != 0 {
		t.Fatalf("temporary example differs from documented policy: %+v", temporary)
	}
	audit := rules.MatchRequest("audit/payment-details")
	permanent := rules.MatchRequest("audit/permanent/example")
	if len(rules.RequestRules) != 2 || audit == nil || audit.RemoveTimeout != 86400 ||
		permanent == nil || permanent.RemoveTimeout != 0 || rules.MatchRequest("ordinary") != nil {
		t.Fatal("example request policies differ from documentation")
	}
}

func TestRequestRulesDefaultsAndPrecedence(t *testing.T) {
	rules, err := ReadRules(strings.NewReader(`{"rules":[],"request_rules":[
		{"request_subject_key":"audit.details"},
		{"request_subject_key":{"regex":"^audit\\."},"remove_timeout":0}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	exact := rules.MatchRequest("audit.details")
	regex := rules.MatchRequest("audit.other")
	if exact == nil || exact.RemoveTimeout != 3600 || regex == nil || regex.RemoveTimeout != 0 {
		t.Fatal("default, explicit zero or first-match precedence lost")
	}
	if rules.MatchRequest("auditXdetails") != nil {
		t.Fatal("request subject string interpreted as regex")
	}
	empty, err := ReadRules(strings.NewReader(`{"rules":[]}`))
	if err != nil || empty.MatchRequest("anything") != nil {
		t.Fatal("omitted request_rules must discard new request logs")
	}
}
