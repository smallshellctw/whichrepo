package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStripJSONCommentsPreservesURLs(t *testing.T) {
	input := []byte("{\n// comment\n\"url\": \"https://example.com/a//b\", /* block */ \"ok\": true\n}")
	cleaned, err := stripJSONComments(input)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(cleaned, &value); err != nil {
		t.Fatal(err)
	}
	if value["url"] != "https://example.com/a//b" {
		t.Fatalf("URL changed: %#v", value["url"])
	}
}

func TestSetupCursorMergesAndBacksUp(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte("{\n  // existing server\n  \"mcpServers\": {\"other\": {\"command\": \"other\"}}\n}\n")
	path := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatal(err)
	}
	results, err := Apply(t.Context(), Options{Client: "cursor", Scope: "user", Workspace: workspace, Binary: "/tmp/whichrepo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].BackupPath == "" {
		t.Fatalf("unexpected results: %+v", results)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if len(root.MCPServers) != 2 || root.MCPServers["whichrepo"] == nil || root.MCPServers["other"] == nil {
		t.Fatalf("servers were not merged: %s", data)
	}
}

func TestDryRunDoesNotWriteCursorConfig(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	results, err := Apply(t.Context(), Options{Client: "cursor", Scope: "project", Workspace: workspace, Binary: "/tmp/whichrepo", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "dry_run" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".cursor", "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote config: %v", err)
	}
}

func TestArgumentValue(t *testing.T) {
	if got := argumentValue([]string{"mcp", "add", "whichrepo", "--scope", "project"}, "--scope", "user"); got != "project" {
		t.Fatalf("scope = %q", got)
	}
	if got := argumentValue([]string{"--scope=local"}, "--scope", "user"); got != "local" {
		t.Fatalf("equals scope = %q", got)
	}
}
