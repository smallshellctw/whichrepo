package router

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/smallshellctw/whichrepo/internal/gateway"
	"github.com/smallshellctw/whichrepo/internal/index"
	"github.com/smallshellctw/whichrepo/internal/model"
)

type Router struct {
	Store   *index.Store
	Gateway gateway.Client
}

func (r Router) Route(ctx context.Context, requirement string, topK int) (model.RouteResult, error) {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return model.RouteResult{}, fmt.Errorf("requirement must not be empty")
	}
	if topK <= 0 || topK > 20 {
		topK = 8
	}
	candidates, err := r.Store.Search(ctx, requirement, topK)
	if err != nil {
		return model.RouteResult{}, err
	}
	result := localResult(candidates)
	projects, _ := r.Store.Projects(ctx)
	applyLocalSignals(&result, candidates, projects, requirement)
	if len(candidates) == 0 {
		result.Warnings = append(result.Warnings, "The local index returned no candidate projects. Refresh the index or add project metadata.")
		return result, nil
	}
	if !r.Gateway.Available() {
		result.ProviderStatus = "disabled_missing_api_key"
		result.Warnings = append(result.Warnings, "No decision provider API key is configured; returning local retrieval results.")
		return result, nil
	}

	state := buildState(requirement, candidates)
	questions := buildQuestions(candidates)
	response, err := r.Gateway.Evaluate(ctx, state, questions)
	if err != nil {
		result.ProviderStatus = "provider_error"
		if providerErr, ok := err.(*gateway.ProviderError); ok {
			result.ProviderStatus = string(providerErr.Kind)
		}
		result.Warnings = append(result.Warnings, "Decision provider failed; returned local retrieval results: "+err.Error())
		return result, nil
	}
	applyJev(&result, candidates, response)
	if r.Gateway.Provider != "" {
		result.DecisionProvider = r.Gateway.Provider
	}
	result.ProviderMetadata = map[string]any{"model": response.Model}
	return result, nil
}

func localResult(candidates []model.Candidate) model.RouteResult {
	result := model.RouteResult{
		Mode:             "local_retrieval_only",
		DecisionProvider: "local",
		ProviderStatus:   "not_called",
		Candidates:       append([]model.Candidate(nil), candidates...),
	}
	if len(candidates) > 0 {
		result.PrimaryProject = candidates[0].Project
	}
	return result
}

func applyLocalSignals(result *model.RouteResult, candidates []model.Candidate, projects []model.Project, task string) {
	if len(candidates) == 0 {
		value := true
		result.NeedsClarification = &value
		return
	}
	projectByName := map[string]model.Project{}
	for _, project := range projects {
		projectByName[project.Name] = project
	}
	primary := projectByName[result.PrimaryProject]
	links := map[string]struct{}{}
	for _, name := range primary.Dependencies {
		links[name] = struct{}{}
	}
	for _, project := range projects {
		for _, dependency := range project.Dependencies {
			if dependency == primary.Name {
				links[project.Name] = struct{}{}
			}
		}
	}
	best := candidates[0].LocalScore
	for index, candidate := range candidates {
		if index == 0 {
			continue
		}
		_, linked := links[candidate.Project]
		if linked || (best > 0 && candidate.LocalScore/best >= 0.55) {
			result.RelatedProjects = append(result.RelatedProjects, candidate.Project)
		}
	}
	clarification := len(candidates) > 1 && best > 0 && candidates[1].LocalScore/best >= 0.82
	result.NeedsClarification = &clarification
	cross := len(result.RelatedProjects) > 0
	result.CrossProject = &cross
	lower := strings.ToLower(task)
	switch {
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "retry") || strings.Contains(lower, "deadline"):
		result.ChangeType = "reliability"
	case strings.Contains(lower, "database") || strings.Contains(lower, "schema") || strings.Contains(lower, "migration") || strings.Contains(lower, "sql"):
		result.ChangeType = "database"
	case strings.Contains(lower, "config") || strings.Contains(lower, "deployment") || strings.Contains(lower, "kubernetes"):
		result.ChangeType = "configuration"
	case strings.Contains(lower, "api") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "rpc"):
		result.ChangeType = "api"
	case strings.Contains(lower, "fix") || strings.Contains(lower, "bug") || strings.Contains(lower, "error"):
		result.ChangeType = "bug_fix"
	default:
		result.ChangeType = "other"
	}
}

func buildState(requirement string, candidates []model.Candidate) map[string]any {
	items := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, map[string]any{
			"project":     candidate.Project,
			"description": candidate.Description,
			"evidence":    sanitizeEvidence(candidate.Evidence),
		})
	}
	return map[string]any{
		"requirement": requirement,
		"candidates":  items,
		"instruction": "Judge only from the requirement and supplied candidate evidence. Do not invent projects or files.",
	}
}

