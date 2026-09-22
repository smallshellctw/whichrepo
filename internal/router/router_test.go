package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smallshellctw/whichrepo/internal/gateway"
	"github.com/smallshellctw/whichrepo/internal/index"
	"github.com/smallshellctw/whichrepo/internal/model"
)

func TestRouteFallsBackWithoutAPIKey(t *testing.T) {
	store, err := index.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	projects := []model.Project{
		{Name: "payments-api", Path: "/repo/payments-api", Description: "Checkout and webhook API", Aliases: []string{"checkout"}, Content: "webhook retry timeout", IndexedAt: time.Now()},
		{Name: "webhook-worker", Path: "/repo/webhook-worker", Description: "Webhook background worker", Content: "webhook job processor", IndexedAt: time.Now()},
	}
	if err := store.ReplaceProjects(context.Background(), projects); err != nil {
		t.Fatal(err)
	}
	result, err := (Router{Store: store, Gateway: gateway.Client{}}).Route(context.Background(), "Add retry limits to checkout webhooks", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.PrimaryProject != "payments-api" {
		t.Fatalf("primary project = %q, candidates = %+v", result.PrimaryProject, result.Candidates)
	}
	if result.ProviderStatus != "disabled_missing_api_key" {
		t.Fatalf("provider status = %q", result.ProviderStatus)
	}
}

func TestRouteUsesJevAnswers(t *testing.T) {
	store, err := index.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	projects := []model.Project{
		{Name: "payments-api", Path: "/repo/payments-api", Description: "Checkout and webhook API", Aliases: []string{"checkout"}, Content: "webhook retry timeout", IndexedAt: time.Now()},
		{Name: "webhook-worker", Path: "/repo/webhook-worker", Description: "Webhook background worker", Aliases: []string{"jobs"}, Content: "webhook job retry", IndexedAt: time.Now()},
	}
	if err := store.ReplaceProjects(context.Background(), projects); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
          "model":"jev-1.13.0",
          "answers":{
            "primary_project":{"type":"choice","choice":"webhook-worker","probabilities":{"webhook-worker":0.8,"payments-api":0.2},"confidence":0.7},
            "need_clarification":{"type":"noul","noul":0.9},
            "cross_project":{"type":"noul","noul":0.7},
            "change_type":{"type":"choice","choice":"timeout","probabilities":{"timeout":0.9},"confidence":0.9},
            "requirement_completeness":{"type":"score","score":2.0,"confidence":0.8},
            "change_risk":{"type":"score","score":2.5,"confidence":0.7},
            "candidate_0_relevant":{"type":"noul","noul":0.9},
            "candidate_1_relevant":{"type":"noul","noul":0.8},
            "missing_target_scope":{"type":"noul","noul":0.9},
            "missing_expected_behavior":{"type":"noul","noul":0.2},
            "missing_acceptance_criteria":{"type":"noul","noul":0.7}
          },
          "usage":{"input_tokens":100,"output_tokens":20}
        }`))
	}))
	defer server.Close()

	result, err := (Router{Store: store, Gateway: gateway.Client{Provider: "jev-vercel", APIKey: "test", Endpoint: server.URL}}).Route(context.Background(), "Add retry limits to checkout webhooks", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "semantic_routed" || result.PrimaryProject != "webhook-worker" || result.ChangeType != "timeout" {
		t.Fatalf("unexpected route result: %+v", result)
	}
	if result.NeedsClarification == nil || !*result.NeedsClarification {
		t.Fatalf("expected clarification: %+v", result)
	}
	if len(result.MissingDimensions) != 2 {
		t.Fatalf("missing dimensions = %v", result.MissingDimensions)
	}
}

func TestSanitizeEvidenceRedactsSecrets(t *testing.T) {
	lines := sanitizeEvidence([]string{"password: hello", "redis_timeout: 10"})
	if strings.Contains(strings.Join(lines, " "), "hello") {
		t.Fatalf("secret was not redacted: %v", lines)
	}
	if !strings.Contains(strings.Join(lines, " "), "redis_timeout") {
		t.Fatalf("non-secret evidence was removed: %v", lines)
	}
}
