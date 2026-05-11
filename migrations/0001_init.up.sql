CREATE TABLE sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trace_id       CHAR(32)    NOT NULL UNIQUE,   -- 16-byte OTEL trace ID as 32-char hex
    agent_name     TEXT        NOT NULL DEFAULT '',
    status         TEXT        NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'error')),
    root_span_name TEXT        NOT NULL DEFAULT '',
    attributes     JSONB       NOT NULL DEFAULT '{}',
    started_at     TIMESTAMPTZ NOT NULL,
    ended_at       TIMESTAMPTZ,
    duration_ms    BIGINT,                        -- populated when ended_at is set
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_started_at_idx ON sessions (started_at DESC);
CREATE INDEX sessions_status_idx     ON sessions (status);
CREATE INDEX sessions_attributes_idx ON sessions USING gin (attributes);

CREATE TABLE spans (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    span_id        CHAR(16)    NOT NULL,          -- 8-byte OTEL span ID as 16-char hex
    trace_id       CHAR(32)    NOT NULL,
    parent_span_id CHAR(16),                      -- NULL for root span
    session_id     UUID        NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    name           TEXT        NOT NULL,
    kind           TEXT        NOT NULL DEFAULT 'internal' CHECK (kind IN ('internal', 'server', 'client', 'producer', 'consumer')),
    status_code    TEXT        NOT NULL DEFAULT 'unset' CHECK (status_code IN ('unset', 'ok', 'error')),
    status_message TEXT        NOT NULL DEFAULT '',
    attributes     JSONB       NOT NULL DEFAULT '{}',
    events         JSONB       NOT NULL DEFAULT '[]', -- [{name, timestamp, attributes}]
    started_at     TIMESTAMPTZ NOT NULL,
    ended_at       TIMESTAMPTZ,
    duration_ms    BIGINT,
    UNIQUE (trace_id, span_id)
);

CREATE INDEX spans_session_id_idx    ON spans (session_id);
CREATE INDEX spans_trace_id_idx      ON spans (trace_id);
CREATE INDEX spans_parent_span_id_idx ON spans (parent_span_id);
CREATE INDEX spans_started_at_idx    ON spans (started_at DESC);
CREATE INDEX spans_attributes_idx    ON spans USING gin (attributes);
