package aes

import (
	"context"
	"database/sql"
	"encoding/json"
	"engine5/internal/aes/entity"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	StateAwaiting  = "awaiting_result"
	StatePending   = "retry_pending"
	StateKept      = "kept"
	StateDisabled  = "retry_disabled"
	StateSucceeded = "succeeded"
	StateExhausted = "exhausted"
)

type RetryMessage struct {
	MessageID         string
	OriginalMessageID string
	Subject           string
	Group             string
	Content           string
}

type Publisher interface {
	Publish(context.Context, RetryMessage) error
}

var ErrNoTarget = errors.New("no active consumer in target group")
var ErrRetryRejected = errors.New("retry rejected by broker")

type Processor struct {
	store     *Store
	rules     *Rules
	publisher Publisher
	mu        sync.Mutex
}

func NewProcessor(store *Store, rules *Rules, publisher Publisher) *Processor {
	return &Processor{store: store, rules: rules, publisher: publisher}
}

func policyOf(d *entity.AesDelivery) (Policy, error) {
	var policy Policy
	if err := json.Unmarshal(d.Policy, &policy); err != nil {
		return policy, fmt.Errorf("decode policy for %s: %w", d.Id, err)
	}
	return policy, nil
}

func after(now time.Time, seconds int64) time.Time {
	if seconds == 0 {
		return time.Time{}
	}
	return now.Add(time.Duration(seconds) * time.Second)
}

func fail(d *entity.AesDelivery, p Policy, now time.Time, reason string) {
	d.LastError = reason
	d.CompletedAt = time.Time{}
	switch {
	case p.TryAfter == 0:
		d.State = StateDisabled
	case p.TryCount > 0 && d.RetryCount >= p.TryCount:
		d.State = StateExhausted
	default:
		d.State = StatePending
		d.NextActionAt = after(now, p.TryAfter)
		return
	}
	d.CompletedAt = now
	d.NextActionAt = after(now, p.RemoveFailedTimeout)
}

func (p *Processor) Handle(ctx context.Context, ev TapEvent, now time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	tx, err := p.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if ev.Time.IsZero() {
		ev.Time = now
	}
	if err := insertTap(ctx, tx, ev); err != nil {
		return fmt.Errorf("save tap event: %w", err)
	}
	switch ev.Kind {
	case "EVENT_DELIVERED", "CONSUMING_SUCCESS", "CONSUMING_ERROR":
		if ev.MessageId == "" || ev.Subject == "" {
			return fmt.Errorf("%s missing messageId or subject", ev.Kind)
		}
	default:
		return tx.Commit()
	}
	d, err := p.store.find(ctx, tx, ev)
	insert := false
	if errors.Is(err, sql.ErrNoRows) {
		if ev.Kind != "EVENT_DELIVERED" || ev.OriginalMessageId != "" {
			if err := tx.Commit(); err != nil {
				return err
			}
			slog.Warn("AES event has no tracked delivery", "kind", ev.Kind, "messageId", ev.MessageId, "subject", ev.Subject, "group", ev.Group)
			return nil
		}
		rule := p.rules.Match(ev.Subject, ev.Group, ev.Instance)
		if rule == nil {
			return tx.Commit()
		}
		policy, err := json.Marshal(rule.Policy)
		if err != nil {
			return err
		}
		d = &entity.AesDelivery{
			Id:                recordKey(ev.MessageId, ev.Subject, ev.Group),
			OriginalMessageId: ev.MessageId, AttemptMessageId: ev.MessageId,
			Subject: ev.Subject, Group: ev.Group, Instance: ev.Instance,
			Content: ev.Content, State: StateAwaiting, Policy: policy,
			NextActionAt: after(now, rule.ConsumptionTimeout),
		}
		insert = true
		if err := addAttempt(ctx, tx, d); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	policy, err := policyOf(d)
	if err != nil {
		return err
	}
	switch ev.Kind {
	case "EVENT_DELIVERED":
		if !insert && d.State == StateAwaiting && d.AttemptMessageId == ev.MessageId {
			d.Instance = ev.Instance
			// Retries start their consumption clock when actually delivered.
			if ev.MessageId != d.OriginalMessageId {
				d.NextActionAt = after(now, policy.ConsumptionTimeout)
			}
		}
	case "CONSUMING_SUCCESS":
		if d.State != StateSucceeded {
			d.State = StateSucceeded
			d.CompletedAt = now
			d.NextActionAt = after(now, policy.RemoveSuccessfulTimeout)
			d.LastError = ""
		}
	case "CONSUMING_ERROR":
		if d.AttemptMessageId == ev.MessageId && (d.State == StateAwaiting || d.State == StateKept) {
			reason := ev.Err
			if reason == "" {
				reason = ev.Msg
			}
			fail(d, policy, now, reason)
		}
	}
	if err := saveDelivery(ctx, tx, d, insert); err != nil {
		return err
	}
	return tx.Commit()
}

func (p *Processor) Tick(ctx context.Context, now time.Time) error {
	ids, err := p.store.dueIDs(ctx, now)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := p.processDue(ctx, id, now); err != nil {
			return fmt.Errorf("process AES delivery %s: %w", id, err)
		}
	}
	return nil
}

