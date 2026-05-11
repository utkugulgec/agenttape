import { useMemo } from 'react'
import {
  ReactFlow,
  Handle,
  Position,
  type NodeProps,
  type Node,
  type Edge,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { Span } from './types'
import { interpretSpan } from './semantic'

// ─── helpers ────────────────────────────────────────────────────────────────

function attr<T>(span: Span, key: string): T | undefined {
  return (span.attributes as Record<string, unknown>)?.[key] as T | undefined
}

function formatDuration(ms: number | null | undefined): string {
  if (ms == null) return '—'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

// ─── custom nodes ────────────────────────────────────────────────────────────

function StartNode() {
  return (
    <div className="flex flex-col items-center gap-1">
      <div className="h-10 w-10 rounded-full bg-gray-700 border-2 border-gray-500 flex items-center justify-center text-gray-300 text-xs">
        👤
      </div>
      <span className="text-gray-500 text-xs">User</span>
      <Handle type="source" position={Position.Right} className="!bg-gray-500" />
    </div>
  )
}

function EndNode() {
  return (
    <div className="flex flex-col items-center gap-1">
      <Handle type="target" position={Position.Left} className="!bg-gray-500" />
      <div className="h-10 w-10 rounded-full bg-gray-700 border-2 border-gray-500 flex items-center justify-center text-gray-300 text-lg">
        ✓
      </div>
      <span className="text-gray-500 text-xs">Done</span>
    </div>
  )
}

function LLMNode({ data }: NodeProps) {
  const span = data.span as Span
  const model = attr<string>(span, 'model') ?? 'LLM'
  const inputTokens = attr<number>(span, 'input_tokens')
  const outputTokens = attr<number>(span, 'output_tokens')
  const cacheRead = attr<number>(span, 'cache_read_tokens')
  const selected = data.selected as boolean
  const semantic = interpretSpan(span)

  return (
    <div
      className={`bg-blue-950 border rounded-xl p-3 w-48 shadow-lg cursor-pointer transition-colors ${
        selected ? 'border-blue-400 ring-1 ring-blue-400' : 'border-blue-700 hover:border-blue-500'
      }`}
    >
      <Handle type="target" position={Position.Left} className="!bg-blue-500" />
      <p className="text-gray-400 text-xs mb-1">{semantic.icon} {semantic.label}</p>
      <p className="text-blue-300 font-mono text-xs font-semibold truncate">{model}</p>
      <p className="text-gray-500 text-xs mt-1">{formatDuration(span.duration_ms)}</p>
      {inputTokens != null && (
        <p className="text-gray-400 text-xs mt-1.5">
          {inputTokens} in · {outputTokens} out
          {cacheRead ? <span className="text-gray-500"> · {cacheRead} cached</span> : null}
        </p>
      )}
      <Handle type="source" position={Position.Right} className="!bg-blue-500" />
    </div>
  )
}

function ToolNode({ data }: NodeProps) {
  const span = data.span as Span
  const toolName = attr<string>(span, 'tool_name') ?? span.name
  const isError = span.status_code === 'error'
  const selected = data.selected as boolean
  const semantic = interpretSpan(span)

  const borderColor = isError
    ? selected ? 'border-red-400 ring-1 ring-red-400' : 'border-red-700 hover:border-red-500'
    : selected ? 'border-purple-400 ring-1 ring-purple-400' : 'border-purple-700 hover:border-purple-500'
  const bgColor = isError ? 'bg-red-950' : 'bg-purple-950'
  const textColor = isError ? 'text-red-300' : 'text-purple-300'

  return (
    <div className={`${bgColor} border ${borderColor} rounded-xl p-3 w-40 shadow-lg cursor-pointer transition-colors`}>
      <Handle type="target" position={Position.Left} className="!bg-purple-500" />
      <p className="text-gray-400 text-xs mb-1">{semantic.icon} {semantic.label}</p>
      <p className={`${textColor} font-mono text-xs font-semibold truncate`}>{toolName}</p>
      <p className="text-gray-500 text-xs mt-1">{formatDuration(span.duration_ms)}</p>
      <Handle type="source" position={Position.Right} className="!bg-purple-500" />
    </div>
  )
}

const nodeTypes = { startNode: StartNode, endNode: EndNode, llmNode: LLMNode, toolNode: ToolNode }

// ─── layout builder ──────────────────────────────────────────────────────────

const NODE_WIDTH: Record<string, number> = { startNode: 60, endNode: 60, llmNode: 192, toolNode: 160 }
const GAP = 70
const ROW_WIDTH = 860   // matches ~max-w-5xl content area
const ROW_STEP = 160    // vertical distance between row baselines
const ROW_Y = 40        // vertical offset within each row

function buildFlow(
  spans: Span[],
  selectedId: string | null,
): { nodes: Node[]; edges: Edge[]; totalHeight: number } {
  const root = spans.find((s) => !s.parent_span_id)
  if (!root) return { nodes: [], edges: [], totalHeight: 0 }

  const children = spans
    .filter((s) => s.parent_span_id === root.span_id)
    .sort((a, b) => new Date(a.started_at).getTime() - new Date(b.started_at).getTime())

  const nodes: Node[] = []
  const edges: Edge[] = []
  let x = 0
  let row = 0

  nodes.push({ id: 'start', type: 'startNode', position: { x, y: row * ROW_STEP + ROW_Y }, data: {} })
  x += NODE_WIDTH.startNode + GAP

  let prevId = 'start'
  let prevRow = 0

  for (const span of children) {
    const spanType = attr<string>(span, 'span.type') ?? ''
    const type = spanType === 'llm_request' ? 'llmNode' : 'toolNode'
    const w = NODE_WIDTH[type]

    if (x + w > ROW_WIDTH) {
      row++
      x = 0
    }

    const crossRow = row !== prevRow
    nodes.push({
      id: span.id,
      type,
      position: { x, y: row * ROW_STEP + ROW_Y },
      data: { span, selected: span.id === selectedId },
    })
    edges.push({
      id: `e-${prevId}-${span.id}`,
      source: prevId,
      target: span.id,
      type: crossRow ? 'smoothstep' : undefined,
      animated: !crossRow && spanType === 'llm_request',
      style: { stroke: spanType === 'llm_request' ? '#3b82f6' : '#a855f7' },
    })

    prevId = span.id
    prevRow = row
    x += w + GAP
  }

  // end node
  if (x + NODE_WIDTH.endNode > ROW_WIDTH) {
    row++
    x = 0
  }
  nodes.push({ id: 'end', type: 'endNode', position: { x, y: row * ROW_STEP + ROW_Y }, data: {} })
  edges.push({
    id: 'e-last-end',
    source: prevId,
    target: 'end',
    type: prevRow !== row ? 'smoothstep' : undefined,
    style: { stroke: '#6b7280' },
  })

  return { nodes, edges, totalHeight: row * ROW_STEP + ROW_Y + 120 }
}

// ─── component ───────────────────────────────────────────────────────────────

interface Props {
  spans: Span[]
  selectedSpanId: string | null
  onSpanClick: (span: Span | null) => void
}

export default function FlowDiagram({ spans, selectedSpanId, onSpanClick }: Props) {
  const { nodes, edges, totalHeight } = useMemo(
    () => buildFlow(spans, selectedSpanId),
    [spans, selectedSpanId],
  )

  if (nodes.length === 0) {
    return <p className="text-center text-gray-500 text-sm py-16">No spans to diagram.</p>
  }

  return (
    <div style={{ height: Math.max(totalHeight, 180) }} className="bg-gray-950 rounded-xl">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView={false}
        defaultViewport={{ x: 20, y: 10, zoom: 1 }}
        proOptions={{ hideAttribution: true }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        zoomOnScroll={false}
        panOnDrag={false}
        onNodeClick={(_evt, node) => {
          const span = (node.data as { span?: Span }).span
          if (!span) return
          onSpanClick(selectedSpanId === span.id ? null : span)
        }}
      />
    </div>
  )
}
