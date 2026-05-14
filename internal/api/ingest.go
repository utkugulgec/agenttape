package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/utkugulgec/agenttape/internal/normalizer"
	"github.com/utkugulgec/agenttape/internal/storage"
)

const maxBodyBytes = 4 << 20 // 4 MB

func (h *Handler) IngestTraces(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var req coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "failed to decode OTLP request")
		return
	}

	ctx := r.Context()

	for _, rs := range req.ResourceSpans {
		agentName := resourceAttr(rs.GetResource().GetAttributes(), "service.name")
		resourceAttrs, _ := json.Marshal(attrsToMap(rs.GetResource().GetAttributes()))

		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				traceID := hex.EncodeToString(span.TraceId)
				spanID := hex.EncodeToString(span.SpanId)
				isRoot := len(span.ParentSpanId) == 0

				// get or create the session for this trace
				session, err := h.sessions.GetByTraceID(ctx, traceID)
				if errors.Is(err, pgx.ErrNoRows) {
					session = &storage.Session{
						TraceID:    traceID,
						AgentName:  agentName,
						Status:     "running",
						Attributes: resourceAttrs,
						StartedAt:  time.Unix(0, int64(span.StartTimeUnixNano)).UTC(),
					}
					if err := h.sessions.Insert(ctx, session); err != nil {
						writeError(w, http.StatusInternalServerError, "failed to create session")
						return
					}
				} else if err != nil {
					writeError(w, http.StatusInternalServerError, "failed to get session")
					return
				}

				var parentSpanID *string
				if !isRoot {
					pid := hex.EncodeToString(span.ParentSpanId)
					parentSpanID = &pid
				}

				startedAt := time.Unix(0, int64(span.StartTimeUnixNano)).UTC()
				var endedAt *time.Time
				var durationMs *int64
				if span.EndTimeUnixNano > 0 {
					t := time.Unix(0, int64(span.EndTimeUnixNano)).UTC()
					endedAt = &t
					d := t.Sub(startedAt).Milliseconds()
					durationMs = &d
				}

				rawAttrs := attrsToMap(span.Attributes)
				spanAttrs, _ := json.Marshal(rawAttrs)
				normalized, _ := json.Marshal(normalizer.Normalize(rawAttrs))
				spanEvents, _ := json.Marshal(convertEvents(span.Events))

				s := &storage.Span{
					SpanID:          spanID,
					TraceID:         traceID,
					ParentSpanID:    parentSpanID,
					SessionID:       session.ID,
					Name:            span.Name,
					Kind:            kindString(span.Kind),
					StatusCode:      statusString(span.GetStatus().GetCode()),
					StatusMessage:   span.GetStatus().GetMessage(),
					Attributes:      spanAttrs,
					NormalizedAttrs: normalized,
					Events:          spanEvents,
					StartedAt:       startedAt,
					EndedAt:         endedAt,
					DurationMs:      durationMs,
				}
				if err := h.spans.Insert(ctx, s); err != nil {
					writeError(w, http.StatusInternalServerError, "failed to insert span")
					return
				}
				h.hub.Broadcast(EventSpanCreated, s)

				// when the root span arrives with an end time, close the session
				if isRoot && endedAt != nil {
					status := "completed"
					if span.GetStatus().GetCode() == tracepb.Status_STATUS_CODE_ERROR {
						status = "error"
					}
					if err := h.sessions.UpdateFromRootSpan(ctx, traceID, span.Name, status, endedAt, durationMs); err != nil {
						writeError(w, http.StatusInternalServerError, "failed to update session")
						return
					}
					if sess, err := h.sessions.GetByTraceID(ctx, traceID); err == nil {
						h.hub.Broadcast(EventSessionUpdated, sess)
					}
				}
			}
		}
	}

	out, _ := proto.Marshal(&coltracepb.ExportTraceServiceResponse{})
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(out) //nolint:errcheck
}

func resourceAttr(attrs []*commonpb.KeyValue, key string) string {
	for _, kv := range attrs {
		if kv.Key == key {
			if s, ok := kv.Value.Value.(*commonpb.AnyValue_StringValue); ok {
				return s.StringValue
			}
		}
	}
	return ""
}

func attrsToMap(attrs []*commonpb.KeyValue) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, kv := range attrs {
		m[kv.Key] = anyValue(kv.Value)
	}
	return m
}

func anyValue(v *commonpb.AnyValue) any {
	if v == nil {
		return nil
	}
	switch val := v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_BoolValue:
		return val.BoolValue
	case *commonpb.AnyValue_IntValue:
		return val.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return val.DoubleValue
	case *commonpb.AnyValue_ArrayValue:
		arr := make([]any, 0, len(val.ArrayValue.GetValues()))
		for _, item := range val.ArrayValue.GetValues() {
			arr = append(arr, anyValue(item))
		}
		return arr
	case *commonpb.AnyValue_KvlistValue:
		return attrsToMap(val.KvlistValue.GetValues())
	default:
		return nil
	}
}

func convertEvents(events []*tracepb.Span_Event) []map[string]any {
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, map[string]any{
			"name":       e.Name,
			"timestamp":  time.Unix(0, int64(e.TimeUnixNano)).UTC(),
			"attributes": attrsToMap(e.Attributes),
		})
	}
	return out
}

func kindString(k tracepb.Span_SpanKind) string {
	switch k {
	case tracepb.Span_SPAN_KIND_SERVER:
		return "server"
	case tracepb.Span_SPAN_KIND_CLIENT:
		return "client"
	case tracepb.Span_SPAN_KIND_PRODUCER:
		return "producer"
	case tracepb.Span_SPAN_KIND_CONSUMER:
		return "consumer"
	default:
		return "internal"
	}
}

func statusString(c tracepb.Status_StatusCode) string {
	switch c {
	case tracepb.Status_STATUS_CODE_OK:
		return "ok"
	case tracepb.Status_STATUS_CODE_ERROR:
		return "error"
	default:
		return "unset"
	}
}
