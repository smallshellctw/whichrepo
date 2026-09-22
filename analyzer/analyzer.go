// Package analyzer defines WhichRepo's language-neutral analyzer SDK.
//
// The official binary registers the built-in analyzers. Integrators that
// produce a custom WhichRepo build can add analyzers without depending on the
// scanner or route result implementation.
package analyzer

// APIVersion changes only when the analyzer contract is incompatible.
const APIVersion = 1

// Result is the common output shared by manifest and syntax analyzers.
type Result struct {
	Language     string
	Identifiers  []string
	Dependencies []string
	Symbols      []string
	Evidence     []string
}

// LanguageAnalyzer analyzes one bounded local source or manifest file.
// Implementations should be deterministic, avoid network access, and return a
// partial Result instead of failing an entire workspace index.
type LanguageAnalyzer interface {
	Name() string
	Matches(path string) bool
	Analyze(path string, source []byte) Result
}

// Registry dispatches a file to the first matching analyzer.
type Registry struct {
	analyzers []LanguageAnalyzer
}

func NewRegistry(analyzers ...LanguageAnalyzer) Registry {
	return Registry{analyzers: append([]LanguageAnalyzer(nil), analyzers...)}
}

// Analyze dispatches path to the first registered matching analyzer.
func (r Registry) Analyze(path string, source []byte) Result {
	for _, item := range r.analyzers {
		if item.Matches(path) {
			return item.Analyze(path, source)
		}
	}
	return Result{}
}

// Analyzers returns a copy of the registered analyzers for diagnostics.
func (r Registry) Analyzers() []LanguageAnalyzer {
	return append([]LanguageAnalyzer(nil), r.analyzers...)
}
