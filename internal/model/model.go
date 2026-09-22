package model

import "time"

type Project struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Description  string    `json:"description"`
	Aliases      []string  `json:"aliases,omitempty"`
	Languages    []string  `json:"languages,omitempty"`
	Dependencies []string  `json:"dependencies,omitempty"`
	Manifests    []string  `json:"manifests,omitempty"`
	FileCount    int       `json:"file_count"`
	Content      string    `json:"-"`
	IndexedAt    time.Time `json:"indexed_at"`
}

type Candidate struct {
	Project       string   `json:"project"`
	Path          string   `json:"path"`
	Description   string   `json:"description,omitempty"`
	LocalScore    float64  `json:"local_score"`
	SemanticScore *float64 `json:"semantic_score,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

type RouteResult struct {
	Mode                    string         `json:"mode"`
	DecisionProvider        string         `json:"decision_provider"`
	ProviderStatus          string         `json:"provider_status"`
	PrimaryProject          string         `json:"primary_project,omitempty"`
	RelatedProjects         []string       `json:"related_projects,omitempty"`
	ChangeType              string         `json:"change_type,omitempty"`
	NeedsClarification      *bool          `json:"needs_clarification,omitempty"`
	CrossProject            *bool          `json:"cross_project,omitempty"`
	RequirementCompleteness *float64       `json:"requirement_completeness,omitempty"`
	ChangeRisk              *float64       `json:"change_risk,omitempty"`
	MissingDimensions       []string       `json:"missing_dimensions,omitempty"`
	Candidates              []Candidate    `json:"candidates"`
	Warnings                []string       `json:"warnings,omitempty"`
	Model                   string         `json:"model,omitempty"`
	Usage                   *Usage         `json:"usage,omitempty"`
	ProviderMetadata        map[string]any `json:"provider_metadata,omitempty"`
}

type WorkspaceStatus struct {
	Workspace        string    `json:"workspace"`
	ProjectCount     int       `json:"project_count"`
	LastIndexedAt    time.Time `json:"last_indexed_at,omitempty"`
	DecisionProvider string    `json:"decision_provider"`
}
