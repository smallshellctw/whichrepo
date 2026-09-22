package scanner

import (
	"os"
	"path/filepath"
	"slices"
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

func TestScanResolvesCrossLanguagePackageIdentifiers(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "shared-contracts")
	fraud := filepath.Join(root, "fraud-model")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fraud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shared, "package.json"), []byte(`{"name":"shared-contracts","version":"0.1.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shared, "index.ts"), []byte("export interface PaymentAttempt { countryMismatch: boolean }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fraud, "pyproject.toml"), []byte("[project]\nname = \"fraud-model\"\ndependencies = [\"shared-contracts>=1.0\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fraud, "score.py"), []byte("from shared_contracts import PaymentAttempt\ndef score(attempt): return 0.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	projects, err := (Scanner{Root: root, Config: config.Config{Version: 1, Projects: map[string]config.ProjectOverride{}}}).Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("projects = %d, want 2", len(projects))
	}
	for _, project := range projects {
		if project.Name == "fraud-model" {
			if !slices.Contains(project.Dependencies, "shared-contracts") {
				t.Fatalf("fraud dependencies = %v", project.Dependencies)
			}
			if !slices.Contains(project.Languages, "Python") {
				t.Fatalf("fraud languages = %v", project.Languages)
			}
			return
		}
	}
	t.Fatal("fraud-model was not discovered")
}

func TestScanHonorsNestedIncludeAndExcludeGlobs(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"services/payment-api", "services/archive-old", "tools/dev-cli"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path, "package.json"), []byte(`{"name":"`+filepath.Base(path)+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	projects, err := (Scanner{Root: root, Config: config.Config{
		Version: 1,
		Workspace: config.Workspace{
			Include: []string{"services/*"},
			Exclude: []string{"archive-*"},
		},
		Projects: map[string]config.ProjectOverride{},
	}}).Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "payment-api" {
		t.Fatalf("projects = %+v", projects)
	}
}
