import { useEffect, useState, useCallback } from 'react'
import { api } from './api'
import type { Session, Span, SpanNode } from './types'
import FlowDiagram from './FlowDiagram'
import { interpretSpan } from './semantic'
import { useWebSocket } from './useWebSocket'

function buildTree(spans: Span[]): SpanNode[] {
  const bySpanId = new Map<string, SpanNode>()
  for (const s of spans) bySpanId.set(s.span_id, { ...s, children: [] })

  const roots: SpanNode[] = []
  for (const node of bySpanId.values()) {
    if (node.parent_span_id && bySpanId.has(node.parent_span_id)) {
      bySpanId.get(node.parent_span_id)!.children.push(node)
    } else {
      roots.push(node)
    }
  }
  return roots
}

function formatDuration(ms: number | null): string {
  if (ms === null) return '—'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  const m = Math.floor(ms / 60_000)
  const s = Math.floor((ms % 60_000) / 1000)
  return `${m}m ${s}s`
}

const STATUS_COLORS: Record<string, string> = {
  ok: 'bg-green-500',
  error: 'bg-red-500',
  unset: 'bg-gray-500',
}

const KIND_COLORS: Record<string, string> = {
  internal: 'text-gray-400',
  server: 'text-blue-400',
  client: 'text-purple-400',
  producer: 'text-yellow-400',
  consumer: 'text-orange-400',
}

// ─── span detail panel ───────────────────────────────────────────────────────

function attr<T>(span: Span, key: string): T | undefined {
  return (span.attributes as Record<string, unknown>)?.[key] as T | undefined
}

interface SpanDetailPanelProps {
  span: Span
  onClose: () => void
}

function SpanDetailPanel({ span, onClose }: SpanDetailPanelProps) {
  const spanType = attr<string>(span, 'span.type') ?? ''
  const isLLM = spanType === 'llm_request'
  const isError = span.status_code === 'error'

  const semantic = interpretSpan(span)
  const accentBorder = isError ? 'border-red-700' : isLLM ? 'border-blue-700' : 'border-purple-700'
  const accentText = isError ? 'text-red-300' : isLLM ? 'text-blue-300' : 'text-purple-300'

  const attrs = span.attributes as Record<string, unknown> | null
  const events = Array.isArray(span.events) ? span.events : []

  // surface the most useful fields first
  const highlights: { label: string; value: string }[] = []
  if (isLLM) {
    const model = attr<string>(span, 'model')
    const inputTokens = attr<number>(span, 'input_tokens')
    const outputTokens = attr<number>(span, 'output_tokens')
    const cacheRead = attr<number>(span, 'cache_read_tokens')
    const ttft = attr<number>(span, 'ttft_ms')
    const stopReason = attr<string>(span, 'stop_reason')
    if (model) highlights.push({ label: 'Model', value: model })
    if (inputTokens != null) highlights.push({ label: 'Input tokens', value: String(inputTokens) })
    if (outputTokens != null) highlights.push({ label: 'Output tokens', value: String(outputTokens) })
    if (cacheRead != null) highlights.push({ label: 'Cache read tokens', value: String(cacheRead) })
    if (ttft != null) highlights.push({ label: 'Time to first token', value: `${ttft}ms` })
    if (stopReason) highlights.push({ label: 'Stop reason', value: stopReason })
  } else {
    const toolName = attr<string>(span, 'tool_name')
    if (toolName) highlights.push({ label: 'Tool', value: toolName })
  }
  if (span.duration_ms != null) highlights.push({ label: 'Duration', value: formatDuration(span.duration_ms) })
  highlights.push({ label: 'Started', value: new Date(span.started_at).toLocaleString() })
  highlights.push({ label: 'Status', value: span.status_code })
  highlights.push({ label: 'Kind', value: span.kind })

  return (
    <div className={`bg-gray-900 border ${accentBorder} rounded-xl p-4 mt-4 relative`}>
      <button
        onClick={onClose}
        className="absolute top-3 right-3 text-gray-600 hover:text-gray-300 text-sm transition-colors"
        aria-label="Close"
      >
        ✕
      </button>

      <p className="text-gray-200 text-sm font-medium mb-1">{semantic.icon} {semantic.label}</p>
      <p className={`font-mono text-xs mb-3 ${accentText}`}>{span.name}</p>

      {/* highlights grid */}
      <div className="grid grid-cols-2 gap-x-6 gap-y-2 mb-4">
        {highlights.map(({ label, value }) => (
          <div key={label}>
            <p className="text-gray-500 text-xs">{label}</p>
            <p className="text-gray-200 text-xs font-mono mt-0.5">{value}</p>
          </div>
        ))}
      </div>

      {/* raw attributes */}
      {attrs && Object.keys(attrs).length > 0 && (
        <details className="mb-3">
          <summary className="text-gray-500 text-xs cursor-pointer hover:text-gray-300 select-none">
            All attributes
          </summary>
          <pre className="mt-2 text-xs font-mono text-gray-300 bg-gray-950 rounded p-3 whitespace-pre-wrap break-all overflow-auto max-h-64">
            {JSON.stringify(attrs, null, 2)}
          </pre>
        </details>
      )}

      {/* events */}
      {events.length > 0 && (
        <details>
          <summary className="text-gray-500 text-xs cursor-pointer hover:text-gray-300 select-none">
            Events ({events.length})
          </summary>
          <pre className="mt-2 text-xs font-mono text-gray-300 bg-gray-950 rounded p-3 whitespace-pre-wrap break-all overflow-auto max-h-64">
            {JSON.stringify(events, null, 2)}
          </pre>
        </details>
      )}
    </div>
  )
}

