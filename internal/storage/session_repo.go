package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

func (r *SessionRepo) Insert(ctx context.Context, s *Session) error {
	const q = `
		INSERT INTO sessions
			(trace_id, agent_name, status, root_span_name, attributes, started_at, ended_at, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, q,
		s.TraceID, s.AgentName, s.Status, s.RootSpanName, s.Attributes,
		s.StartedAt, s.EndedAt, s.DurationMs,
	).Scan(&s.ID, &s.CreatedAt)
}

func (r *SessionRepo) UpdateStatus(ctx context.Context, traceID, status string, endedAt time.Time, durationMs int64) error {
	const q = `
		UPDATE sessions
		SET status=$2, ended_at=$3, duration_ms=$4
		WHERE trace_id=$1`
	_, err := r.pool.Exec(ctx, q, traceID, status, endedAt, durationMs)
	return err
}

func (r *SessionRepo) UpdateFromRootSpan(ctx context.Context, traceID, rootSpanName, status string, endedAt *time.Time, durationMs *int64) error {
	const q = `
		UPDATE sessions
		SET root_span_name=$2, status=$3, ended_at=$4, duration_ms=$5
		WHERE trace_id=$1`
	_, err := r.pool.Exec(ctx, q, traceID, rootSpanName, status, endedAt, durationMs)
	return err
}

func (r *SessionRepo) GetByID(ctx context.Context, id string) (*Session, error) {
	const q = `
		SELECT id, trace_id, agent_name, status, root_span_name, attributes,
		       started_at, ended_at, duration_ms, created_at
		FROM sessions
		WHERE id=$1`
	s := &Session{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&s.ID, &s.TraceID, &s.AgentName, &s.Status, &s.RootSpanName, &s.Attributes,
		&s.StartedAt, &s.EndedAt, &s.DurationMs, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *SessionRepo) GetByTraceID(ctx context.Context, traceID string) (*Session, error) {
	const q = `
		SELECT id, trace_id, agent_name, status, root_span_name, attributes,
		       started_at, ended_at, duration_ms, created_at
		FROM sessions
		WHERE trace_id=$1`
	s := &Session{}
	err := r.pool.QueryRow(ctx, q, traceID).Scan(
		&s.ID, &s.TraceID, &s.AgentName, &s.Status, &s.RootSpanName, &s.Attributes,
		&s.StartedAt, &s.EndedAt, &s.DurationMs, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *SessionRepo) List(ctx context.Context, limit, offset int) ([]*Session, error) {
	const q = `
		SELECT id, trace_id, agent_name, status, root_span_name, attributes,
		       started_at, ended_at, duration_ms, created_at
		FROM sessions
		ORDER BY started_at DESC
		LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		s := &Session{}
		if err := rows.Scan(
			&s.ID, &s.TraceID, &s.AgentName, &s.Status, &s.RootSpanName, &s.Attributes,
			&s.StartedAt, &s.EndedAt, &s.DurationMs, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
