package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/utkugulgec/agenttape/internal/api"
	"github.com/utkugulgec/agenttape/internal/storage"
)

func TestIngestTraces(t *testing.T) {
	ctx := context.Background()

	pgCtr, pool := startPostgres(t, ctx)
	_ = pgCtr

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	sessions := storage.NewSessionRepo(pool)
	spans := storage.NewSpanRepo(pool)
	h := api.NewHandler(pool, sessions, spans)

	// fixed IDs so assertions are deterministic
	traceID := bytes.Repeat([]byte{0xab}, 16) // 32-char hex: abab...
	rootSpanID := bytes.Repeat([]byte{0x01}, 8)
	childSpanID := bytes.Repeat([]byte{0x02}, 8)

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	toNano := func(t time.Time) uint64 { return uint64(t.UnixNano()) }

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						strAttr("service.name", "test-agent"),
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							// child span arrives first (out-of-order)
							{
								TraceId:           traceID,
								SpanId:            childSpanID,
								ParentSpanId:      rootSpanID,
								Name:              "tool.bash.execute",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: toNano(base.Add(100 * time.Millisecond)),
								EndTimeUnixNano:   toNano(base.Add(400 * time.Millisecond)),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									strAttr("tool.name", "bash"),
									strAttr("command", "ls -la"),
								},
							},
							// root span arrives second
							{
								TraceId:           traceID,
								SpanId:            rootSpanID,
								Name:              "agent.session",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: toNano(base),
								EndTimeUnixNano:   toNano(base.Add(500 * time.Millisecond)),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									strAttr("agent.version", "1.0.0"),
								},
							},
						},
					},
				},
			},
		},
	}

	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	httpReq := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	rec := httptest.NewRecorder()

	h.IngestTraces(rec, httpReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-protobuf" {
		t.Errorf("Content-Type: want application/x-protobuf, got %q", ct)
	}
	var resp coltracepb.ExportTraceServiceResponse
	if err := proto.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// verify session was created and closed correctly
	traceIDHex := "abababababababababababababababababab" // 32 × ab... wait let me recount
	// 16 bytes of 0xab = "abababababababababababababababababab" — actually 32 chars
	traceIDHex = ""
	for range 16 {
		traceIDHex += "ab"
	}

	session, err := sessions.GetByTraceID(ctx, traceIDHex)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.AgentName != "test-agent" {
		t.Errorf("agent_name: want test-agent, got %q", session.AgentName)
	}
	if session.Status != "completed" {
		t.Errorf("status: want completed, got %q", session.Status)
	}
	if session.RootSpanName != "agent.session" {
		t.Errorf("root_span_name: want agent.session, got %q", session.RootSpanName)
	}
	if session.DurationMs == nil || *session.DurationMs != 500 {
		t.Errorf("duration_ms: want 500, got %v", session.DurationMs)
	}

	// verify both spans were inserted
	allSpans, err := spans.ListBySession(ctx, session.ID)
	if err != nil {
		t.Fatalf("list spans: %v", err)
	}
	if len(allSpans) != 2 {
		t.Fatalf("span count: want 2, got %d", len(allSpans))
	}

	// spans ordered by started_at ASC: root starts at base, child at base+100ms
	root := allSpans[0]
	child := allSpans[1]

	if child.Name != "tool.bash.execute" {
		t.Errorf("child name: want tool.bash.execute, got %q", child.Name)
	}
	if child.ParentSpanID == nil {
		t.Error("child span should have a parent_span_id")
	}
	if root.Name != "agent.session" {
		t.Errorf("root name: want agent.session, got %q", root.Name)
	}
	if root.ParentSpanID != nil {
		t.Errorf("root span should have no parent_span_id, got %q", *root.ParentSpanID)
	}
}

func TestIngestTraces_Idempotent(t *testing.T) {
	ctx := context.Background()

	pgCtr, pool := startPostgres(t, ctx)
	_ = pgCtr

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	sessions := storage.NewSessionRepo(pool)
	spans := storage.NewSpanRepo(pool)
	h := api.NewHandler(pool, sessions, spans)

	traceID := bytes.Repeat([]byte{0xcc}, 16)
	spanID := bytes.Repeat([]byte{0x01}, 8)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	toNano := func(t time.Time) uint64 { return uint64(t.UnixNano()) }

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{strAttr("service.name", "test-agent")},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            spanID,
								Name:              "agent.session",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: toNano(base),
								EndTimeUnixNano:   toNano(base.Add(time.Second)),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
							},
						},
					},
				},
			},
		},
	}

	body, _ := proto.Marshal(req)

	// send the same payload twice
	for i := range 2 {
		rec := httptest.NewRecorder()
		h.IngestTraces(rec, httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d — body: %s", i+1, rec.Code, rec.Body.String())
		}
	}

	traceIDHex := ""
	for range 16 {
		traceIDHex += "cc"
	}
	session, err := sessions.GetByTraceID(ctx, traceIDHex)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	allSpans, err := spans.ListBySession(ctx, session.ID)
	if err != nil {
		t.Fatalf("list spans: %v", err)
	}
	if len(allSpans) != 1 {
		t.Errorf("idempotency: want 1 span, got %d", len(allSpans))
	}
}

func strAttr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   k,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}},
	}
}
