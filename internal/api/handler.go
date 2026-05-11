package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/utkugulgec/agenttape/internal/storage"
)

type Handler struct {
	pool     *pgxpool.Pool
	sessions *storage.SessionRepo
	spans    *storage.SpanRepo
}

func NewHandler(pool *pgxpool.Pool, sessions *storage.SessionRepo, spans *storage.SpanRepo) *Handler {
	return &Handler{pool: pool, sessions: sessions, spans: spans}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	type healthResponse struct {
		Status string `json:"status"`
		DB     string `json:"db"`
	}

	if err := h.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "degraded", DB: "down"})
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", DB: "ok"})
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50, 200)
	offset := queryInt(r, "offset", 0, -1)

	sessions, err := h.sessions.List(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	type response struct {
		Sessions []*storage.Session `json:"sessions"`
		Limit    int                `json:"limit"`
		Offset   int                `json:"offset"`
	}
	writeJSON(w, http.StatusOK, response{Sessions: sessions, Limit: limit, Offset: offset})
}

func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	session, err := h.sessions.GetByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get session")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) ListSpans(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")

	// verify session exists so we return 404 rather than an empty list
	if _, err := h.sessions.GetByID(r.Context(), sessionID); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get session")
		return
	}

	spans, err := h.spans.ListBySession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list spans")
		return
	}

	type response struct {
		Spans []*storage.Span `json:"spans"`
	}
	writeJSON(w, http.StatusOK, response{Spans: spans})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// queryInt parses an integer query parameter. Falls back to def if missing or invalid.
// If max > 0, the value is clamped to max.
func queryInt(r *http.Request, key string, def, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return def
	}
	if max > 0 && v > max {
		return max
	}
	return v
}
