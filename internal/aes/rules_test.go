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
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := ReadRules(strings.NewReader(input)); err == nil {
				t.Fatal("invalid rules accepted")
			}
		})
	}
}