// ─── waterfall ───────────────────────────────────────────────────────────────

interface SpanRowProps {
  node: SpanNode
  depth: number
  sessionStartMs: number
  sessionDurationMs: number
}

function SpanRow({ node, depth, sessionStartMs, sessionDurationMs }: SpanRowProps) {
  const [open, setOpen] = useState(false)
  const startOffset = new Date(node.started_at).getTime() - sessionStartMs
  const offsetPct = sessionDurationMs > 0 ? (startOffset / sessionDurationMs) * 100 : 0
  const widthPct =
    sessionDurationMs > 0 ? Math.max(((node.duration_ms ?? 0) / sessionDurationMs) * 100, 0.3) : 1

  const hasAttrs = Object.keys(node.attributes ?? {}).length > 0
  const hasEvents = Array.isArray(node.events) && node.events.length > 0

  return (
    <>
      <div
        className="flex items-center gap-2 py-1.5 px-3 hover:bg-gray-800/60 rounded group"
        style={{ paddingLeft: `${12 + depth * 20}px` }}
      >
        <span className={`h-2 w-2 rounded-full shrink-0 ${STATUS_COLORS[node.status_code] ?? 'bg-gray-500'}`} />
        <span
          className={`text-xs font-mono w-56 truncate shrink-0 ${KIND_COLORS[node.kind] ?? 'text-gray-300'}`}
          title={node.name}
        >
          {node.name}
        </span>

        <div className="flex-1 h-4 bg-gray-800 rounded relative overflow-hidden">
          <div
            className="absolute h-full bg-blue-600/70 rounded"
            style={{ left: `${offsetPct}%`, width: `${widthPct}%` }}
          />
        </div>

        <span className="text-xs text-gray-500 tabular-nums w-14 text-right shrink-0">
          {formatDuration(node.duration_ms)}
        </span>

        {(hasAttrs || hasEvents) && (
          <button
            onClick={() => setOpen((v) => !v)}
            className="text-gray-600 hover:text-gray-300 text-xs ml-1 shrink-0 transition-colors"
          >
            {open ? '▲' : '▼'}
          </button>
        )}
      </div>

      {open && (
        <div
          className="mx-3 mb-1 bg-gray-900 rounded text-xs font-mono text-gray-300 p-3 space-y-2"
          style={{ marginLeft: `${20 + depth * 20}px` }}
        >
          {hasAttrs && (
            <div>
              <p className="text-gray-500 mb-1">attributes</p>
              <pre className="whitespace-pre-wrap break-all">{JSON.stringify(node.attributes, null, 2)}</pre>
            </div>
          )}
          {hasEvents && (
            <div>
              <p className="text-gray-500 mb-1">events</p>
              <pre className="whitespace-pre-wrap break-all">{JSON.stringify(node.events, null, 2)}</pre>
            </div>
          )}
        </div>
      )}

      {node.children.map((child) => (
        <SpanRow
          key={child.id}
          node={child}
          depth={depth + 1}
          sessionStartMs={sessionStartMs}
          sessionDurationMs={sessionDurationMs}
        />
      ))}
    </>
  )
}

// ─── main component ──────────────────────────────────────────────────────────

interface Props {
  sessionId: string
  onBack: () => void
}

