package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWorkspaceAppliesDefaultsAndLocalOverride(t *testing.T) {
	root := t.TempDir()
	shared := `version: 1
workspace:
  name: commerce
routing:
  top_k: 5
projects:
  payments:
    path: services/payments
`
	local := `routing:
  provider: jev-vercel
dashboard:
  open: true
`
	if err := os.WriteFile(filepath.Join(root, ".whichrepo.yaml"), []byte(shared), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".whichrepo.local.yaml"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadWorkspace(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routing.TopK != 5 || cfg.Routing.Provider != "jev-vercel" || !cfg.Dashboard.Open {
		t.Fatalf("unexpected layered config: %+v", cfg)
	}
	if cfg.Index.MaxFilesPerProject != 800 || cfg.Routing.ProviderTimeout != "20s" {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
	if _, ok := cfg.Projects["payments"]; !ok {
		t.Fatalf("project override lost: %+v", cfg.Projects)
	}
}

func TestConfigurationSchemaIsValidJSON(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "configs", "whichrepo.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["title"] == "" {
		t.Fatal("schema title is missing")
	}
}

func TestRenderIncludesEveryConfigSectionAndSchema(t *testing.T) {
	data, err := Render("commerce")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, expected := range []string{SchemaURL, "workspace:", "index:", "routing:", "dashboard:", "privacy:", "projects:"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("rendered config missing %q:\n%s", expected, text)
		}
	}
}

func TestLoadRejectsUnsafeOrInvalidValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".whichrepo.yaml")
	data := []byte("version: 1\nrouting:\n  top_k: 99\ndashboard:\n  listen: 0.0.0.0:8787\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid configuration to fail")
	}
}
