// Package analyzer provides language-neutral project and source analysis.
// The scanner depends on this package instead of hard-coding one language's
// build system, which keeps mixed-language workspaces first-class.
package analyzer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	csharpgrammar "github.com/odvcencio/gotreesitter/grammars/c_sharp"
	gogrammar "github.com/odvcencio/gotreesitter/grammars/go"
	javagrammar "github.com/odvcencio/gotreesitter/grammars/java"
	javascriptgrammar "github.com/odvcencio/gotreesitter/grammars/javascript"
	phpgrammar "github.com/odvcencio/gotreesitter/grammars/php"
	pythongrammar "github.com/odvcencio/gotreesitter/grammars/python"
	rubygrammar "github.com/odvcencio/gotreesitter/grammars/ruby"
	rustgrammar "github.com/odvcencio/gotreesitter/grammars/rust"
	tsxgrammar "github.com/odvcencio/gotreesitter/grammars/tsx"
	typescriptgrammar "github.com/odvcencio/gotreesitter/grammars/typescript"
	sdk "github.com/smallshellctw/whichrepo/analyzer"
)

type Result = sdk.Result
type LanguageAnalyzer = sdk.LanguageAnalyzer
type Registry = sdk.Registry

func DefaultRegistry() Registry {
	return sdk.NewRegistry(normalizingAnalyzer{manifestAnalyzer{}}, normalizingAnalyzer{syntaxAnalyzer{}}, fallbackAnalyzer{})
}

type normalizingAnalyzer struct{ LanguageAnalyzer }

func (a normalizingAnalyzer) Analyze(path string, source []byte) Result {
	return normalize(a.LanguageAnalyzer.Analyze(path, source))
}

type fallbackAnalyzer struct{}

func (fallbackAnalyzer) Name() string        { return "fallback" }
func (fallbackAnalyzer) Matches(string) bool { return true }
func (fallbackAnalyzer) Analyze(path string, _ []byte) Result {
	return Result{Language: LanguageForPath(path)}
}

func ManifestNames() []string {
	return []string{
		"go.mod", "go.work", "package.json", "pnpm-workspace.yaml",
		"pyproject.toml", "requirements.txt", "cargo.toml", "pom.xml",
		"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "composer.json", "gemfile",
		"*.gemspec", "*.csproj", "*.fsproj", "*.vbproj", "*.sln",
	}
}

func IsManifest(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	for _, pattern := range ManifestNames() {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
	}
	return false
}

func LanguageForPath(path string) string {
	name := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case name == "go.mod" || name == "go.work" || ext == ".go":
		return "Go"
	case name == "package.json" || name == "pnpm-workspace.yaml" || ext == ".js" || ext == ".jsx" || ext == ".mjs" || ext == ".cjs":
		return "JavaScript"
	case ext == ".ts" || ext == ".tsx" || ext == ".mts" || ext == ".cts":
		return "TypeScript"
	case name == "pyproject.toml" || name == "requirements.txt" || ext == ".py":
		return "Python"
	case name == "pom.xml" || strings.HasPrefix(name, "build.gradle") || strings.HasPrefix(name, "settings.gradle") || ext == ".java" || ext == ".kt":
		return "JVM"
	case name == "cargo.toml" || ext == ".rs":
		return "Rust"
	case ext == ".cs" || ext == ".fs" || ext == ".fsx" || ext == ".vb" || ext == ".csproj" || ext == ".fsproj" || ext == ".vbproj" || ext == ".sln":
		return ".NET"
	case name == "composer.json" || ext == ".php":
		return "PHP"
	case name == "gemfile" || ext == ".gemspec" || ext == ".rb":
		return "Ruby"
	case ext == ".vue":
		return "Vue"
	case ext == ".proto":
		return "Protocol Buffers"
	case ext == ".sql":
		return "SQL"
	case ext == ".swift":
		return "Swift"
	default:
		return ""
	}
}

type manifestAnalyzer struct{}

