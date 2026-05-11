import { useEffect, useState } from 'react'
import { api } from './api'
import type { Session } from './types'

function formatDuration(ms: number | null): string {
  if (ms === null) return '—'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  const m = Math.floor(ms / 60_000)
  const s = Math.floor((ms % 60_000) / 1000)
  return `${m}m ${s}s`
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString()
}

function StatusBadge({ status }: { status: Session['status'] }) {
  const colors = {
    completed: 'bg-green-500',
    running: 'bg-yellow-400 animate-pulse',
    error: 'bg-red-500',
  }
  return <span className={`h-2.5 w-2.5 rounded-full shrink-0 ${colors[status]}`} />
}

interface Props {
  onSelect: (id: string) => void
}

export default function SessionList({ onSelect }: Props) {
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.sessions
      .list()
      .then((r) => setSessions(r.sessions ?? []))
      .catch((e: unknown) => setError(String(e)))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 p-6">
      <div className="max-w-5xl mx-auto">
        <h1 className="text-2xl font-bold mb-6 tracking-tight">AgentTape</h1>

        {loading && <p className="text-gray-400 text-sm">Loading sessions…</p>}

        {error && (
          <div className="bg-red-900/30 border border-red-700 rounded-lg p-4 text-red-300 text-sm">
            {error}
          </div>
        )}

        {!loading && !error && sessions.length === 0 && (
          <div className="text-center py-24 text-gray-500 text-sm">
            No sessions yet. Point your OTEL exporter at{' '}
            <code className="font-mono text-gray-400">POST /v1/traces</code> to get started.
          </div>
        )}

        {sessions.length > 0 && (
          <div className="rounded-xl border border-gray-800 overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-900 text-gray-400 text-xs uppercase tracking-wider">
                <tr>
                  <th className="px-4 py-3 text-left w-8" />
                  <th className="px-4 py-3 text-left">Agent</th>
                  <th className="px-4 py-3 text-left">Root span</th>
                  <th className="px-4 py-3 text-left">Started</th>
                  <th className="px-4 py-3 text-right">Duration</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800">
                {sessions.map((s) => (
                  <tr
                    key={s.id}
                    onClick={() => onSelect(s.id)}
                    className="hover:bg-gray-900 cursor-pointer transition-colors"
                  >
                    <td className="px-4 py-3">
                      <StatusBadge status={s.status} />
                    </td>
                    <td className="px-4 py-3 font-medium">
                      {s.agent_name || <span className="text-gray-500 italic">unknown</span>}
                    </td>
                    <td className="px-4 py-3 text-gray-400 font-mono text-xs">
                      {s.root_span_name || <span className="text-gray-600">—</span>}
                    </td>
                    <td className="px-4 py-3 text-gray-400">{formatTime(s.started_at)}</td>
                    <td className="px-4 py-3 text-right tabular-nums text-gray-300">
                      {formatDuration(s.duration_ms)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
