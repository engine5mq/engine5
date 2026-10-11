package aes

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"engine5/internal/aes/entity"
	"engine5/internal/e5dbase"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const SchemaVersion = 1

type Store struct {
	db     *sql.DB
	driver string
}

func NewStore(manager *e5dbase.DatabaseManager) *Store {
	return &Store{db: manager.GetDB(), driver: manager.GetDriverName()}
}

func (s *Store) Init(ctx context.Context) error {
	if s.driver != "mysql" && s.driver != "sqlite3" {
		return fmt.Errorf("AES schema v1 supports mysql (sqlite3 for tests), got %q", s.driver)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS aes_schema_version (id INTEGER PRIMARY KEY, version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create AES version table: %w", err)
	}
	if err := s.CheckVersion(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	definition := entity.AesEventTapTableDefinition()
	// AES assigns UUIDs itself; database-specific UUID defaults are unnecessary.
	definition.Columns[0].SafeFunc = ""
	if _, err := s.db.ExecContext(ctx, e5dbase.TableCreationQueryFromDefinition(s.driver, definition)); err != nil {
		return fmt.Errorf("create AES tap table: %w", err)
	}
	deliveryIndex := ""
	attemptIndex := ""
	if s.driver == "mysql" {
		deliveryIndex = ", INDEX aes_delivery_due (due_at)"
		attemptIndex = ", INDEX aes_attempt_delivery (delivery_id), INDEX aes_attempt_message (message_id, subject)"
	}
	for _, query := range []string{
		`CREATE TABLE IF NOT EXISTS aes_delivery (
			id VARCHAR(64) PRIMARY KEY, state VARCHAR(32) NOT NULL,
			due_at BIGINT NOT NULL, data LONGTEXT NOT NULL` + deliveryIndex + `)`,
		`CREATE TABLE IF NOT EXISTS aes_attempt (
			id VARCHAR(64) PRIMARY KEY, delivery_id VARCHAR(64) NOT NULL,
			message_id VARCHAR(128) NOT NULL, subject VARCHAR(255) NOT NULL,
			instance_group VARCHAR(128) NOT NULL` + attemptIndex + `)`,
	} {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("create AES schema: %w", err)
		}
	}
	if s.driver == "sqlite3" {
		for _, query := range []string{
			`CREATE INDEX IF NOT EXISTS aes_delivery_due ON aes_delivery (due_at)`,
			`CREATE INDEX IF NOT EXISTS aes_attempt_delivery ON aes_attempt (delivery_id)`,
			`CREATE INDEX IF NOT EXISTS aes_attempt_message ON aes_attempt (message_id, subject)`,
			`CREATE INDEX IF NOT EXISTS aes_tap_message ON aes_event_tap (message_id, subject)`,
		} {
			if _, err := s.db.ExecContext(ctx, query); err != nil {
				return err
			}
		}
	} else {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.statistics
			WHERE table_schema = DATABASE() AND table_name = 'aes_event_tap' AND index_name = 'aes_tap_message'`).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := s.db.ExecContext(ctx, `CREATE INDEX aes_tap_message ON aes_event_tap (message_id, subject)`); err != nil {
				return err
			}
		}
	}
	var version int
	err := s.db.QueryRowContext(ctx, `SELECT version FROM aes_schema_version WHERE id = 1`).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.db.ExecContext(ctx, `INSERT INTO aes_schema_version (id, version) VALUES (1, ?)`, SchemaVersion)
		return err
	}
	if err != nil {
		return err
	}
	if version != SchemaVersion {
		return fmt.Errorf("unsupported AES schema version %d; expected %d", version, SchemaVersion)
	}
	return nil
}

func (s *Store) CheckVersion(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM aes_schema_version WHERE id = 1`).Scan(&version); err != nil {
		return fmt.Errorf("read AES schema version: %w", err)
	}
	if version != SchemaVersion {
		return fmt.Errorf("unsupported AES schema version %d; expected %d", version, SchemaVersion)
	}
	return nil
}

func recordKey(values ...string) string {
	data, _ := json.Marshal(values)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (s *Store) load(ctx context.Context, tx *sql.Tx, id string) (*entity.AesDelivery, error) {
	query := `SELECT data FROM aes_delivery WHERE id = ?`
	if s.driver == "mysql" {
		query += ` FOR UPDATE`
	}
	var data []byte
	if err := tx.QueryRowContext(ctx, query, id).Scan(&data); err != nil {
		return nil, err
	}
	var delivery entity.AesDelivery
	if err := json.Unmarshal(data, &delivery); err != nil {
		return nil, fmt.Errorf("decode AES delivery %s: %w", id, err)
	}
	return &delivery, nil
}

func (s *Store) find(ctx context.Context, tx *sql.Tx, ev TapEvent) (*entity.AesDelivery, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT delivery_id FROM aes_attempt WHERE id = ?`,
		recordKey(ev.MessageId, ev.Subject, ev.Group)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.load(ctx, tx, id)
}

func saveDelivery(ctx context.Context, tx *sql.Tx, d *entity.AesDelivery, insert bool) error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	due := int64(0)
	if !d.NextActionAt.IsZero() {
		due = d.NextActionAt.UnixMilli()
	}
	if insert {
		_, err = tx.ExecContext(ctx, `INSERT INTO aes_delivery (state, due_at, data, id) VALUES (?, ?, ?, ?)`,
			d.State, due, data, d.Id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE aes_delivery SET state = ?, due_at = ?, data = ? WHERE id = ?`,
			d.State, due, data, d.Id)
	}
	return err
}

