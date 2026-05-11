export interface Session {
  id: string
  trace_id: string
  agent_name: string
  status: 'running' | 'completed' | 'error'
  root_span_name: string
  attributes: Record<string, unknown>
  started_at: string
  ended_at: string | null
  duration_ms: number | null
  created_at: string
}

export interface SpanEvent {
  name: string
  timestamp: string
  attributes: Record<string, unknown>
}

export interface Span {
  id: string
  span_id: string
  trace_id: string
  parent_span_id: string | null
  session_id: string
  name: string
  kind: string
  status_code: 'ok' | 'error' | 'unset'
  status_message: string
  attributes: Record<string, unknown>
  events: SpanEvent[]
  started_at: string
  ended_at: string | null
  duration_ms: number | null
}

export interface SpanNode extends Span {
  children: SpanNode[]
}
