import type { Span, NormalizedAttrs } from './types'

export interface SpanSemantic {
  label: string
  description: string
  icon: string
  category: 'llm' | 'tool' | 'agent' | 'unknown'
}

// ─── normalized field accessors ──────────────────────────────────────────────
// Always read from normalized_attrs first; fall back to raw attributes so that
// spans ingested before the migration still render correctly.

function n(span: Span): NormalizedAttrs | null {
  return span.normalized_attrs ?? null
}

function operationName(span: Span): string {
  return n(span)?.operation_name ?? (span.attributes?.['span.type'] === 'llm_request' ? 'chat' : '')
}

function toolName(span: Span): string {
  return n(span)?.tool_name ?? (span.attributes?.['tool_name'] as string) ?? span.name ?? ''
}

function model(span: Span): string {
  return n(span)?.request_model ?? (span.attributes?.['model'] as string) ?? ''
}

function finishReason(span: Span): string {
  return n(span)?.finish_reason ?? (span.attributes?.['stop_reason'] as string) ?? ''
}

// ─── helpers ─────────────────────────────────────────────────────────────────

function modelFamily(m: string): string {
  if (m.includes('haiku')) return 'Haiku'
  if (m.includes('sonnet')) return 'Sonnet'
  if (m.includes('opus')) return 'Opus'
  if (m.includes('gpt')) return 'GPT'
  if (m.includes('o1') || m.includes('o3')) return 'OpenAI'
  if (m.includes('gemini')) return 'Gemini'
  if (m.includes('mistral')) return 'Mistral'
  if (m.includes('llama')) return 'Llama'
  return m || 'LLM'
}

const TOOL_GLOSSARY: Record<string, { label: string; description: string; icon: string }> = {
  Bash:         { icon: '🖥',  label: 'Ran shell command',    description: 'Executed a command in the terminal' },
  Read:         { icon: '📄',  label: 'Read file',            description: 'Read the contents of a file' },
  Write:        { icon: '✏️', label: 'Wrote file',            description: 'Created or overwrote a file' },
  Edit:         { icon: '✏️', label: 'Edited file',           description: 'Applied a targeted edit to a file' },
  MultiEdit:    { icon: '✏️', label: 'Edited multiple files', description: 'Applied edits across several files' },
  Glob:         { icon: '🔍',  label: 'Searched files',       description: 'Found files matching a pattern' },
  Grep:         { icon: '🔍',  label: 'Searched code',        description: 'Searched for text across files' },
  LS:           { icon: '📁',  label: 'Listed directory',     description: 'Listed files in a directory' },
  WebFetch:     { icon: '🌐',  label: 'Fetched URL',          description: 'Retrieved content from a URL' },
  WebSearch:    { icon: '🌐',  label: 'Searched the web',     description: 'Performed a web search' },
  Agent:        { icon: '🤖',  label: 'Spawned sub-agent',    description: 'Delegated a task to a sub-agent' },
  TodoWrite:    { icon: '✅',  label: 'Updated task list',    description: 'Wrote to the task list' },
  TodoRead:     { icon: '✅',  label: 'Read task list',       description: 'Read the current task list' },
  NotebookRead: { icon: '📓',  label: 'Read notebook',        description: 'Read a Jupyter notebook' },
  NotebookEdit: { icon: '📓',  label: 'Edited notebook',      description: 'Modified a Jupyter notebook cell' },
}

// ─── main interpreter ────────────────────────────────────────────────────────

export function interpretSpan(span: Span): SpanSemantic {
  const op = operationName(span)
  const isError = span.status_code === 'error'

  // ── LLM call ────────────────────────────────────────────────────────────────
  if (op === 'chat' || op === 'generate_content' || op === 'text_completion') {
    const family = modelFamily(model(span))
    const reason = finishReason(span)

    if (isError) return { icon: '❌', label: `${family} failed`, description: 'The LLM call returned an error', category: 'llm' }
    if (reason === 'tool_calls' || reason === 'tool_use') return { icon: '🧠', label: `${family} chose a tool`, description: 'The model decided to invoke a tool to continue', category: 'llm' }
    if (reason === 'length' || reason === 'max_tokens') return { icon: '⚠️', label: `${family} hit token limit`, description: 'The response was cut off at the token limit', category: 'llm' }
    return { icon: '💬', label: `${family} responded`, description: 'The model generated a complete response', category: 'llm' }
  }

  // ── tool call ────────────────────────────────────────────────────────────────
  if (op === 'execute_tool' || op === '' ) {
    const tool = toolName(span)

    if (tool.startsWith('mcp__')) {
      const parts = tool.split('__')
      const server = parts[1] ?? 'mcp'
      const name = parts[2] ?? tool
      return { icon: '🔌', label: isError ? `MCP error: ${name}` : `MCP: ${name}`, description: `Called ${name} via the ${server} MCP server`, category: 'tool' }
    }

    const entry = TOOL_GLOSSARY[tool]
    if (entry) {
      return {
        icon: isError ? '❌' : entry.icon,
        label: isError ? `${entry.label} (failed)` : entry.label,
        description: isError ? `${entry.description} — returned an error` : entry.description,
        category: tool === 'Agent' ? 'agent' : 'tool',
      }
    }

    return { icon: isError ? '❌' : '⚙', label: isError ? `${tool} failed` : tool, description: isError ? 'Tool returned an error' : `Invoked tool: ${tool}`, category: 'unknown' }
  }

  return { icon: '⚙', label: op || span.name, description: `Operation: ${op || span.name}`, category: 'unknown' }
}