func addAttempt(ctx context.Context, tx *sql.Tx, d *entity.AesDelivery) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO aes_attempt (id, delivery_id, message_id, subject, instance_group)
		VALUES (?, ?, ?, ?, ?)`, recordKey(d.AttemptMessageId, d.Subject, d.Group), d.Id,
		d.AttemptMessageId, d.Subject, d.Group)
	return err
}

func insertTap(ctx context.Context, tx *sql.Tx, ev TapEvent) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO aes_event_tap
		(id, time, level, kind, instance, instance_group, subject, message_id, remote, content, err, msg)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), ev.Time.UTC(), ev.Level, ev.Kind, ev.Instance, ev.Group,
		ev.Subject, ev.MessageId, ev.Remote, ev.Content, ev.Err, ev.Msg)
	return err
}

func (s *Store) dueIDs(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM aes_delivery
		WHERE due_at > 0 AND due_at <= ? ORDER BY due_at, id LIMIT 100`, now.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func deleteDelivery(ctx context.Context, tx *sql.Tx, d *entity.AesDelivery) error {
	rows, err := tx.QueryContext(ctx, `SELECT message_id FROM aes_attempt WHERE delivery_id = ?`, d.Id)
	if err != nil {
		return err
	}
	var messages []string
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			rows.Close()
			return err
		}
		messages = append(messages, message)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM aes_attempt WHERE delivery_id = ?`, d.Id); err != nil {
		return err
	}
	for _, message := range messages {
		if _, err := tx.ExecContext(ctx, `DELETE FROM aes_event_tap WHERE message_id = ? AND subject = ? AND instance_group = ?`,
			message, d.Subject, d.Group); err != nil {
			return err
		}
		// Keep shared original logs while another group's delivery still needs them.
		var references int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM aes_attempt WHERE message_id = ? AND subject = ?`,
			message, d.Subject).Scan(&references); err != nil {
			return err
		}
		if references == 0 {
			if _, err := tx.ExecContext(ctx, `DELETE FROM aes_event_tap WHERE message_id = ? AND subject = ? AND kind = 'EVENT_RECEIVED'`,
				message, d.Subject); err != nil {
				return err
			}
		}
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM aes_delivery WHERE id = ?`, d.Id)
	return err
}
