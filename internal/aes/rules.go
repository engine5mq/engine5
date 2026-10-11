package aes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

type SubjectKey struct {
	Exact string
	Regex string
}

func (s *SubjectKey) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &s.Exact)
	}
	var value struct {
		Regex string `json:"regex"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	s.Regex = value.Regex
	return nil
}

type Policy struct {
	TryAfter                int64  `json:"try_after"`
	TryCount                int64  `json:"try_count"`
	RemoveSuccessfulTimeout int64  `json:"remove_successful_timeout"`
	RemoveFailedTimeout     int64  `json:"remove_failed_timeout"`
	ConsumptionTimeout      int64  `json:"consumption_timeout"`
	OnConsumptionTimeout    string `json:"on_consumption_timeout"`
}

type Rule struct {
	EventSubjectKey SubjectKey `json:"event_subject_key"`
	InstanceGroup   string     `json:"instance_group"`
	InstanceName    string     `json:"instance_name"`
	Policy
	subject *regexp.Regexp
	group   *regexp.Regexp
	name    *regexp.Regexp
}

func (r *Rule) UnmarshalJSON(data []byte) error {
	type plain Rule
	value := plain{
		InstanceGroup: "*", InstanceName: "*",
		Policy: Policy{RemoveSuccessfulTimeout: 3600, OnConsumptionTimeout: "keep"},
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	*r = Rule(value)
	return nil
}

type Rules struct {
	Rules        []Rule        `json:"rules"`
	RequestRules []RequestRule `json:"request_rules"`
}

type RequestRule struct {
	RequestSubjectKey SubjectKey `json:"request_subject_key"`
	RemoveTimeout     int64      `json:"remove_timeout"`
	subject           *regexp.Regexp
}

func (r *RequestRule) UnmarshalJSON(data []byte) error {
	type plain RequestRule
	value := plain{RemoveTimeout: 3600}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	*r = RequestRule(value)
	return nil
}

func LoadRules(path string) (*Rules, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open AES rules: %w", err)
	}
	defer file.Close()
	return ReadRules(file)
}

func ReadRules(reader io.Reader) (*Rules, error) {
	var rules Rules
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rules); err != nil {
		return nil, fmt.Errorf("decode AES rules: %w", err)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("AES rules must contain one JSON object")
	}
	for i := range rules.Rules {
		if err := rules.Rules[i].compile(); err != nil {
			return nil, fmt.Errorf("AES rule %d: %w", i+1, err)
		}
	}
	for i := range rules.RequestRules {
		r := &rules.RequestRules[i]
		var err error
		r.subject, err = compileSubject(r.RequestSubjectKey)
		if err != nil {
			return nil, fmt.Errorf("AES request rule %d: request_subject_key: %w", i+1, err)
		}
		if err := validateSeconds(r.RemoveTimeout); err != nil {
			return nil, fmt.Errorf("AES request rule %d: remove_timeout: %w", i+1, err)
		}
	}
	return &rules, nil
}

func compileSubject(key SubjectKey) (*regexp.Regexp, error) {
	if key.Exact == "" && key.Regex == "" {
		return nil, fmt.Errorf("a non-empty exact string or regex is required")
	}
	if key.Regex != "" {
		return regexp.Compile(key.Regex)
	}
	return nil, nil
}

func validateSeconds(seconds int64) error {
	if seconds < 0 || seconds > int64((1<<63-1)/time.Second) {
		return fmt.Errorf("timeouts must be non-negative seconds within time.Duration range")
	}
	return nil
}

func (r *Rule) compile() error {
	var err error
	r.subject, err = compileSubject(r.EventSubjectKey)
	if err != nil {
		return fmt.Errorf("event_subject_key: %w", err)
	}
	for _, field := range []struct {
		name   string
		value  string
		target **regexp.Regexp
	}{
		{"instance_group", r.InstanceGroup, &r.group},
		{"instance_name", r.InstanceName, &r.name},
	} {
		if field.value == "*" {
			continue
		}
		if field.value == "" {
			return fmt.Errorf("%s must be '*' or a regex", field.name)
		}
		*field.target, err = regexp.Compile(field.value)
		if err != nil {
			return fmt.Errorf("%s regex: %w", field.name, err)
		}
	}
	for _, seconds := range []int64{
		r.TryAfter, r.RemoveSuccessfulTimeout, r.RemoveFailedTimeout, r.ConsumptionTimeout,
	} {
		if err := validateSeconds(seconds); err != nil {
			return err
		}
	}
	if r.TryCount < 0 {
		return fmt.Errorf("try_count must be non-negative")
	}
	switch r.OnConsumptionTimeout {
	case "keep", "try", "remove":
		return nil
	default:
		return fmt.Errorf("on_consumption_timeout must be keep, try or remove")
	}
}

func (rules *Rules) Match(subject, group, instance string) *Rule {
	for i := range rules.Rules {
		r := &rules.Rules[i]
		if matchesSubject(r.EventSubjectKey, r.subject, subject) && (r.group == nil || r.group.MatchString(group)) &&
			(r.name == nil || r.name.MatchString(instance)) {
			return r
		}
	}
	return nil
}

func matchesSubject(key SubjectKey, pattern *regexp.Regexp, subject string) bool {
	if pattern != nil {
		return pattern.MatchString(subject)
	}
	return subject == key.Exact
}

func (rules *Rules) MatchRequest(subject string) *RequestRule {
	for i := range rules.RequestRules {
		r := &rules.RequestRules[i]
		if matchesSubject(r.RequestSubjectKey, r.subject, subject) {
			return r
		}
	}
	return nil
}
