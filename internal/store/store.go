package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sudok8s/hookr/internal/model"
	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite file and runs migrations.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close shuts down the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS subscribers (
			id         TEXT PRIMARY KEY,
			url        TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS events (
			id         TEXT PRIMARY KEY,
			topic      TEXT NOT NULL,
			payload    TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS delivery_attempts (
			id            TEXT PRIMARY KEY,
			event_id      TEXT NOT NULL,
			subscriber_id TEXT NOT NULL,
			attempt_num   INTEGER NOT NULL,
			status_code   INTEGER,
			status        TEXT NOT NULL,
			error         TEXT,
			attempted_at  DATETIME NOT NULL
		);
	`)
	return err
}

// --- Subscribers ---

func (s *Store) CreateSubscriber(ctx context.Context, sub model.Subscriber) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO subscribers (id, url, created_at) VALUES (?, ?, ?)`,
		sub.ID, sub.URL, sub.CreatedAt,
	)
	return err
}

func (s *Store) ListSubscribers(ctx context.Context) ([]model.Subscriber, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, url, created_at FROM subscribers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []model.Subscriber
	for rows.Next() {
		var sub model.Subscriber
		if err := rows.Scan(&sub.ID, &sub.URL, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// --- Events ---

func (s *Store) CreateEvent(ctx context.Context, e model.Event) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO events (id, topic, payload, created_at) VALUES (?, ?, ?, ?)`,
		e.ID, e.Topic, string(payload), e.CreatedAt,
	)
	return err
}

// --- Delivery attempts ---

func (s *Store) CreateDeliveryAttempt(ctx context.Context, d model.DeliveryAttempt) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO delivery_attempts
			(id, event_id, subscriber_id, attempt_num, status_code, status, error, attempted_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.EventID, d.SubscriberID, d.AttemptNum,
		d.StatusCode, d.Status, d.Error, d.AttemptedAt,
	)
	return err
}

func (s *Store) ListDeliveriesForEvent(ctx context.Context, eventID string) ([]model.DeliveryAttempt, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_id, subscriber_id, attempt_num, status_code, status, error, attempted_at
		 FROM delivery_attempts WHERE event_id = ?
		 ORDER BY attempted_at ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []model.DeliveryAttempt
	for rows.Next() {
		var d model.DeliveryAttempt
		var errStr sql.NullString
		if err := rows.Scan(
			&d.ID, &d.EventID, &d.SubscriberID, &d.AttemptNum,
			&d.StatusCode, &d.Status, &errStr, &d.AttemptedAt,
		); err != nil {
			return nil, err
		}
		if errStr.Valid {
			d.Error = errStr.String
		}
		attempts = append(attempts, d)
	}
	return attempts, rows.Err()
}

// CountDeadLetters returns how many delivery attempts are in "dead" status.
// This will become a Prometheus gauge later.
func (s *Store) CountDeadLetters(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM delivery_attempts WHERE status = 'dead'`,
	).Scan(&count)
	return count, err
}

// helper — not exported, used in tests
func (s *Store) now() time.Time { return time.Now().UTC() }
