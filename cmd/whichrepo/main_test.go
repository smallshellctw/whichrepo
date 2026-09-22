package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultSettingsUsesLayeredConfigAndEnvironment(t *testing.T) {
	root := t.TempDir()
	shared := `version: 1
routing:
  top_k: 4
  provider: local
  model: shared-model
  provider_timeout: 15s
dashboard:
  listen: 127.0.0.1:9000
`
	local := `routing:
  top_k: 6
`
	if err := os.WriteFile(filepath.Join(root, ".whichrepo.yaml"), []byte(shared), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".whichrepo.local.yaml"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WHICHREPO_WORKSPACE", "")
	t.Setenv("WHICHREPO_CONFIG", "")
	t.Setenv("WHICHREPO_DB", "")
	t.Setenv("WHICHREPO_PROVIDER", "")
	t.Setenv("WHICHREPO_MODEL", "environment-model")
	settings, err := defaultSettings(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if settings.topK != 6 || settings.model != "environment-model" || settings.providerTimeout != 15*time.Second || settings.dashboardListen != "127.0.0.1:9000" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestCommandFlagsUseTargetWorkspaceConfiguration(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WHICHREPO_WORKSPACE", "")
	t.Setenv("WHICHREPO_CONFIG", "")
	configText := `version: 1
routing:
  top_k: 3
dashboard:
  listen: 127.0.0.1:8787
`
	if err := os.WriteFile(filepath.Join(root, ".whichrepo.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"--workspace", root, "--top-k", "5", "route a task"}
	flags, settings, err := commandFlags("route", args)
	if err != nil {
		t.Fatal(err)
	}
	flags.IntVar(&settings.topK, "top-k", settings.topK, "")
	if err := parseSettingsFlags(flags, args, settings); err != nil {
		t.Fatal(err)
	}
	if settings.topK != 5 || settings.workspace != root {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestFlagValueSupportsBothForms(t *testing.T) {
	if value := flagValue([]string{"--workspace=/tmp/one"}, "workspace", ""); value != "/tmp/one" {
		t.Fatalf("equals form = %q", value)
	}
	if value := flagValue([]string{"--workspace", "/tmp/two"}, "workspace", ""); value != "/tmp/two" {
		t.Fatalf("separate form = %q", value)
	}
}
