// Package normalizer translates vendor-specific OTEL span attributes into the
// OpenTelemetry GenAI semantic conventions, giving the rest of the system a
// single stable schema to work with regardless of which agent produced the trace.
package normalizer

import "strings"

// Schema identifies which attribute vocabulary was detected on a span.
type Schema string

const (
	SchemaClaudeCode Schema = "claude_code" // Claude Code custom attributes
	SchemaGenAI      Schema = "genai_spec"  // OpenTelemetry GenAI semantic conventions
	SchemaUnknown    Schema = "unknown"
)

// NormalizedAttrs is the canonical internal representation of a span's semantic
// meaning. All fields use GenAI spec naming and units (tokens as int64, TTFT in ms).
type NormalizedAttrs struct {
	Schema Schema `json:"schema"`

	// operation classification
	OperationName string `json:"operation_name,omitempty"` // "chat" | "execute_tool" | "embeddings"
	ProviderName  string `json:"provider_name,omitempty"`  // "anthropic" | "openai" | ...

	// model
	RequestModel  string `json:"request_model,omitempty"`
	ResponseModel string `json:"response_model,omitempty"`

	// token usage
	InputTokens         *int64 `json:"input_tokens,omitempty"`
	OutputTokens        *int64 `json:"output_tokens,omitempty"`
	CacheReadTokens     *int64 `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens *int64 `json:"cache_creation_tokens,omitempty"`

	// latency
	TTFTMs *float64 `json:"ttft_ms,omitempty"` // time to first token, always in ms

	// completion
	FinishReason string `json:"finish_reason,omitempty"` // "stop" | "tool_calls" | "length"

	// tool execution
	ToolName string `json:"tool_name,omitempty"`
}

// Normalize detects the schema of raw span attributes and returns a NormalizedAttrs.
// The raw attributes map is never mutated.
func Normalize(attrs map[string]any) NormalizedAttrs {
	switch detect(attrs) {
	case SchemaClaudeCode:
		return fromClaudeCode(attrs)
	case SchemaGenAI:
		return fromGenAI(attrs)
	default:
		return NormalizedAttrs{Schema: SchemaUnknown}
	}
}

// ─── detection ───────────────────────────────────────────────────────────────

func detect(attrs map[string]any) Schema {
	if _, ok := attrs["gen_ai.operation.name"]; ok {
		return SchemaGenAI
	}
	if _, ok := attrs["span.type"]; ok {
		return SchemaClaudeCode
	}
	return SchemaUnknown
}

// ─── Claude Code adapter ─────────────────────────────────────────────────────

func fromClaudeCode(a map[string]any) NormalizedAttrs {
	n := NormalizedAttrs{Schema: SchemaClaudeCode}

	spanType, _ := a["span.type"].(string)
	switch spanType {
	case "llm_request":
		n.OperationName = "chat"
	case "tool_use":
		n.OperationName = "execute_tool"
	}

	if model, ok := a["model"].(string); ok {
		n.RequestModel = model
		n.ResponseModel = model
		n.ProviderName = providerFromModel(model)
	}

	n.InputTokens = int64Ptr(a, "input_tokens")
	n.OutputTokens = int64Ptr(a, "output_tokens")
	n.CacheReadTokens = int64Ptr(a, "cache_read_tokens")
	n.CacheCreationTokens = int64Ptr(a, "cache_creation_tokens")

	if ttft := int64Ptr(a, "ttft_ms"); ttft != nil {
		v := float64(*ttft)
		n.TTFTMs = &v
	}

	if stop, ok := a["stop_reason"].(string); ok {
		n.FinishReason = mapStopReason(stop)
	}

	if tool, ok := a["tool_name"].(string); ok {
		n.ToolName = tool
	}

	return n
}

// mapStopReason converts Claude Code stop reasons to GenAI spec finish reasons.
func mapStopReason(r string) string {
	switch r {
	case "end_turn":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		return r
	}
}

// ─── GenAI spec adapter ───────────────────────────────────────────────────────

func fromGenAI(a map[string]any) NormalizedAttrs {
	n := NormalizedAttrs{Schema: SchemaGenAI}

	n.OperationName, _ = a["gen_ai.operation.name"].(string)
	n.ProviderName, _ = a["gen_ai.provider.name"].(string)
	n.RequestModel, _ = a["gen_ai.request.model"].(string)
	n.ResponseModel, _ = a["gen_ai.response.model"].(string)

	n.InputTokens = int64Ptr(a, "gen_ai.usage.input_tokens")
	n.OutputTokens = int64Ptr(a, "gen_ai.usage.output_tokens")
	n.CacheReadTokens = int64Ptr(a, "gen_ai.usage.cache_read.input_tokens")
	n.CacheCreationTokens = int64Ptr(a, "gen_ai.usage.cache_creation.input_tokens")

	// GenAI spec reports TTFT in seconds; convert to ms for consistency
	if v, ok := a["gen_ai.response.time_to_first_chunk"].(float64); ok {
		ms := v * 1000
		n.TTFTMs = &ms
	}

	// finish_reasons is an array; take the first entry
	if reasons, ok := a["gen_ai.response.finish_reasons"].([]any); ok && len(reasons) > 0 {
		if r, ok := reasons[0].(string); ok {
			n.FinishReason = r
		}
	}

	n.ToolName, _ = a["gen_ai.tool.name"].(string)

	return n
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func int64Ptr(a map[string]any, key string) *int64 {
	v, ok := a[key]
	if !ok || v == nil {
		return nil
	}
	switch n := v.(type) {
	case int64:
		return &n
	case float64:
		i := int64(n)
		return &i
	case int:
		i := int64(n)
		return &i
	}
	return nil
}

func providerFromModel(model string) string {
	model = strings.ToLower(model)
	switch {
	case strings.Contains(model, "claude"):
		return "anthropic"
	case strings.Contains(model, "gpt") || strings.Contains(model, "o1") || strings.Contains(model, "o3"):
		return "openai"
	case strings.Contains(model, "gemini"):
		return "google"
	case strings.Contains(model, "mistral"):
		return "mistral"
	case strings.Contains(model, "llama"):
		return "meta"
	default:
		return ""
	}
}
