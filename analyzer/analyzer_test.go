package analyzer

import "testing"

type fixtureAnalyzer struct{}

func (fixtureAnalyzer) Name() string             { return "fixture" }
func (fixtureAnalyzer) Matches(path string) bool { return path == "fixture.demo" }
func (fixtureAnalyzer) Analyze(string, []byte) Result {
	return Result{Language: "Fixture", Identifiers: []string{"fixture"}}
}

func TestRegistryDispatch(t *testing.T) {
	registry := NewRegistry(fixtureAnalyzer{})
	result := registry.Analyze("fixture.demo", nil)
	if result.Language != "Fixture" || len(result.Identifiers) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if APIVersion != 1 {
		t.Fatalf("unexpected API version: %d", APIVersion)
	}
}
