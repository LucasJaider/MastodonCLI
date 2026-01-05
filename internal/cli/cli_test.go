package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"mastodoncli/internal/config"
)

func TestRunTimelineRejectsInvalidType(t *testing.T) {
	if err := runTimeline([]string{"--type", "nope"}); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestRunTimelineRejectsInvalidLimit(t *testing.T) {
	if err := runTimeline([]string{"--limit", "0"}); err == nil {
		t.Fatal("expected error for invalid limit")
	}
}

func TestRunTimelineLocalUsesPublicEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/timelines/public" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("local") != "true" {
			t.Fatalf("expected local=true")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	cfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)

	cfg := &config.Config{
		Instance:    server.URL,
		AccessToken: "token",
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	err := runTimeline([]string{"--limit", "1", "--type", "local"})
	if err != nil {
		t.Fatalf("runTimeline error: %v", err)
	}

	path, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("expected config dir to exist: %v", err)
	}
}