func (manifestAnalyzer) Name() string             { return "manifest" }
func (manifestAnalyzer) Matches(path string) bool { return IsManifest(path) }
func (manifestAnalyzer) Analyze(path string, source []byte) Result {
	name := strings.ToLower(filepath.Base(path))
	result := Result{Language: LanguageForPath(path)}
	switch {
	case name == "go.mod":
		parseGoMod(source, &result)
	case name == "package.json":
		parsePackageJSON(source, &result)
	case name == "pyproject.toml":
		parsePyProject(source, &result)
	case name == "requirements.txt":
		parseRequirements(source, &result)
	case name == "cargo.toml":
		parseCargo(source, &result)
	case name == "pom.xml":
		parseMaven(source, &result)
	case strings.HasPrefix(name, "build.gradle") || strings.HasPrefix(name, "settings.gradle"):
		parseGradle(source, &result)
	case name == "composer.json":
		parseComposer(source, &result)
	case name == "gemfile" || strings.HasSuffix(name, ".gemspec"):
		parseRubyManifest(source, &result)
	case strings.HasSuffix(name, ".csproj") || strings.HasSuffix(name, ".fsproj") || strings.HasSuffix(name, ".vbproj") || strings.HasSuffix(name, ".sln"):
		parseDotNet(source, &result)
	}
	for _, id := range result.Identifiers {
		result.Evidence = append(result.Evidence, fmt.Sprintf("PACKAGE %s", id))
	}
	return result
}

func parseGoMod(source []byte, result *Result) {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	inRequire := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			result.Identifiers = append(result.Identifiers, strings.TrimSpace(strings.TrimPrefix(line, "module ")))
		}
		if line == "require (" {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		}
		if inRequire || strings.Contains(line, " ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && (strings.Contains(fields[0], ".") || strings.Contains(fields[0], "/")) {
				result.Dependencies = append(result.Dependencies, fields[0])
			}
		}
	}
}

func parsePackageJSON(source []byte, result *Result) {
	var pkg struct {
		Name                 string            `json:"name"`
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if json.Unmarshal(source, &pkg) != nil {
		return
	}
	result.Identifiers = append(result.Identifiers, pkg.Name)
	for _, values := range []map[string]string{pkg.Dependencies, pkg.DevDependencies, pkg.PeerDependencies, pkg.OptionalDependencies} {
		for dependency := range values {
			result.Dependencies = append(result.Dependencies, dependency)
		}
	}
}

var (
	tomlNameRE       = regexp.MustCompile(`(?m)^\s*name\s*=\s*["']([^"']+)["']`)
	tomlDependencyRE = regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_.-]+)\s*(?:=|@)\s*`)
	pythonImportRE   = regexp.MustCompile(`(?m)^\s*(?:from|import)\s+([A-Za-z0-9_.]+)`)
	jsImportRE       = regexp.MustCompile(`(?m)(?:from\s+|require\s*\(\s*)["']([^"']+)["']`)
	rustUseRE        = regexp.MustCompile(`(?m)^\s*(?:use|extern\s+crate)\s+([A-Za-z0-9_]+)`)
	phpUseRE         = regexp.MustCompile(`(?m)^\s*use\s+([A-Za-z0-9_\\]+)`)
	rubyRequireRE    = regexp.MustCompile(`(?m)^\s*require\s+["']([^"']+)["']`)
	dotnetUsingRE    = regexp.MustCompile(`(?m)^\s*using\s+([A-Za-z0-9_.]+)`)
)

func parsePyProject(source []byte, result *Result) {
	if match := tomlNameRE.FindSubmatch(source); len(match) > 1 {
		result.Identifiers = append(result.Identifiers, string(match[1]))
	}
	inDependencyArray := false
	inDependencyTable := false
	scanner := bufio.NewScanner(bytes.NewReader(source))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[tool.poetry.dependencies]") || strings.HasPrefix(line, "[project.optional-dependencies]") {
			inDependencyTable = true
			inDependencyArray = false
			continue
		}
		if strings.HasPrefix(line, "[") {
			inDependencyTable = false
		}
		if strings.HasPrefix(line, "dependencies") && strings.Contains(line, "[") {
			inDependencyArray = true
		}
		if inDependencyArray {
			for _, match := range regexp.MustCompile(`["']([A-Za-z0-9_.-]+)(?:[<>=!~ ].*)?["']`).FindAllStringSubmatch(line, -1) {
				result.Dependencies = append(result.Dependencies, match[1])
			}
			if strings.Contains(line, "]") {
				inDependencyArray = false
			}
		}
		if inDependencyTable {
			if match := tomlDependencyRE.FindStringSubmatch(line); len(match) > 1 && match[1] != "python" {
				result.Dependencies = append(result.Dependencies, match[1])
			}
		}
	}
}

