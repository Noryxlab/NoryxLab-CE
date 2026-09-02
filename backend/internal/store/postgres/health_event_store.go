package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
)

// HealthEventStore persists what the platform noticed about itself.
type HealthEventStore struct{ Store *Store }

// Raise inserts a condition, ignoring one that is already open.
//
// The uniqueness is enforced by a partial index rather than by reading first
// and writing after: two replicas sweeping at the same second would both read
// "not open" and both insert.
func (s *HealthEventStore) Raise(event health.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := event.ID
	if id == "" {
		id = uuid.NewString()
	}
	raisedAt := event.RaisedAt
	if raisedAt.IsZero() {
		raisedAt = time.Now().UTC()
	}
	_, err := s.Store.db.ExecContext(ctx, `
		INSERT INTO platform_health_events (id, key, source, severity, summary, detail, raised_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT DO NOTHING`,
		id, event.Key, event.Source, string(event.Severity), event.Summary, event.Detail, raisedAt.UTC())
	return err
}

func (s *HealthEventStore) Resolve(key string, at time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Store.db.ExecContext(ctx, `
		UPDATE platform_health_events SET resolved_at = $2
		WHERE key = $1 AND resolved_at IS NULL`, key, at.UTC())
	return err
}

func (s *HealthEventStore) Open() ([]health.Event, error) {
	return s.query(`
		SELECT id, key, source, severity, summary, detail, raised_at, resolved_at
		FROM platform_health_events WHERE resolved_at IS NULL ORDER BY raised_at DESC`)
}

func (s *HealthEventStore) List(since time.Time, limit int) ([]health.Event, error) {
	if limit <= 0 {
		limit = 200
	}
	return s.query(`
		SELECT id, key, source, severity, summary, detail, raised_at, resolved_at
		FROM platform_health_events WHERE raised_at >= $1
		ORDER BY raised_at DESC LIMIT $2`, since.UTC(), limit)
}

// Purge removes resolved events only. An open condition is never deleted
// however old: one that has been firing for four months is the most important
// row in the table.
func (s *HealthEventStore) Purge(before time.Time) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := s.Store.db.ExecContext(ctx, `
		DELETE FROM platform_health_events
		WHERE resolved_at IS NOT NULL AND resolved_at < $1`, before.UTC())
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func (s *HealthEventStore) query(statement string, args ...any) ([]health.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.Store.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []health.Event{}
	for rows.Next() {
		var event health.Event
		var severity string
		var resolvedAt sql.NullTime
		if err := rows.Scan(&event.ID, &event.Key, &event.Source, &severity,
			&event.Summary, &event.Detail, &event.RaisedAt, &resolvedAt); err != nil {
			return nil, err
		}
		event.Severity = health.Severity(severity)
		if resolvedAt.Valid {
			stamp := resolvedAt.Time.UTC()
			event.ResolvedAt = &stamp
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
