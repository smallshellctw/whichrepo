package scanner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

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
	".php": {}, ".js": {}, ".jsx": {}, ".ts": {}, ".tsx": {}, ".vue": {},
	".sql": {}, ".toml": {}, ".mod": {}, ".gradle": {}, ".xml": {}, ".py": {},
	".rs": {}, ".java": {}, ".kt": {}, ".swift": {}, ".rb": {}, ".cs": {},
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
	for _, path := range paths {
		name := filepath.Base(path)
		override := findOverride(s.Config.Projects, name, path, s.Root)
		project, err := s.scanProject(name, path, override)
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
		if looksLikeProject(path) {
			add(path)
		}
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
				if _, ignored := ignoredDirs[entry.Name()]; ignored {
					return filepath.SkipDir
				}
				if hasManifest(path) {
					add(path)
				}
			}
			return nil
		})
	}
	return sortedKeys(seen), nil
}

func (s Scanner) scanProject(name, path string, override config.ProjectOverride) (model.Project, error) {
	files := projectFiles(path)
	if len(files) > maxFilesPerProject {
		files = files[:maxFilesPerProject]
	}
	var builder strings.Builder
	var totalBytes int
	languages := map[string]struct{}{}
	manifests := []string{}
	rawDependencies := []string{}
	for _, relativePath := range files {
		if totalBytes >= maxBytesPerProject || !shouldIndex(relativePath) {
			continue
		}
		filePath := filepath.Join(path, relativePath)
		ext := strings.ToLower(filepath.Ext(relativePath))
		if language := languageForExtension(ext); language != "" {
			languages[language] = struct{}{}
		}
		data, err := readPrefix(filePath, maxBytesPerFile)
		if err != nil || !utf8.Valid(data) {
			continue
		}
		builder.WriteString("\nFILE ")
		builder.WriteString(filepath.ToSlash(relativePath))
		builder.WriteByte('\n')
		builder.Write(data)
		totalBytes += len(data)
		if isManifest(filepath.Base(relativePath)) {
			manifests = append(manifests, filepath.ToSlash(relativePath))
			rawDependencies = append(rawDependencies, parseManifest(relativePath, data)...)
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
	for _, name := range []string{"go.mod", "package.json", "pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle", "build.gradle.kts"} {
		if fileExists(filepath.Join(path, name)) {
			return true
		}
	}
	return false
}

func hasWorkspaceMarker(path string) bool {
	for _, name := range []string{"go.work", "pnpm-workspace.yaml", "lerna.json", "nx.json"} {
		if fileExists(filepath.Join(path, name)) {
			return true
		}
	}
	for _, name := range []string{"package.json", "Cargo.toml"} {
		data, err := os.ReadFile(filepath.Join(path, name))
		if err == nil && (bytes.Contains(data, []byte("workspaces")) || bytes.Contains(data, []byte("[workspace]"))) {
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
	if isManifest(name) || name == "Makefile" || name == "Dockerfile" || strings.HasPrefix(lowerName, "readme") || name == "AGENTS.md" {
		return true
	}
	_, ok := indexedExtensions[strings.ToLower(filepath.Ext(name))]
	return ok
}

func isManifest(name string) bool {
	switch strings.ToLower(name) {
	case "go.mod", "go.work", "package.json", "pnpm-workspace.yaml", "pyproject.toml", "cargo.toml", "pom.xml", "build.gradle", "build.gradle.kts":
		return true
	default:
		return false
	}
}

func parseManifest(path string, data []byte) []string {
	name := strings.ToLower(filepath.Base(path))
	var dependencies []string
	switch name {
	case "go.mod":
		scanner := bufio.NewScanner(bytes.NewReader(data))
		inRequire := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "module ") {
				dependencies = append(dependencies, strings.TrimSpace(strings.TrimPrefix(line, "module ")))
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
					dependencies = append(dependencies, fields[0])
				}
			}
		}
	case "package.json":
		var pkg struct {
			Name                 string            `json:"name"`
			Dependencies         map[string]string `json:"dependencies"`
			DevDependencies      map[string]string `json:"devDependencies"`
			PeerDependencies     map[string]string `json:"peerDependencies"`
			OptionalDependencies map[string]string `json:"optionalDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			dependencies = append(dependencies, pkg.Name)
			for _, values := range []map[string]string{pkg.Dependencies, pkg.DevDependencies, pkg.PeerDependencies, pkg.OptionalDependencies} {
				for dependency := range values {
					dependencies = append(dependencies, dependency)
				}
			}
		}
	}
	return dependencies
}

func resolveDependencies(projects []model.Project) {
	identities := map[string]string{}
	for _, project := range projects {
		identities[strings.ToLower(project.Name)] = project.Name
		for _, alias := range project.Aliases {
			identities[strings.ToLower(alias)] = project.Name
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

func languageForExtension(ext string) string {
	switch ext {
	case ".go", ".mod":
		return "Go"
	case ".php":
		return "PHP"
	case ".js", ".jsx":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".vue":
		return "Vue"
	case ".proto":
		return "Protocol Buffers"
	case ".sql":
		return "SQL"
	case ".java", ".gradle", ".kt":
		return "JVM"
	case ".py":
		return "Python"
	case ".rs":
		return "Rust"
	case ".swift":
		return "Swift"
	case ".rb":
		return "Ruby"
	case ".cs":
		return "C#"
	default:
		return ""
	}
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
func excluded(name string, excludes []string) bool {
	for _, pattern := range excludes {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
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
