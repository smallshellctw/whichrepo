package analyzer

import (
	"slices"
	"testing"
)

func TestManifestAnalyzers(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		source     string
		language   string
		identifier string
		dependency string
	}{
		{"node", "package.json", `{"name":"checkout-web","dependencies":{"payment-sdk":"workspace:*"}}`, "JavaScript", "checkout-web", "payment-sdk"},
		{"python", "pyproject.toml", "[project]\nname = \"fraud-model\"\ndependencies = [\"httpx>=0.27\"]\n", "Python", "fraud-model", "httpx"},
		{"rust", "Cargo.toml", "[package]\nname = \"event-router\"\n[dependencies]\nserde = \"1\"\n", "Rust", "event-router", "serde"},
		{"maven", "pom.xml", `<project><groupId>dev.whichrepo</groupId><artifactId>ledger-api</artifactId><dependencies><dependency><groupId>dev.shared</groupId><artifactId>contracts</artifactId></dependency></dependencies></project>`, "JVM", "ledger-api", "contracts"},
		{"dotnet", "Billing.csproj", `<Project><PropertyGroup><AssemblyName>Billing.Api</AssemblyName></PropertyGroup><ItemGroup><ProjectReference Include="..\Shared\Shared.csproj" /></ItemGroup></Project>`, ".NET", "Billing.Api", "Shared"},
		{"php", "composer.json", `{"name":"acme/catalog","require":{"acme/contracts":"^1"}}`, "PHP", "acme/catalog", "acme/contracts"},
		{"ruby", "billing.gemspec", "spec.name = \"billing\"\nspec.add_dependency \"faraday\"", "Ruby", "billing", "faraday"},
	}
	registry := DefaultRegistry()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := registry.Analyze(test.path, []byte(test.source))
			if result.Language != test.language {
				t.Fatalf("language = %q, want %q", result.Language, test.language)
			}
			if !slices.Contains(result.Identifiers, test.identifier) {
				t.Fatalf("identifiers = %v, want %q", result.Identifiers, test.identifier)
			}
			if !slices.Contains(result.Dependencies, test.dependency) {
				t.Fatalf("dependencies = %v, want %q", result.Dependencies, test.dependency)
			}
		})
	}
}

func TestTreeSitterAndFallbackImports(t *testing.T) {
	tests := []struct {
		path       string
		source     string
		language   string
		dependency string
	}{
		{"handler.go", "package handler\nimport \"example.com/shared/contracts\"\nfunc Handle() {}", "Go", "example.com/shared/contracts"},
		{"risk.py", "from shared_contracts import Risk\ndef score():\n    return Risk()\n", "Python", "shared_contracts"},
		{"checkout.ts", "import { retry } from '@workspace/retry-policy'\nexport function checkout() { retry() }", "TypeScript", "@workspace/retry-policy"},
		{"worker.rs", "use shared_events::WebhookEvent;\nfn consume() {}", "Rust", "shared_events"},
		{"Billing.cs", "using Company.Shared.Contracts;\nclass Billing {}", ".NET", "Company.Shared.Contracts"},
		{"catalog.php", "<?php\nuse Acme\\Contracts\\Product;\n", "PHP", `Acme\Contracts\Product`},
		{"job.rb", "require 'shared_jobs'\nclass Job; end", "Ruby", "shared_jobs"},
	}
	registry := DefaultRegistry()
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			result := registry.Analyze(test.path, []byte(test.source))
			if result.Language != test.language {
				t.Fatalf("language = %q, want %q", result.Language, test.language)
			}
			if !slices.Contains(result.Dependencies, test.dependency) {
				t.Fatalf("dependencies = %v, want %q", result.Dependencies, test.dependency)
			}
		})
	}
}

func TestManifestRecognition(t *testing.T) {
	for _, path := range []string{"go.mod", "go.work", "package.json", "pnpm-workspace.yaml", "pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle.kts", "settings.gradle", "service.csproj", "workspace.sln", "composer.json", "Gemfile", "tool.gemspec"} {
		if !IsManifest(path) {
			t.Errorf("expected %s to be a manifest", path)
		}
	}
}
