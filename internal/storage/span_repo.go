package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SpanRepo struct {
	pool *pgxpool.Pool
}

func NewSpanRepo(pool *pgxpool.Pool) *SpanRepo {
	return &SpanRepo{pool: pool}
}

func (r *SpanRepo) Insert(ctx context.Context, s *Span) error {
	const q = `
		INSERT INTO spans
			(span_id, trace_id, parent_span_id, session_id, name, kind,
			 status_code, status_message, attributes, normalized_attrs, events,
			 started_at, ended_at, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (trace_id, span_id) DO NOTHING
		RETURNING id`
	err := r.pool.QueryRow(ctx, q,
		s.SpanID, s.TraceID, s.ParentSpanID, s.SessionID, s.Name, s.Kind,
		s.StatusCode, s.StatusMessage, s.Attributes, s.NormalizedAttrs, s.Events,
		s.StartedAt, s.EndedAt, s.DurationMs,
	).Scan(&s.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (r *SpanRepo) GetByID(ctx context.Context, id string) (*Span, error) {
	const q = `
		SELECT id, span_id, trace_id, parent_span_id, session_id, name, kind,
		       status_code, status_message, attributes, normalized_attrs, events,
		       started_at, ended_at, duration_ms
		FROM spans
		WHERE id=$1`
	s := &Span{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&s.ID, &s.SpanID, &s.TraceID, &s.ParentSpanID, &s.SessionID, &s.Name, &s.Kind,
		&s.StatusCode, &s.StatusMessage, &s.Attributes, &s.NormalizedAttrs, &s.Events,
		&s.StartedAt, &s.EndedAt, &s.DurationMs,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *SpanRepo) ListBySession(ctx context.Context, sessionID string) ([]*Span, error) {
	const q = `
		SELECT id, span_id, trace_id, parent_span_id, session_id, name, kind,
		       status_code, status_message, attributes, normalized_attrs, events,
		       started_at, ended_at, duration_ms
		FROM spans
		WHERE session_id=$1
		ORDER BY started_at ASC`
	rows, err := r.pool.Query(ctx, q, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spans []*Span
	for rows.Next() {
		s := &Span{}
		if err := rows.Scan(
			&s.ID, &s.SpanID, &s.TraceID, &s.ParentSpanID, &s.SessionID, &s.Name, &s.Kind,
			&s.StatusCode, &s.StatusMessage, &s.Attributes, &s.NormalizedAttrs, &s.Events,
			&s.StartedAt, &s.EndedAt, &s.DurationMs,
		); err != nil {
			return nil, err
		}
		spans = append(spans, s)
	}
	return spans, rows.Err()
}
