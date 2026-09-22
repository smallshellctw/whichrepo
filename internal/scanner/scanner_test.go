package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/smallshellctw/whichrepo/internal/config"
)

func TestScanUsesOverridesAndSourceContent(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "payments-api")
	if err := os.MkdirAll(filepath.Join(projectPath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "go.mod"), []byte("module example/payments-api\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "risk.go"), []byte("package risk\n// clickhouse timeout\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	projects, err := (Scanner{
		Root: root,
		Config: config.Config{Version: 1, Projects: map[string]config.ProjectOverride{
			"payments-api": {Description: "Payment and checkout service", Aliases: []string{"checkout"}},
		}},
	}).Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects = %d", len(projects))
	}
	if projects[0].Description != "Payment and checkout service" || len(projects[0].Aliases) != 1 {
		t.Fatalf("unexpected project: %+v", projects[0])
	}
	if projects[0].Content == "" {
		t.Fatal("expected indexed source content")
	}
}
