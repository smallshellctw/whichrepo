package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	_ "modernc.org/sqlite"

	"github.com/smallshellctw/whichrepo/internal/model"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create index directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS projects_v2 (
    name TEXT PRIMARY KEY,
    path TEXT NOT NULL,
    description TEXT NOT NULL,
    aliases_json TEXT NOT NULL,
    languages_json TEXT NOT NULL,
    dependencies_json TEXT NOT NULL,
    manifests_json TEXT NOT NULL,
    file_count INTEGER NOT NULL,
    content TEXT NOT NULL,
    indexed_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_projects_v2_path ON projects_v2(path);
`)
	return err
}

func (s *Store) ReplaceProjects(ctx context.Context, projects []model.Project) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM projects_v2"); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO projects_v2
(name, path, description, aliases_json, languages_json, dependencies_json, manifests_json, file_count, content, indexed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, project := range projects {
		aliases, _ := json.Marshal(project.Aliases)
		languages, _ := json.Marshal(project.Languages)
		dependencies, _ := json.Marshal(project.Dependencies)
		manifests, _ := json.Marshal(project.Manifests)
		if _, err := stmt.ExecContext(ctx, project.Name, project.Path, project.Description, string(aliases), string(languages), string(dependencies), string(manifests), project.FileCount, project.Content, project.IndexedAt.Format("2006-01-02T15:04:05Z07:00")); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Projects(ctx context.Context) ([]model.Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, path, description, aliases_json, languages_json, dependencies_json, manifests_json, file_count, content, indexed_at FROM projects_v2 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []model.Project
	for rows.Next() {
		var project model.Project
		var aliases, languages, dependencies, manifests, indexedAt string
		if err := rows.Scan(&project.Name, &project.Path, &project.Description, &aliases, &languages, &dependencies, &manifests, &project.FileCount, &project.Content, &indexedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(aliases), &project.Aliases)
		_ = json.Unmarshal([]byte(languages), &project.Languages)
		_ = json.Unmarshal([]byte(dependencies), &project.Dependencies)
		_ = json.Unmarshal([]byte(manifests), &project.Manifests)
		_ = project.IndexedAt.UnmarshalText([]byte(indexedAt))
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Store) Search(ctx context.Context, query string, limit int) ([]model.Candidate, error) {
	projects, err := s.Projects(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 8
	}
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return []model.Candidate{}, nil
	}
	docTokens := make([]map[string]int, len(projects))
	docLengths := make([]int, len(projects))
	documentFrequency := make(map[string]int)
	for i, project := range projects {
		tokens := tokenize(project.Name + " " + strings.Join(project.Aliases, " ") + " " + project.Description + " " + project.Content)
		frequencies := make(map[string]int, len(tokens))
		for _, token := range tokens {
			frequencies[token]++
		}
		docTokens[i] = frequencies
		docLengths[i] = len(tokens)
		for _, queryToken := range unique(queryTokens) {
			if frequencies[queryToken] > 0 {
				documentFrequency[queryToken]++
			}
		}
	}
	averageLength := 1.0
	if len(docLengths) > 0 {
		var sum int
		for _, length := range docLengths {
			sum += length
		}
		averageLength = math.Max(1, float64(sum)/float64(len(docLengths)))
	}

	candidates := make([]model.Candidate, 0, len(projects))
	for i, project := range projects {
		score := bm25(queryTokens, docTokens[i], docLengths[i], averageLength, documentFrequency, len(projects))
		lowerQuery := strings.ToLower(query)
		if strings.Contains(lowerQuery, strings.ToLower(project.Name)) {
			score += 12
		}
		for _, alias := range project.Aliases {
			if alias != "" && strings.Contains(lowerQuery, strings.ToLower(alias)) {
				score += 7
			}
		}
		if score <= 0 {
			continue
		}
		candidates = append(candidates, model.Candidate{
			Project:     project.Name,
			Path:        project.Path,
			Description: project.Description,
			LocalScore:  score,
			Evidence:    evidence(project, queryTokens, 4),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].LocalScore == candidates[j].LocalScore {
			return candidates[i].Project < candidates[j].Project
		}
		return candidates[i].LocalScore > candidates[j].LocalScore
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func bm25(query []string, frequencies map[string]int, documentLength int, averageLength float64, documentFrequency map[string]int, documentCount int) float64 {
	const k1, b = 1.5, 0.75
	var score float64
	for _, token := range unique(query) {
		tf := float64(frequencies[token])
		if tf == 0 {
			continue
		}
		df := float64(documentFrequency[token])
		idf := math.Log(1 + (float64(documentCount)-df+0.5)/(df+0.5))
		normalized := tf + k1*(1-b+b*float64(documentLength)/averageLength)
		score += idf * tf * (k1 + 1) / normalized
	}
	return score
}

func tokenize(value string) []string {
	value = strings.ToLower(value)
	var tokens []string
	var word []rune
	flushWord := func() {
		if len(word) >= 2 {
			tokens = append(tokens, string(word))
		}
		word = word[:0]
	}
	var hanRun []rune
	flushHan := func() {
		for i := 0; i+1 < len(hanRun); i++ {
			tokens = append(tokens, string(hanRun[i:i+2]))
		}
		hanRun = hanRun[:0]
	}
	for _, r := range value {
		switch {
		case unicode.Is(unicode.Han, r):
			flushWord()
			hanRun = append(hanRun, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-':
			flushHan()
			word = append(word, r)
		default:
			flushWord()
			flushHan()
		}
	}
	flushWord()
	flushHan()
	return tokens
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func evidence(project model.Project, queryTokens []string, limit int) []string {
	var results []string
	seen := map[string]struct{}{}
	sources := []string{
		"Project description: " + project.Description,
		"Project aliases: " + strings.Join(project.Aliases, ", "),
	}
	sources = append(sources, strings.Split(project.Content, "\n")...)
	for _, line := range sources {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || len([]rune(trimmed)) > 240 {
			continue
		}
		lineTokens := tokenize(trimmed)
		matched := false
		for _, queryToken := range unique(queryTokens) {
			for _, lineToken := range lineTokens {
				if queryToken == lineToken {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		results = append(results, trimmed)
		if len(results) >= limit {
			break
		}
	}
	return results
}
