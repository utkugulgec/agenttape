package normalizer_test

import (
	"testing"

	"github.com/utkugulgec/agenttape/internal/normalizer"
)

// ─── Claude Code ─────────────────────────────────────────────────────────────

func TestNormalize_ClaudeCode_LLMResponded(t *testing.T) {
	attrs := map[string]any{
		"span.type":           "llm_request",
		"model":               "claude-sonnet-4-6",
		"input_tokens":        int64(500),
		"output_tokens":       int64(120),
		"cache_read_tokens":   int64(11930),
		"cache_creation_tokens": int64(0),
		"ttft_ms":             int64(340),
		"stop_reason":         "end_turn",
	}

	got := normalizer.Normalize(attrs)

	assertEqual(t, string(normalizer.SchemaClaudeCode), string(got.Schema))
	assertEqual(t, "chat", got.OperationName)
	assertEqual(t, "anthropic", got.ProviderName)
	assertEqual(t, "claude-sonnet-4-6", got.RequestModel)
	assertInt64(t, 500, got.InputTokens)
	assertInt64(t, 120, got.OutputTokens)
	assertInt64(t, 11930, got.CacheReadTokens)
	assertFloat64(t, 340, got.TTFTMs)
	assertEqual(t, "stop", got.FinishReason)
	assertEqual(t, "", got.ToolName)
}

func TestNormalize_ClaudeCode_ToolUse(t *testing.T) {
	attrs := map[string]any{
		"span.type":   "llm_request",
		"model":       "claude-sonnet-4-6",
		"input_tokens": int64(300),
		"output_tokens": int64(40),
		"stop_reason": "tool_use",
	}

	got := normalizer.Normalize(attrs)

	assertEqual(t, "chat", got.OperationName)
	assertEqual(t, "tool_calls", got.FinishReason)
}

func TestNormalize_ClaudeCode_MaxTokens(t *testing.T) {
	attrs := map[string]any{
		"span.type":   "llm_request",
		"model":       "claude-haiku-4-5",
		"stop_reason": "max_tokens",
	}

	got := normalizer.Normalize(attrs)

	assertEqual(t, "length", got.FinishReason)
	assertEqual(t, "anthropic", got.ProviderName)
}

func TestNormalize_ClaudeCode_Tool(t *testing.T) {
	attrs := map[string]any{
		"span.type": "tool_use",
		"tool_name": "Bash",
	}

	got := normalizer.Normalize(attrs)

	assertEqual(t, string(normalizer.SchemaClaudeCode), string(got.Schema))
	assertEqual(t, "execute_tool", got.OperationName)
	assertEqual(t, "Bash", got.ToolName)
}

// ─── GenAI spec ──────────────────────────────────────────────────────────────

func TestNormalize_GenAI_Chat(t *testing.T) {
	attrs := map[string]any{
		"gen_ai.operation.name":               "chat",
		"gen_ai.provider.name":                "openai",
		"gen_ai.request.model":                "gpt-4o",
		"gen_ai.response.model":               "gpt-4o-2024-08-06",
		"gen_ai.usage.input_tokens":           int64(800),
		"gen_ai.usage.output_tokens":          int64(200),
		"gen_ai.usage.cache_read.input_tokens": int64(400),
		"gen_ai.response.time_to_first_chunk": float64(0.45),
		"gen_ai.response.finish_reasons":      []any{"stop"},
	}

	got := normalizer.Normalize(attrs)

	assertEqual(t, string(normalizer.SchemaGenAI), string(got.Schema))
	assertEqual(t, "chat", got.OperationName)
	assertEqual(t, "openai", got.ProviderName)
	assertEqual(t, "gpt-4o", got.RequestModel)
	assertEqual(t, "gpt-4o-2024-08-06", got.ResponseModel)
	assertInt64(t, 800, got.InputTokens)
	assertInt64(t, 200, got.OutputTokens)
	assertInt64(t, 400, got.CacheReadTokens)
	assertFloat64(t, 450, got.TTFTMs) // 0.45s → 450ms
	assertEqual(t, "stop", got.FinishReason)
}

func TestNormalize_GenAI_ToolCalls(t *testing.T) {
	attrs := map[string]any{
		"gen_ai.operation.name":          "chat",
		"gen_ai.provider.name":           "openai",
		"gen_ai.request.model":           "gpt-4o",
		"gen_ai.response.finish_reasons": []any{"tool_calls"},
	}

	got := normalizer.Normalize(attrs)

	assertEqual(t, "tool_calls", got.FinishReason)
}

func TestNormalize_GenAI_ExecuteTool(t *testing.T) {
	attrs := map[string]any{
		"gen_ai.operation.name": "execute_tool",
		"gen_ai.tool.name":      "web_search",
	}

	got := normalizer.Normalize(attrs)

	assertEqual(t, string(normalizer.SchemaGenAI), string(got.Schema))
	assertEqual(t, "execute_tool", got.OperationName)
	assertEqual(t, "web_search", got.ToolName)
}

// ─── unknown schema ───────────────────────────────────────────────────────────

func TestNormalize_Unknown(t *testing.T) {
	attrs := map[string]any{
		"some.custom.key": "value",
	}

	got := normalizer.Normalize(attrs)

	assertEqual(t, string(normalizer.SchemaUnknown), string(got.Schema))
	assertEqual(t, "", got.OperationName)
}

func TestNormalize_Empty(t *testing.T) {
	got := normalizer.Normalize(map[string]any{})

	assertEqual(t, string(normalizer.SchemaUnknown), string(got.Schema))
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func assertEqual(t *testing.T, want, got string) {
	t.Helper()
	if want != got {
		t.Errorf("want %q, got %q", want, got)
	}
}

func assertInt64(t *testing.T, want int64, got *int64) {
	t.Helper()
	if got == nil {
		t.Errorf("want %d, got nil", want)
		return
	}
	if want != *got {
		t.Errorf("want %d, got %d", want, *got)
	}
}

func assertFloat64(t *testing.T, want float64, got *float64) {
	t.Helper()
	if got == nil {
		t.Errorf("want %g, got nil", want)
		return
	}
	if want != *got {
		t.Errorf("want %g, got %g", want, *got)
	}
}
