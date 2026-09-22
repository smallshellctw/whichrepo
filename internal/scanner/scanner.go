package scanner

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/smallshellctw/whichrepo/internal/analyzer"
	"github.com/smallshellctw/whichrepo/internal/config"
	"github.com/smallshellctw/whichrepo/internal/model"
)

const (
	maxFilesPerProject = 800
	maxBytesPerFile    = 12 * 1024
	maxBytesPerProject = 4 * 1024 * 1024
)

var ignoredDirs = map[string]struct{}{
	".git": {}, ".idea": {}, ".vscode": {}, "vendor": {}, "node_modules": {},
	"dist": {}, "build": {}, "coverage": {}, ".next": {}, "tmp": {}, "logs": {},
	"mocks": {}, "mock": {}, "third_party": {}, "target": {}, ".venv": {}, "venv": {},
}

var sensitiveNames = map[string]struct{}{
	".env": {}, ".env.local": {}, ".npmrc": {}, ".pypirc": {}, "credentials": {},
	"credentials.json": {}, "id_rsa": {}, "id_ed25519": {}, "secrets.yaml": {}, "secrets.yml": {},
}

var indexedExtensions = map[string]struct{}{
	".go": {}, ".proto": {}, ".yaml": {}, ".yml": {}, ".json": {}, ".md": {},
	".php": {}, ".js": {}, ".jsx": {}, ".mjs": {}, ".cjs": {}, ".ts": {}, ".tsx": {}, ".mts": {}, ".cts": {}, ".vue": {},
	".sql": {}, ".toml": {}, ".mod": {}, ".gradle": {}, ".xml": {}, ".py": {},
	".rs": {}, ".java": {}, ".kt": {}, ".swift": {}, ".rb": {}, ".cs": {}, ".fs": {}, ".fsx": {}, ".vb": {},
}

type Scanner struct {
	Root   string
	Config config.Config
}

func (s Scanner) Scan() ([]model.Project, error) {
	paths, err := s.discoverProjects()
	if err != nil {
		return nil, err
	}
	projects := make([]model.Project, 0, len(paths))
	registry := analyzer.DefaultRegistry()
	for _, path := range paths {
		name := filepath.Base(path)
		override := findOverride(s.Config.Projects, name, path, s.Root)
		project, err := s.scanProject(name, path, override, registry)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	resolveDependencies(projects)
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}

func (s Scanner) discoverProjects() ([]string, error) {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	add := func(path string) {
		absolute, err := filepath.Abs(path)
		if err == nil {
			seen[absolute] = struct{}{}
		}
	}
	for _, override := range s.Config.Projects {
		if override.Path != "" {
			path := override.Path
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			if looksLikeProject(path) {
				add(path)
			}
		}
	}

	rootIsProject := looksLikeProject(root)
	if rootIsProject {
		add(root)
		if !hasWorkspaceMarker(root) {
			return sortedKeys(seen), nil
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read workspace root: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || excluded(entry.Name(), s.Config.Workspace.Exclude) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if included(entry.Name(), s.Config.Workspace.Include) && looksLikeProject(path) {
			add(path)
		}
	}
	if len(s.Config.Workspace.Include) > 0 {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || path == root {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") || isIgnoredDir(entry.Name()) || excluded(rel, s.Config.Workspace.Exclude) {
					return filepath.SkipDir
				}
				if strings.Count(rel, "/") > 4 {
					return filepath.SkipDir
				}
				if included(rel, s.Config.Workspace.Include) && looksLikeProject(path) {
					add(path)
				}
			}
			return nil
		})
	}

	if rootIsProject && hasWorkspaceMarker(root) {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if !entry.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			depth := strings.Count(filepath.ToSlash(rel), "/")
			if depth > 2 {
				return filepath.SkipDir
			}
			if path != root {
				if isIgnoredDir(entry.Name()) || excluded(filepath.ToSlash(rel), s.Config.Workspace.Exclude) {
					return filepath.SkipDir
				}
				if included(filepath.ToSlash(rel), s.Config.Workspace.Include) && hasManifest(path) {
					add(path)
				}
			}
			return nil
		})
	}
	return sortedKeys(seen), nil
}

