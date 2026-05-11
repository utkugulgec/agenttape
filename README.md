# AgentTape

A self-hosted, model-agnostic OpenTelemetry agent session visualizer.

AgentTape ingests OTLP trace data from any instrumented AI agent and visualizes sessions as interactive flow diagrams and waterfall timelines — making it easy to understand what your agent did, in what order, and why.

## Features

- **OTLP ingest** — accepts standard OpenTelemetry traces over HTTP/protobuf
- **Flow diagram** — interactive node graph showing LLM calls, tool uses, and their connections
- **Waterfall view** — span timeline with expandable attributes and events
- **Semantic layer** — translates raw span attributes into human-readable labels
- **Self-hosted** — Go backend, React frontend, PostgreSQL storage

## Stack

- **Backend**: Go, chi, pgx v5, PostgreSQL
- **Frontend**: React, TypeScript, Tailwind CSS, React Flow
- **Ingest**: OpenTelemetry Protocol (OTLP/HTTP protobuf)

## Quick start

```bash
# Start postgres and adminer
docker compose up -d postgres adminer

# Start backend + frontend together
make dev-all
```

Open [http://localhost:5173](http://localhost:5173) for the UI and [http://localhost:8081](http://localhost:8081) for the database viewer.

## Connecting an agent

Point any OTEL-instrumented agent at `http://localhost:8080`. For Claude Code:

```bash
CLAUDE_CODE_ENABLE_TELEMETRY=1 \
CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1 \
OTEL_TRACES_EXPORTER=otlp \
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:8080 \
claude
```

## Development

```bash
make test               # run tests (requires Docker for testcontainers)
go run ./cmd/seed       # populate with synthetic data
go run ./cmd/otelsnoop  # inspect raw OTLP payloads on :9999
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `HTTP_PORT` | `8080` | TCP port the server listens on |
| `DATABASE_URL` | _(required)_ | PostgreSQL connection string |

---

> This project was built leveraging agentic development with [Claude Code](https://claude.ai/code).
