package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/utkugulgec/agenttape/internal/api"
	"github.com/utkugulgec/agenttape/internal/storage"
)

func TestHealth_OK(t *testing.T) {
	ctx := context.Background()

	_, pool := startPostgres(t, ctx)

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	h := api.NewHandler(pool, storage.NewSessionRepo(pool), storage.NewSpanRepo(pool))
	srv := httptest.NewServer(http.HandlerFunc(h.Health))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status: want %q, got %q", "ok", body["status"])
	}
	if body["db"] != "ok" {
		t.Errorf("db: want %q, got %q", "ok", body["db"])
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	pattern := filepath.Join("..", "..", "migrations", "*.up.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		var stmts []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "--") {
				stmts = append(stmts, line)
			}
		}
		if len(stmts) == 0 {
			continue
		}
		if _, err := pool.Exec(ctx, strings.Join(stmts, "\n")); err != nil {
			return fmt.Errorf("exec %s: %w", f, err)
		}
	}
	return nil
}