func buildQuestions(candidates []model.Candidate) map[string]gateway.Question {
	criteria := make(map[string]any, len(candidates))
	for _, candidate := range candidates {
		criteria[candidate.Project] = map[string]any{
			"description": candidate.Description,
			"evidence":    candidate.Evidence,
		}
	}
	questions := map[string]gateway.Question{
		"primary_project": {
			Type:         "choice",
			Instructions: "Which candidate project is the primary implementation target for the requirement?",
			Criteria:     criteria,
		},
		"need_clarification": {
			Type:         "noul",
			Instructions: "Is a material clarification required before an engineer can safely decide the target project and intended change?",
		},
		"cross_project": {
			Type:         "noul",
			Instructions: "Does the requirement likely require coordinated code changes in more than one candidate project?",
		},
		"change_type": {
			Type:         "choice",
			Instructions: "What is the dominant engineering change type?",
			Criteria: map[string]any{
				"business_logic": "Business behavior or domain logic",
				"api":            "HTTP, RPC, protobuf, or public interface",
				"configuration":  "Configuration values or feature switches",
				"database":       "Schema, query, migration, or repository persistence",
				"timeout":        "Timeout, deadline, retry, or connection timing",
				"bug_fix":        "Defect investigation and correction",
				"other":          "None of the listed types clearly dominates",
			},
		},
		"requirement_completeness": {
			Type:         "score",
			Instructions: "How complete is the requirement for locating the project and beginning a read-only code investigation?",
			Criteria: []string{
				"Critical target and expected behavior are missing",
				"Several material details are missing",
				"Mostly clear but one clarification may be needed",
				"Clear enough to begin investigation",
				"Explicit target, behavior, constraints, and acceptance expectation",
			},
		},
		"change_risk": {
			Type:         "score",
			Instructions: "Estimate implementation risk using only the requirement and supplied evidence.",
			Criteria: []string{
				"Trivial and isolated",
				"Low risk",
				"Moderate risk or uncertain blast radius",
				"High risk, cross-service, data, or compatibility impact",
				"Critical production or irreversible impact",
			},
		},
		"missing_target_scope": {
			Type:         "noul",
			Instructions: "Is the intended project or service scope materially ambiguous?",
		},
		"missing_expected_behavior": {
			Type:         "noul",
			Instructions: "Is the desired resulting behavior materially underspecified?",
		},
		"missing_acceptance_criteria": {
			Type:         "noul",
			Instructions: "Are acceptance or verification expectations materially missing for this change?",
		},
	}
	for i, candidate := range candidates {
		questions[fmt.Sprintf("candidate_%d_relevant", i)] = gateway.Question{
			Type: "noul",
			Instructions: map[string]any{
				"question":  "Is `candidate` likely to require a code or configuration change for this requirement?",
				"candidate": candidate.Project,
				"evidence":  sanitizeEvidence(candidate.Evidence),
			},
		}
	}
	return questions
}

var secretAssignment = regexp.MustCompile(`(?i)(password|passwd|secret|api[_-]?key|access[_-]?key|private[_-]?key|authorization|dsn)\s*[:=].*`)

func sanitizeEvidence(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, secretAssignment.ReplaceAllString(line, "$1=[REDACTED]"))
	}
	return result
}

func applyJev(result *model.RouteResult, candidates []model.Candidate, response gateway.Response) {
	result.Mode = "semantic_routed"
	result.DecisionProvider = "jev"
	result.ProviderStatus = "ok"
	result.Model = response.Model
	result.Usage = &model.Usage{InputTokens: response.Usage.InputTokens, OutputTokens: response.Usage.OutputTokens}
	if answer, ok := response.Answers["primary_project"]; ok {
		result.PrimaryProject = answer.Choice
		for i := range result.Candidates {
			if probability, exists := answer.Probabilities[result.Candidates[i].Project]; exists {
				value := probability
				result.Candidates[i].SemanticScore = &value
			}
		}
	}
	if answer, ok := response.Answers["need_clarification"]; ok {
		value := answer.Noul >= 0.6
		result.NeedsClarification = &value
	}
	if answer, ok := response.Answers["cross_project"]; ok {
		value := answer.Noul >= 0.6
		result.CrossProject = &value
	}
	if answer, ok := response.Answers["change_type"]; ok {
		result.ChangeType = answer.Choice
	}
	if answer, ok := response.Answers["requirement_completeness"]; ok {
		value := answer.Score
		result.RequirementCompleteness = &value
	}
	if answer, ok := response.Answers["change_risk"]; ok {
		value := answer.Score
		result.ChangeRisk = &value
	}
	for i, candidate := range candidates {
		if answer, ok := response.Answers[fmt.Sprintf("candidate_%d_relevant", i)]; ok && answer.Noul >= 0.6 && candidate.Project != result.PrimaryProject {
			result.RelatedProjects = append(result.RelatedProjects, candidate.Project)
		}
	}
	missingQuestions := []struct {
		ID   string
		Name string
	}{
		{"missing_target_scope", "target_scope"},
		{"missing_expected_behavior", "expected_behavior"},
		{"missing_acceptance_criteria", "acceptance_criteria"},
	}
	for _, item := range missingQuestions {
		if answer, ok := response.Answers[item.ID]; ok && answer.Noul >= 0.6 {
			result.MissingDimensions = append(result.MissingDimensions, item.Name)
		}
	}
}