func (p *Processor) reserve(ctx context.Context, id string, now time.Time) (*entity.AesDelivery, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	tx, err := p.store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := p.store.load(ctx, tx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if d.NextActionAt.IsZero() || d.NextActionAt.After(now) {
		return nil, nil
	}
	policy, err := policyOf(d)
	if err != nil {
		return nil, err
	}
	switch d.State {
	case StateSucceeded, StateDisabled, StateExhausted:
		if err := deleteDelivery(ctx, tx, d); err != nil {
			return nil, err
		}
		return nil, tx.Commit()
	case StateAwaiting:
		switch policy.OnConsumptionTimeout {
		case "keep":
			d.State = StateKept
			d.LastError = "consumption timeout"
			d.NextActionAt = time.Time{}
		case "try":
			fail(d, policy, now, "consumption timeout")
		case "remove":
			if err := deleteDelivery(ctx, tx, d); err != nil {
				return nil, err
			}
			return nil, tx.Commit()
		default:
			return nil, fmt.Errorf("unknown consumption timeout action %q", policy.OnConsumptionTimeout)
		}
	case StatePending:
		if policy.TryAfter == 0 || (policy.TryCount > 0 && d.RetryCount >= policy.TryCount) {
			fail(d, policy, now, d.LastError)
			break
		}
		d.RetryCount++
		d.AttemptMessageId = uuid.NewString()
		d.State = StateAwaiting
		d.NextActionAt = after(now, policy.ConsumptionTimeout)
		if err := addAttempt(ctx, tx, d); err != nil {
			return nil, err
		}
		if err := saveDelivery(ctx, tx, d, false); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return d, nil
	default:
		return nil, fmt.Errorf("unexpected scheduled state %q", d.State)
	}
	if err := saveDelivery(ctx, tx, d, false); err != nil {
		return nil, err
	}
	return nil, tx.Commit()
}

func (p *Processor) processDue(ctx context.Context, id string, now time.Time) error {
	d, err := p.reserve(ctx, id, now)
	if err != nil || d == nil {
		return err
	}
	err = p.publisher.Publish(ctx, RetryMessage{
		MessageID: d.AttemptMessageId, OriginalMessageID: d.OriginalMessageId,
		Subject: d.Subject, Group: d.Group, Content: d.Content,
	})
	if err == nil {
		return nil
	}
	slog.Error("AES retry publish failed", "delivery", id, "error", err)
	// An ambiguous network failure may have delivered the message. Keep waiting
	// for its result/timeout; only a definite broker rejection releases the attempt.
	if !errors.Is(err, ErrNoTarget) && !errors.Is(err, ErrRetryRejected) {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	tx, txErr := p.store.db.BeginTx(ctx, nil)
	if txErr != nil {
		return txErr
	}
	defer tx.Rollback()
	current, txErr := p.store.load(ctx, tx, id)
	if errors.Is(txErr, sql.ErrNoRows) {
		return nil
	}
	if txErr != nil {
		return txErr
	}
	if current.State != StateAwaiting || current.AttemptMessageId != d.AttemptMessageId {
		return nil
	}
	policy, txErr := policyOf(current)
	if txErr != nil {
		return txErr
	}
	current.RetryCount--
	current.State = StatePending
	current.LastError = err.Error()
	current.NextActionAt = after(now, policy.TryAfter)
	if txErr := saveDelivery(ctx, tx, current, false); txErr != nil {
		return txErr
	}
	return tx.Commit()
}

func (p *Processor) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := p.Tick(ctx, now); err != nil && ctx.Err() == nil {
				slog.Error("AES scheduler failed", "error", err)
			}
		}
	}
}
