import type { Span } from './types'

export interface SpanSemantic {
  label: string
  description: string
  icon: string
  category: 'llm' | 'tool' | 'agent' | 'unknown'
}

function attr<T>(span: Span, key: string): T | undefined {
  return (span.attributes as Record<string, unknown>)?.[key] as T | undefined
}

function modelFamily(model: string): string {
  if (model.includes('haiku')) return 'Haiku'
  if (model.includes('sonnet')) return 'Sonnet'
  if (model.includes('opus')) return 'Opus'
  return model
}

const TOOL_GLOSSARY: Record<string, { label: string; description: string; icon: string }> = {
  Bash:        { icon: '🖥',  label: 'Ran shell command',    description: 'Executed a command in the terminal' },
  Read:        { icon: '📄',  label: 'Read file',            description: 'Read the contents of a file' },
  Write:       { icon: '✏️', label: 'Wrote file',            description: 'Created or overwrote a file' },
  Edit:        { icon: '✏️', label: 'Edited file',           description: 'Applied a targeted edit to a file' },
  MultiEdit:   { icon: '✏️', label: 'Edited multiple files', description: 'Applied edits across several files' },
  Glob:        { icon: '🔍',  label: 'Searched files',       description: 'Found files matching a pattern' },
  Grep:        { icon: '🔍',  label: 'Searched code',        description: 'Searched for text across files' },
  LS:          { icon: '📁',  label: 'Listed directory',     description: 'Listed files in a directory' },
  WebFetch:    { icon: '🌐',  label: 'Fetched URL',          description: 'Retrieved content from a URL' },
  WebSearch:   { icon: '🌐',  label: 'Searched the web',     description: 'Performed a web search' },
  Agent:       { icon: '🤖',  label: 'Spawned sub-agent',    description: 'Delegated a task to a sub-agent' },
  TodoWrite:   { icon: '✅',  label: 'Updated task list',    description: 'Wrote to the task list' },
  TodoRead:    { icon: '✅',  label: 'Read task list',       description: 'Read the current task list' },
  NotebookRead:{ icon: '📓',  label: 'Read notebook',        description: 'Read a Jupyter notebook' },
  NotebookEdit:{ icon: '📓',  label: 'Edited notebook',      description: 'Modified a Jupyter notebook cell' },
}

export function interpretSpan(span: Span): SpanSemantic {
  const spanType = attr<string>(span, 'span.type') ?? ''
  const isError = span.status_code === 'error'

  // ── LLM call ──────────────────────────────────────────────────────────────
  if (spanType === 'llm_request') {
    const model = attr<string>(span, 'model') ?? 'LLM'
    const stopReason = attr<string>(span, 'stop_reason')
    const family = modelFamily(model)

    if (isError) {
      return {
        icon: '❌',
        label: `${family} failed`,
        description: 'The LLM call returned an error',
        category: 'llm',
      }
    }

    if (stopReason === 'tool_use') {
      return {
        icon: '🧠',
        label: `${family} chose a tool`,
        description: 'The model decided to invoke a tool to continue',
        category: 'llm',
      }
    }

    if (stopReason === 'max_tokens') {
      return {
        icon: '⚠️',
        label: `${family} hit token limit`,
        description: 'The response was cut off at the token limit',
        category: 'llm',
      }
    }

    // end_turn or unset
    return {
      icon: '💬',
      label: `${family} responded`,
      description: 'The model generated a complete response',
      category: 'llm',
    }
  }

  // ── tool call ─────────────────────────────────────────────────────────────
  const toolName = attr<string>(span, 'tool_name') ?? span.name ?? ''

  // MCP tools: tool_name like "mcp__server__toolname"
  if (toolName.startsWith('mcp__')) {
    const parts = toolName.split('__')
    const server = parts[1] ?? 'mcp'
    const tool = parts[2] ?? toolName
    return {
      icon: '🔌',
      label: isError ? `MCP error: ${tool}` : `MCP: ${tool}`,
      description: `Called ${tool} via the ${server} MCP server`,
      category: 'tool',
    }
  }

  const entry = TOOL_GLOSSARY[toolName]
  if (entry) {
    return {
      icon: isError ? '❌' : entry.icon,
      label: isError ? `${entry.label} (failed)` : entry.label,
      description: isError ? `${entry.description} — returned an error` : entry.description,
      category: toolName === 'Agent' ? 'agent' : 'tool',
    }
  }

  // fallback
  return {
    icon: isError ? '❌' : '⚙',
    label: isError ? `${toolName} failed` : toolName,
    description: isError ? `Tool returned an error` : `Invoked tool: ${toolName}`,
    category: 'unknown',
  }
}
