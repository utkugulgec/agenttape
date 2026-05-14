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

export interface NormalizedAttrs {
  schema: 'claude_code' | 'genai_spec' | 'unknown'
  operation_name?: string
  provider_name?: string
  request_model?: string
  response_model?: string
  input_tokens?: number
  output_tokens?: number
  cache_read_tokens?: number
  cache_creation_tokens?: number
  ttft_ms?: number
  finish_reason?: string
  tool_name?: string
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
  normalized_attrs: NormalizedAttrs | null
  events: SpanEvent[]
  started_at: string
  ended_at: string | null
  duration_ms: number | null
}

export interface SpanNode extends Span {
  children: SpanNode[]
}