func (s Scanner) scanProject(name, path string, override config.ProjectOverride, registry analyzer.Registry) (model.Project, error) {
	files := projectFiles(path)
	if len(files) > maxFilesPerProject {
		files = files[:maxFilesPerProject]
	}
	var builder strings.Builder
	var totalBytes int
	languages := map[string]struct{}{}
	manifests := []string{}
	identifiers := []string{}
	rawDependencies := []string{}
	for _, relativePath := range files {
		if totalBytes >= maxBytesPerProject || !shouldIndex(relativePath) {
			continue
		}
		filePath := filepath.Join(path, relativePath)
		data, err := readPrefix(filePath, maxBytesPerFile)
		if err != nil || !utf8.Valid(data) {
			continue
		}
		builder.WriteString("\nFILE ")
		builder.WriteString(filepath.ToSlash(relativePath))
		builder.WriteByte('\n')
		builder.Write(data)
		totalBytes += len(data)
		analysis := registry.Analyze(relativePath, data)
		if analysis.Language != "" {
			languages[analysis.Language] = struct{}{}
		}
		identifiers = append(identifiers, analysis.Identifiers...)
		rawDependencies = append(rawDependencies, analysis.Dependencies...)
		for _, evidence := range analysis.Evidence {
			builder.WriteByte('\n')
			builder.WriteString(evidence)
		}
		if analyzer.IsManifest(relativePath) {
			manifests = append(manifests, filepath.ToSlash(relativePath))
		}
	}

	description := strings.TrimSpace(override.Description)
	if description == "" {
		description = inferDescription(path, builder.String())
	}
	languageList := sortedSet(languages)
	return model.Project{
		Name:         name,
		Path:         path,
		Description:  description,
		Aliases:      append([]string(nil), override.Aliases...),
		Identifiers:  uniqueStrings(identifiers),
		Languages:    languageList,
		Dependencies: uniqueStrings(rawDependencies),
		Manifests:    uniqueStrings(manifests),
		FileCount:    len(files),
		Content:      builder.String(),
		IndexedAt:    time.Now().UTC(),
	}, nil
}

