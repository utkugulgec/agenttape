package storage

import (
	"encoding/json"
	"time"
)

type Session struct {
	ID           string          `json:"id"`
	TraceID      string          `json:"trace_id"`
	AgentName    string          `json:"agent_name"`
	Status       string          `json:"status"`
	RootSpanName string          `json:"root_span_name"`
	Attributes   json.RawMessage `json:"attributes"`
	StartedAt    time.Time       `json:"started_at"`
	EndedAt      *time.Time      `json:"ended_at"`
	DurationMs   *int64          `json:"duration_ms"`
	CreatedAt    time.Time       `json:"created_at"`
}

type Span struct {
	ID              string          `json:"id"`
	SpanID          string          `json:"span_id"`
	TraceID         string          `json:"trace_id"`
	ParentSpanID    *string         `json:"parent_span_id"`
	SessionID       string          `json:"session_id"`
	Name            string          `json:"name"`
	Kind            string          `json:"kind"`
	StatusCode      string          `json:"status_code"`
	StatusMessage   string          `json:"status_message"`
	Attributes      json.RawMessage `json:"attributes"`
	NormalizedAttrs json.RawMessage `json:"normalized_attrs"`
	Events          json.RawMessage `json:"events"`
	StartedAt       time.Time       `json:"started_at"`
	EndedAt         *time.Time      `json:"ended_at"`
	DurationMs      *int64          `json:"duration_ms"`
}
