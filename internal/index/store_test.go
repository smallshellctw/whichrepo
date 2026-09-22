package index

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/smallshellctw/whichrepo/internal/model"
)

func TestSearchRanksAliasAndEvidence(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	projects := []model.Project{
		{Name: "payments-api", Path: "/payments", Description: "Checkout and webhook API", Aliases: []string{"checkout"}, Content: "FILE webhook.go\nwebhook retry timeout", IndexedAt: time.Now()},
		{Name: "webhook-worker", Path: "/worker", Description: "Webhook worker", Content: "FILE worker.go\njob processing", IndexedAt: time.Now()},
	}
	if err := store.ReplaceProjects(context.Background(), projects); err != nil {
		t.Fatal(err)
	}
	candidates, err := store.Search(context.Background(), "Add retry limits to checkout webhooks", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 || candidates[0].Project != "payments-api" {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	if len(candidates[0].Evidence) == 0 {
		t.Fatalf("expected evidence: %+v", candidates[0])
	}
}
