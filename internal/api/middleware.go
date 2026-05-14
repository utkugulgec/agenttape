package api

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"time"
)

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// WebSocket upgrades must not be wrapped — skip logging for them.
		if r.Header.Get("Upgrade") == "websocket" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration", time.Since(start).String(),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack lets the WebSocket upgrader take over the connection.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rw.ResponseWriter.(http.Hijacker).Hijack()
}