func projectFiles(path string) []string {
	if git, err := exec.LookPath("git"); err == nil {
		command := exec.Command(git, "-C", path, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
		if output, err := command.Output(); err == nil {
			parts := bytes.Split(output, []byte{0})
			files := make([]string, 0, len(parts))
			for _, part := range parts {
				if len(part) > 0 {
					files = append(files, string(part))
				}
			}
			sort.Strings(files)
			return files
		}
	}
	var files []string
	_ = filepath.WalkDir(path, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if filePath != path {
				if _, ignored := ignoredDirs[entry.Name()]; ignored {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, _ := filepath.Rel(path, filePath)
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	return files
}

func looksLikeProject(path string) bool {
	if info, err := os.Stat(filepath.Join(path, ".git")); err == nil && info.IsDir() {
		return true
	}
	return hasManifest(path) || fileExists(filepath.Join(path, "README.md")) || fileExists(filepath.Join(path, "AGENTS.md"))
}

func hasManifest(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && analyzer.IsManifest(entry.Name()) {
			return true
		}
	}
	return false
}

func hasWorkspaceMarker(path string) bool {
	for _, name := range []string{"go.work", "pnpm-workspace.yaml", "lerna.json", "nx.json", "global.json", "Directory.Build.props"} {
		if fileExists(filepath.Join(path, name)) {
			return true
		}
	}
	for _, name := range []string{"package.json", "Cargo.toml", "pyproject.toml", "pom.xml", "settings.gradle", "settings.gradle.kts"} {
		data, err := os.ReadFile(filepath.Join(path, name))
		if err == nil && (bytes.Contains(data, []byte("workspaces")) ||
			bytes.Contains(data, []byte("[workspace]")) ||
			bytes.Contains(data, []byte("[tool.uv.workspace]")) ||
			bytes.Contains(data, []byte("<modules>")) ||
			bytes.Contains(data, []byte("include(")) ||
			bytes.Contains(data, []byte("include "))) {
			return true
		}
	}
	entries, _ := os.ReadDir(path)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".sln") {
			return true
		}
	}
	return false
}

func shouldIndex(relativePath string) bool {
	name := filepath.Base(relativePath)
	lowerName := strings.ToLower(name)
	if _, sensitive := sensitiveNames[lowerName]; sensitive || strings.HasPrefix(lowerName, ".env.") || strings.HasSuffix(lowerName, ".pem") || strings.HasSuffix(lowerName, ".key") {
		return false
	}
	if strings.HasSuffix(name, "_gen.go") || name == "wire_gen.go" || strings.HasSuffix(name, ".min.js") ||
		name == "go.sum" || strings.HasPrefix(lowerName, ".golangci") || strings.Contains(lowerName, "package-lock") {
		return false
	}
	if analyzer.IsManifest(name) || name == "Makefile" || name == "Dockerfile" || strings.HasPrefix(lowerName, "readme") || name == "AGENTS.md" {
		return true
	}
	_, ok := indexedExtensions[strings.ToLower(filepath.Ext(name))]
	return ok
}

func resolveDependencies(projects []model.Project) {
	identities := map[string]string{}
	for _, project := range projects {
		identities[strings.ToLower(project.Name)] = project.Name
		for _, alias := range project.Aliases {
			identities[strings.ToLower(alias)] = project.Name
		}
		for _, identifier := range project.Identifiers {
			identities[strings.ToLower(identifier)] = project.Name
			identities[strings.ToLower(filepath.Base(identifier))] = project.Name
		}
	}
	for i := range projects {
		resolved := []string{}
		for _, dependency := range projects[i].Dependencies {
			lower := strings.ToLower(dependency)
			base := strings.ToLower(filepath.Base(dependency))
			if name, ok := identities[lower]; ok && name != projects[i].Name {
				resolved = append(resolved, name)
			} else if name, ok := identities[base]; ok && name != projects[i].Name {
				resolved = append(resolved, name)
			}
		}
		projects[i].Dependencies = uniqueStrings(resolved)
	}
}

func findOverride(overrides map[string]config.ProjectOverride, name, path, root string) config.ProjectOverride {
	if override, ok := overrides[name]; ok {
		return override
	}
	rel, _ := filepath.Rel(root, path)
	for _, override := range overrides {
		if override.Path != "" && filepath.Clean(override.Path) == filepath.Clean(rel) {
			return override
		}
	}
	return config.ProjectOverride{}
}

func inferDescription(path, content string) string {
	for _, fileName := range []string{"README.md", "README.MD", "readme.md", "AGENTS.md"} {
		file, err := os.Open(filepath.Join(path, fileName))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(io.LimitReader(file, 32*1024))
		for scanner.Scan() {
			line := strings.TrimSpace(strings.TrimLeft(scanner.Text(), "#>-* "))
			if len([]rune(line)) >= 12 && !strings.HasPrefix(line, "http") && !strings.HasPrefix(line, "```") {
				_ = file.Close()
				return line
			}
		}
		_ = file.Close()
	}
	if index := strings.Index(content, "module "); index >= 0 {
		line := content[index:]
		if end := strings.IndexByte(line, '\n'); end >= 0 {
			line = line[:end]
		}
		return strings.TrimSpace(line)
	}
	return ""
}

func readPrefix(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, limit))
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
func excluded(name string, excludes []string) bool {
	name = filepath.ToSlash(name)
	for _, pattern := range excludes {
		pattern = filepath.ToSlash(pattern)
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(name)); matched {
			return true
		}
	}
	return false
}
func included(name string, includes []string) bool {
	if len(includes) == 0 {
		return true
	}
	name = filepath.ToSlash(name)
	for _, pattern := range includes {
		pattern = filepath.ToSlash(pattern)
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(name)); matched {
			return true
		}
	}
	return false
}
func isIgnoredDir(name string) bool {
	_, ignored := ignoredDirs[name]
	return ignored
}
func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
func sortedSet(values map[string]struct{}) []string { return sortedKeys(values) }
func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
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
