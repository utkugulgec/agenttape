// Seed sends synthetic OTLP trace data to the local server for UI verification.
// Usage: go run ./cmd/seed
package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

const target = "http://localhost:8080/v1/traces"

func main() {
	sessions := []struct {
		name   string
		status tracepb.Status_StatusCode
		spans  []spanDef
	}{
		{
			name:   "agent.session",
			status: tracepb.Status_STATUS_CODE_OK,
			spans: []spanDef{
				{name: "agent.session", tool: "", ms: 4200, isRoot: true},
				{name: "tool.read", tool: "read", ms: 180, parent: "agent.session"},
				{name: "tool.read", tool: "read", ms: 95, parent: "agent.session"},
				{name: "llm.request", tool: "", ms: 1800, parent: "agent.session"},
				{name: "tool.bash", tool: "bash", ms: 340, parent: "agent.session"},
				{name: "tool.edit", tool: "edit", ms: 210, parent: "agent.session"},
				{name: "llm.request", tool: "", ms: 1200, parent: "agent.session"},
				{name: "tool.write", tool: "write", ms: 60, parent: "agent.session"},
			},
		},
		{
			name:   "agent.session",
			status: tracepb.Status_STATUS_CODE_OK,
			spans: []spanDef{
				{name: "agent.session", tool: "", ms: 9800, isRoot: true},
				{name: "llm.request", tool: "", ms: 2100, parent: "agent.session"},
				{name: "tool.bash", tool: "bash", ms: 5200, parent: "agent.session"},
				{name: "tool.bash", tool: "bash", ms: 420, parent: "tool.bash"},
				{name: "tool.bash", tool: "bash", ms: 380, parent: "tool.bash"},
				{name: "llm.request", tool: "", ms: 1900, parent: "agent.session"},
				{name: "tool.read", tool: "read", ms: 70, parent: "agent.session"},
			},
		},
		{
			name:   "agent.session",
			status: tracepb.Status_STATUS_CODE_ERROR,
			spans: []spanDef{
				{name: "agent.session", tool: "", ms: 2300, isRoot: true, errMsg: "context deadline exceeded"},
				{name: "llm.request", tool: "", ms: 1100, parent: "agent.session"},
				{name: "tool.bash", tool: "bash", ms: 950, parent: "agent.session", errMsg: "exit status 1"},
			},
		},
	}

	for i, s := range sessions {
		req := buildRequest(s.name, s.status, s.spans)
		if err := send(req); err != nil {
			log.Fatalf("session %d: %v", i+1, err)
		}
		fmt.Printf("✓ session %d sent (%d spans)\n", i+1, len(s.spans))
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Println("Done — refresh the UI at http://localhost:5173")
}

type spanDef struct {
	name   string
	tool   string
	ms     int
	parent string
	isRoot bool
	errMsg string
}

func buildRequest(rootName string, rootStatus tracepb.Status_StatusCode, defs []spanDef) *coltracepb.ExportTraceServiceRequest {
	traceID := randBytes(16)
	base := time.Now().Add(-time.Duration(totalMs(defs)) * time.Millisecond)

	// assign IDs and build a name→spanID map for parent lookup
	type built struct {
		def    spanDef
		spanID []byte
	}
	items := make([]built, len(defs))
	nameToID := map[string][]byte{}
	for i, d := range defs {
		id := randBytes(8)
		items[i] = built{d, id}
		if _, seen := nameToID[d.name]; !seen {
			nameToID[d.name] = id
		}
	}

	cursor := base
	spans := make([]*tracepb.Span, 0, len(items))
	for _, item := range items {
		d := item.def
		start := cursor
		end := start.Add(time.Duration(d.ms) * time.Millisecond)
		if !d.isRoot {
			cursor = end
		}

		code := tracepb.Status_STATUS_CODE_OK
		msg := ""
		if d.errMsg != "" {
			code = tracepb.Status_STATUS_CODE_ERROR
			msg = d.errMsg
		}

		span := &tracepb.Span{
			TraceId:           traceID,
			SpanId:            item.spanID,
			Name:              d.name,
			Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
			StartTimeUnixNano: uint64(start.UnixNano()),
			EndTimeUnixNano:   uint64(end.UnixNano()),
			Status:            &tracepb.Status{Code: code, Message: msg},
		}

		if d.parent != "" {
			span.ParentSpanId = nameToID[d.parent]
		}
		if d.isRoot {
			span.Status.Code = rootStatus
		}
		if d.tool != "" {
			span.Attributes = []*commonpb.KeyValue{
				strAttr("tool.name", d.tool),
				strAttr("tool.input", fmt.Sprintf("synthetic input for %s", d.tool)),
			}
		}

		spans = append(spans, span)
	}

	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						strAttr("service.name", "claude-code"),
						strAttr("service.version", "1.0.0"),
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{Spans: spans},
				},
			},
		},
	}
}

func send(req *coltracepb.ExportTraceServiceRequest) error {
	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := http.Post(target, "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func totalMs(defs []spanDef) int {
	total := 0
	for _, d := range defs {
		if !d.isRoot {
			total += d.ms
		}
	}
	return total
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
	return b
}

func strAttr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   k,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}},
	}
}
