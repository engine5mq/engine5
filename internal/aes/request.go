package aes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const requestLogKindsSQL = `('REQUEST_RECEIVED', 'REQUEST_ROUTED', 'REQUEST_NO_TARGET', 'RESPONSE_RECEIVED', 'RESPONSE_DELIVERED')`

func isRequestLog(kind string) bool {
	switch kind {
	case "REQUEST_RECEIVED", "REQUEST_ROUTED", "REQUEST_NO_TARGET", "RESPONSE_RECEIVED", "RESPONSE_DELIVERED":
		return true
	default:
		return false
	}
}

type requestRecord struct {
	ID            string
	MessageID     string
	Subject       string
	RemoveTimeout int64
	DueAt         int64
}

func (s *Store) loadRequest(ctx context.Context, tx *sql.Tx, id string) (*requestRecord, error) {
	query := `SELECT message_id, subject, remove_timeout, due_at FROM aes_request WHERE id = ?`
	if s.driver == "mysql" {
		query += ` FOR UPDATE`
	}
	record := &requestRecord{ID: id}
	err := tx.QueryRowContext(ctx, query, id).Scan(
		&record.MessageID, &record.Subject, &record.RemoveTimeout, &record.DueAt)
	return record, err
}

// Caller holds the processor mutex, shared with request cleanup.
func (p *Processor) handleRequest(ctx context.Context, ev TapEvent, now time.Time) error {
	if ev.MessageId == "" {
		return fmt.Errorf("%s missing messageId for request retention", ev.Kind)
	}
	tx, err := p.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	record, err := p.store.loadRequest(ctx, tx, recordKey(ev.MessageId))
	if errors.Is(err, sql.ErrNoRows) {
		// Responses never start retention or reopen a request already cleaned.
		if ev.Subject == "" || ev.Kind == "RESPONSE_RECEIVED" || ev.Kind == "RESPONSE_DELIVERED" {
			return nil
		}
		rule := p.rules.MatchRequest(ev.Subject)
		if rule == nil {
			return nil
		}
		record = &requestRecord{
			ID: recordKey(ev.MessageId), MessageID: ev.MessageId,
			Subject: ev.Subject, RemoveTimeout: rule.RemoveTimeout,
		}
		if due := after(now, rule.RemoveTimeout); !due.IsZero() {
			record.DueAt = due.UnixMilli()
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO aes_request (id, message_id, subject, remove_timeout, due_at)
			VALUES (?, ?, ?, ?, ?)`, record.ID, record.MessageID, record.Subject, record.RemoveTimeout, record.DueAt)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if ev.Subject != "" && ev.Subject != record.Subject {
		return fmt.Errorf("request %s subject mismatch: stored %q, received %q", ev.MessageId, record.Subject, ev.Subject)
	}
	if record.DueAt > 0 && record.DueAt <= now.UnixMilli() {
		if err := deleteRequest(ctx, tx, record); err != nil {
			return err
		}
		return tx.Commit()
	}
	ev.Subject = record.Subject
	if ev.Time.IsZero() {
		ev.Time = now
	}
	if err := insertTap(ctx, tx, ev); err != nil {
		return fmt.Errorf("save retained request log: %w", err)
	}
	return tx.Commit()
}

func deleteRequest(ctx context.Context, tx *sql.Tx, record *requestRecord) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM aes_event_tap WHERE message_id = ? AND kind IN `+requestLogKindsSQL,
		record.MessageID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM aes_request WHERE id = ?`, record.ID)
	return err
}

func (p *Processor) cleanupRequests(ctx context.Context, now time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	rows, err := p.store.db.QueryContext(ctx, `SELECT id FROM aes_request
		WHERE due_at > 0 AND due_at <= ? ORDER BY due_at, id LIMIT 100`, now.UnixMilli())
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := p.cleanupRequest(ctx, id, now); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) cleanupRequest(ctx context.Context, id string, now time.Time) error {
	tx, err := p.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	record, err := p.store.loadRequest(ctx, tx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if record.DueAt == 0 || record.DueAt > now.UnixMilli() {
		return nil
	}
	if err := deleteRequest(ctx, tx, record); err != nil {
		return err
	}
	return tx.Commit()
}
