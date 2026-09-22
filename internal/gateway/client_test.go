package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientEvaluate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Model != DefaultModel {
			t.Fatalf("model = %q", request.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"primary_project":{"type":"choice","choice":"payments-api","probabilities":{"payments-api":0.9,"webhook-worker":0.1},"confidence":0.8}},"usage":{"input_tokens":42,"output_tokens":8}}`))
	}))
	defer server.Close()

	client := Client{APIKey: "test-key", Endpoint: server.URL}
	response, err := client.Evaluate(context.Background(), map[string]any{"requirement": "test"}, map[string]Question{
		"primary_project": {Type: "choice", Instructions: "pick", Criteria: map[string]any{"payments-api": nil, "webhook-worker": nil}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Answers["primary_project"].Choice != "payments-api" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestClientClassifiesBillingError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"valid credit card required"}}`))
	}))
	defer server.Close()
	client := Client{Provider: "jev-vercel", APIKey: "test-key", Endpoint: server.URL}
	_, err := client.Evaluate(context.Background(), "state", map[string]Question{"ok": {Type: "noul", Instructions: "ok?"}})
	providerErr, ok := err.(*ProviderError)
	if !ok || providerErr.Kind != ErrorBilling {
		t.Fatalf("expected billing error, got %T: %v", err, err)
	}
}