func parseRequirements(source []byte, result *Result) {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		name := regexp.MustCompile(`[<>=!~\[; ]`).Split(line, 2)[0]
		result.Dependencies = append(result.Dependencies, name)
	}
}

func parseCargo(source []byte, result *Result) {
	if match := tomlNameRE.FindSubmatch(source); len(match) > 1 {
		result.Identifiers = append(result.Identifiers, string(match[1]))
	}
	inDependencies := false
	scanner := bufio.NewScanner(bytes.NewReader(source))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[dependencies") || strings.HasPrefix(line, "[dev-dependencies") || strings.HasPrefix(line, "[build-dependencies") {
			inDependencies = true
			continue
		}
		if inDependencies && strings.HasPrefix(line, "[") {
			inDependencies = false
		}
		if inDependencies {
			if match := tomlDependencyRE.FindStringSubmatch(line); len(match) > 1 {
				result.Dependencies = append(result.Dependencies, match[1])
			}
		}
	}
}

func parseMaven(source []byte, result *Result) {
	var project struct {
		ArtifactID   string   `xml:"artifactId"`
		GroupID      string   `xml:"groupId"`
		Modules      []string `xml:"modules>module"`
		Dependencies []struct {
			ArtifactID string `xml:"artifactId"`
			GroupID    string `xml:"groupId"`
		} `xml:"dependencies>dependency"`
	}
	if xml.Unmarshal(source, &project) != nil {
		return
	}
	result.Identifiers = append(result.Identifiers, project.ArtifactID, joinNonEmpty(project.GroupID, project.ArtifactID))
	result.Dependencies = append(result.Dependencies, project.Modules...)
	for _, dependency := range project.Dependencies {
		result.Dependencies = append(result.Dependencies, dependency.ArtifactID, joinNonEmpty(dependency.GroupID, dependency.ArtifactID))
	}
}

func parseGradle(source []byte, result *Result) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:implementation|api|compileOnly|runtimeOnly|testImplementation)\s*\(?["']([^"']+)["']`),
		regexp.MustCompile(`project\s*\(\s*["']:?([^"']+)["']\s*\)`),
	}
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllSubmatch(source, -1) {
			value := string(match[1])
			parts := strings.Split(value, ":")
			result.Dependencies = append(result.Dependencies, value, parts[len(parts)-1])
		}
	}
}

func parseComposer(source []byte, result *Result) {
	var pkg struct {
		Name       string            `json:"name"`
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}
	if json.Unmarshal(source, &pkg) != nil {
		return
	}
	result.Identifiers = append(result.Identifiers, pkg.Name)
	for _, deps := range []map[string]string{pkg.Require, pkg.RequireDev} {
		for dependency := range deps {
			result.Dependencies = append(result.Dependencies, dependency)
		}
	}
}

func parseRubyManifest(source []byte, result *Result) {
	for _, match := range regexp.MustCompile(`(?m)^\s*(?:gem|\w+\.add_(?:runtime_)?dependency)\s+["']([^"']+)["']`).FindAllSubmatch(source, -1) {
		result.Dependencies = append(result.Dependencies, string(match[1]))
	}
	if match := regexp.MustCompile(`(?m)\.name\s*=\s*["']([^"']+)["']`).FindSubmatch(source); len(match) > 1 {
		result.Identifiers = append(result.Identifiers, string(match[1]))
	}
}

func parseDotNet(source []byte, result *Result) {
	for _, match := range regexp.MustCompile(`(?i)<(?:AssemblyName|RootNamespace)>([^<]+)</`).FindAllSubmatch(source, -1) {
		result.Identifiers = append(result.Identifiers, string(match[1]))
	}
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)<ProjectReference\s+Include=["']([^"']+)["']`),
		regexp.MustCompile(`(?i)<PackageReference\s+Include=["']([^"']+)["']`),
		regexp.MustCompile(`(?m)Project\([^)]*\)\s*=\s*["'][^"']+["'],\s*["']([^"']+\.(?:csproj|fsproj|vbproj))["']`),
	} {
		for _, match := range pattern.FindAllSubmatch(source, -1) {
			value := string(match[1])
			if strings.Contains(value, "/") || strings.Contains(value, "\\") {
				value = strings.TrimSuffix(filepath.Base(filepath.ToSlash(strings.ReplaceAll(value, "\\", "/"))), filepath.Ext(value))
			}
			result.Dependencies = append(result.Dependencies, value)
		}
	}
}