export default function SessionDetail({ sessionId, onBack }: Props) {
  const [session, setSession] = useState<Session | null>(null)
  const [roots, setRoots] = useState<SpanNode[]>([])
  const [allSpans, setAllSpans] = useState<Span[]>([])
  const [view, setView] = useState<'waterfall' | 'diagram'>('diagram')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null)

  useEffect(() => {
    Promise.all([api.sessions.get(sessionId), api.sessions.spans(sessionId)])
      .then(([sess, { spans }]) => {
        setSession(sess)
        setAllSpans(spans ?? [])
        setRoots(buildTree(spans ?? []))
      })
      .catch((e: unknown) => setError(String(e)))
      .finally(() => setLoading(false))
  }, [sessionId])

  // clear selection when switching views
  useEffect(() => { setSelectedSpan(null) }, [view])

  useWebSocket(useCallback((event) => {
    if (event.type === 'span.created') {
      const span = event.payload as Span
      if (span.session_id !== sessionId) return
      setAllSpans((prev) => {
        if (prev.some((s) => s.id === span.id)) return prev
        const next = [...prev, span]
        setRoots(buildTree(next))
        return next
      })
    }
    if (event.type === 'session.updated') {
      const sess = event.payload as Session
      if (sess.id !== sessionId) return
      setSession(sess)
    }
  }, [sessionId]))

  const sessionStartMs = session ? new Date(session.started_at).getTime() : 0
  const sessionDurationMs = session?.duration_ms ?? 1

  const statusColors: Record<string, string> = {
    completed: 'text-green-400',
    running: 'text-yellow-400',
    error: 'text-red-400',
  }

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 p-6">
      <div className="max-w-5xl mx-auto">
        <button
          onClick={onBack}
          className="text-gray-400 hover:text-gray-100 text-sm mb-6 flex items-center gap-1 transition-colors"
        >
          ← Sessions
        </button>

        {loading && <p className="text-gray-400 text-sm">Loading session…</p>}

        {error && (
          <div className="bg-red-900/30 border border-red-700 rounded-lg p-4 text-red-300 text-sm">
            {error}
          </div>
        )}

        {session && (
          <>
            {/* session header */}
            <div className="bg-gray-900 rounded-xl border border-gray-800 p-5 mb-6">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h1 className="text-lg font-bold font-mono">{session.root_span_name || session.trace_id}</h1>
                  <p className="text-gray-400 text-xs mt-1 font-mono">{session.trace_id}</p>
                </div>
                <span className={`text-sm font-medium shrink-0 ${statusColors[session.status] ?? 'text-gray-400'}`}>
                  {session.status}
                </span>
              </div>
              <div className="mt-4 grid grid-cols-3 gap-4 text-sm">
                <div>
                  <p className="text-gray-500 text-xs mb-1">Agent</p>
                  <p className="font-medium">{session.agent_name || '—'}</p>
                </div>
                <div>
                  <p className="text-gray-500 text-xs mb-1">Started</p>
                  <p className="font-medium">{new Date(session.started_at).toLocaleString()}</p>
                </div>
                <div>
                  <p className="text-gray-500 text-xs mb-1">Duration</p>
                  <p className="font-medium">{formatDuration(session.duration_ms)}</p>
                </div>
              </div>
            </div>

            {/* view toggle */}
            <div className="flex gap-1 mb-3">
              {(['diagram', 'waterfall'] as const).map((v) => (
                <button
                  key={v}
                  onClick={() => setView(v)}
                  className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors capitalize ${
                    view === v
                      ? 'bg-gray-700 text-gray-100'
                      : 'text-gray-500 hover:text-gray-300'
                  }`}
                >
                  {v}
                </button>
              ))}
            </div>

            {/* diagram view */}
            {view === 'diagram' && (
              <>
                <FlowDiagram
                  spans={allSpans}
                  selectedSpanId={selectedSpan?.id ?? null}
                  onSpanClick={setSelectedSpan}
                />
                {selectedSpan && (
                  <SpanDetailPanel span={selectedSpan} onClose={() => setSelectedSpan(null)} />
                )}
              </>
            )}

            {/* waterfall view */}
            {view === 'waterfall' && (
              <div className="bg-gray-900 rounded-xl border border-gray-800 overflow-hidden">
                <div className="px-4 py-3 border-b border-gray-800 text-xs text-gray-500 flex items-center gap-2">
                  <span className="w-56 shrink-0 pl-6">Span</span>
                  <span className="flex-1">Timeline</span>
                  <span className="w-14 text-right shrink-0">Duration</span>
                  <span className="w-5 shrink-0" />
                </div>
                <div className="py-2">
                  {roots.length === 0 ? (
                    <p className="text-center text-gray-500 text-sm py-8">No spans</p>
                  ) : (
                    roots.map((root) => (
                      <SpanRow
                        key={root.id}
                        node={root}
                        depth={0}
                        sessionStartMs={sessionStartMs}
                        sessionDurationMs={sessionDurationMs}
                      />
                    ))
                  )}
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
