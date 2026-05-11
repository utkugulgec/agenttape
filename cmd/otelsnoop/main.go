// otelsnoop is a dev tool that listens for OTLP/HTTP requests (JSON or protobuf)
// and writes each payload to /tmp/otelsnoop-<n>.json. Not for production use.
//
// Usage:
//
//	go run ./cmd/otelsnoop
//
// Then run claude with:
//
//	CLAUDE_CODE_ENABLE_TELEMETRY=1 \
//	CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1 \
//	OTEL_TRACES_EXPORTER=otlp \
//	OTEL_EXPORTER_OTLP_PROTOCOL=http/json \
//	OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:9999 \
//	claude
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

var requestCount atomic.Int32

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)

	addr := ":9999"
	log.Printf("otelsnoop listening on %s", addr)
	log.Printf("payloads written to /tmp/otelsnoop-<n>.json")
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	n := requestCount.Add(1)
	ct := r.Header.Get("Content-Type")
	outDir := os.Getenv("HOME") + "/otelsnoop"
	os.MkdirAll(outDir, 0o755) //nolint:errcheck

	fmt.Printf("\n─────────────────────────────────────────\n")
	fmt.Printf("  %s  %s %s\n", time.Now().Format("15:04:05.000"), r.Method, r.URL.Path)
	fmt.Printf("  Content-Type: %s  (%d bytes)\n", ct, len(body))

	if ct == "application/json" {
		path := fmt.Sprintf("%s/%03d.json", outDir, n)
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			fmt.Printf("  [invalid JSON: %v]\n", err)
		} else {
			pretty, _ := json.MarshalIndent(v, "", "  ")
			if err := os.WriteFile(path, pretty, 0o644); err != nil {
				fmt.Printf("  [write error: %v]\n", err)
			} else {
				fmt.Printf("  → written to %s\n", path)
			}
		}
	} else if ct == "application/x-protobuf" {
		fmt.Printf("  [protobuf — switch to http/json to see readable output]\n")
	} else {
		path := fmt.Sprintf("%s/%03d.txt", outDir, n)
		os.WriteFile(path, body, 0o644) //nolint:errcheck
		fmt.Printf("  → written to %s\n", path)
	}

	fmt.Printf("─────────────────────────────────────────\n")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{}`)
}
