# Flow Diagram View — Design Notes

## Concept

A second visualization mode for session detail, alongside the existing waterfall view.
Shows the agent interaction as a directed flow graph — nodes are actors, edges are
the data flowing between them.

## Layout

Linear, left to right, time-ordered. Reads like a story.

```
[User] ──► [claude-sonnet-4-6] ──► [Bash] ──► [claude-sonnet-4-6] ──► [Response]
                874ms                 71ms            3.5s
```

## Node Types

**LLM node** (rectangle)
- Model name inside (from `attributes.model`)
- Token counts below: `3 in / 114 out / 11.9k cached`
- Duration label

**Tool node** (gear or hexagon)
- Tool name inside (from `attributes.tool_name`)
- Duration label

**Start / End nodes** (small circles)
- Entry point (user prompt in)
- Exit point (response out)

## Edges

- Directed arrows left to right
- `stop_reason: "tool_use"` on an LLM span → outgoing edge leads to a tool node
- Tool node → next LLM node (continuation)

## Data Mapping

Built from the flat span list returned by `GET /sessions/{id}/spans`:
- Filter to direct children of the root interaction span
- Order by `started_at ASC`
- Map `span.type === "llm_request"` → LLM node
- Map `span.type === "tool"` → Tool node
- Connect sequentially with directed edges

## UI Placement

Toggle at the top of `SessionDetail` between two modes:
- **Waterfall** — existing span tree with timeline bars (current)
- **Diagram** — new React Flow graph

Same API data, two representations.

## Library

**React Flow** — handles custom node shapes, animated arrows, zoom/pan.
Install: `npm install @xyflow/react`
Two custom node components: `LLMNode`, `ToolNode`.
