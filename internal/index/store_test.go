package index

import (
	"context"
	"database/sql"
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

func TestOpenMigratesPreIdentifierIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE projects_v2 (
name TEXT PRIMARY KEY, path TEXT NOT NULL, description TEXT NOT NULL,
aliases_json TEXT NOT NULL, languages_json TEXT NOT NULL,
dependencies_json TEXT NOT NULL, manifests_json TEXT NOT NULL,
file_count INTEGER NOT NULL, content TEXT NOT NULL, indexed_at TEXT NOT NULL
)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project := model.Project{Name: "billing-api", Path: "/billing", Identifiers: []string{"Billing.Api"}, IndexedAt: time.Now()}
	if err := store.ReplaceProjects(context.Background(), []model.Project{project}); err != nil {
		t.Fatal(err)
	}
	projects, err := store.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || len(projects[0].Identifiers) != 1 || projects[0].Identifiers[0] != "Billing.Api" {
		t.Fatalf("unexpected migrated projects: %+v", projects)
	}
}