type syntaxAnalyzer struct{}

func (syntaxAnalyzer) Name() string             { return "tree-sitter" }
func (syntaxAnalyzer) Matches(path string) bool { return syntaxLanguage(path) != nil }
func (syntaxAnalyzer) Analyze(path string, source []byte) Result {
	language := syntaxLanguage(path)
	result := Result{Language: LanguageForPath(path)}
	if language == nil || len(source) == 0 {
		return result
	}
	parser := gotreesitter.NewParser(language)
	parser.SetTimeoutMicros(75_000)
	tree, err := parser.ParseStrict(source)
	if err == nil && tree != nil {
		program, programErr := gotreesitter.NewFactProgram(language, gotreesitter.FactDefinitions|gotreesitter.FactImports)
		if programErr == nil {
			facts := program.Extract(tree)
			for _, item := range facts.Definitions {
				if item.Name != "" {
					result.Symbols = append(result.Symbols, item.Name)
				}
			}
			for _, item := range facts.Imports {
				if item.Path != "" {
					result.Dependencies = append(result.Dependencies, item.Path)
				}
				if item.From != "" {
					result.Dependencies = append(result.Dependencies, item.From)
				}
			}
		}
		result.Evidence = append(result.Evidence, fmt.Sprintf("SYNTAX %s parsed=%s", filepath.ToSlash(path), strings.ToLower(result.Language)))
	}
	result.Dependencies = append(result.Dependencies, fallbackImports(path, source)...)
	if len(result.Symbols) > 0 {
		limit := len(result.Symbols)
		if limit > 12 {
			limit = 12
		}
		result.Evidence = append(result.Evidence, "SYMBOLS "+strings.Join(result.Symbols[:limit], " "))
	}
	return result
}

func syntaxLanguage(path string) *gotreesitter.Language {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return gogrammar.Language()
	case ".js", ".jsx", ".mjs", ".cjs":
		return javascriptgrammar.Language()
	case ".ts", ".mts", ".cts":
		return typescriptgrammar.Language()
	case ".tsx":
		return tsxgrammar.Language()
	case ".py":
		return pythongrammar.Language()
	case ".java":
		return javagrammar.Language()
	case ".rs":
		return rustgrammar.Language()
	case ".cs":
		return csharpgrammar.Language()
	case ".php":
		return phpgrammar.Language()
	case ".rb":
		return rubygrammar.Language()
	default:
		return nil
	}
}

func fallbackImports(path string, source []byte) []string {
	ext := strings.ToLower(filepath.Ext(path))
	var pattern *regexp.Regexp
	switch ext {
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts":
		pattern = jsImportRE
	case ".py":
		pattern = pythonImportRE
	case ".rs":
		pattern = rustUseRE
	case ".cs":
		pattern = dotnetUsingRE
	case ".php":
		pattern = phpUseRE
	case ".rb":
		pattern = rubyRequireRE
	default:
		return nil
	}
	result := []string{}
	for _, match := range pattern.FindAllSubmatch(source, -1) {
		if len(match) > 1 {
			result = append(result, string(match[1]))
		}
	}
	return result
}

func normalize(result Result) Result {
	result.Identifiers = unique(result.Identifiers)
	result.Dependencies = unique(result.Dependencies)
	result.Symbols = unique(result.Symbols)
	result.Evidence = unique(result.Evidence)
	return result
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func joinNonEmpty(parts ...string) string {
	filtered := []string{}
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			filtered = append(filtered, strings.TrimSpace(part))
		}
	}
	return strings.Join(filtered, ":")
}
