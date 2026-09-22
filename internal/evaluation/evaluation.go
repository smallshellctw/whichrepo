package evaluation

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/smallshellctw/whichrepo/internal/router"
)

type Case struct {
	Task             string   `json:"task"`
	ExpectedProjects []string `json:"expected_projects"`
}

type Report struct {
	Cases        int     `json:"cases"`
	Top1Hits     int     `json:"top_1_hits"`
	Top3Hits     int     `json:"top_3_hits"`
	Top1Accuracy float64 `json:"top_1_accuracy"`
	Top3Accuracy float64 `json:"top_3_accuracy"`
	Failures     []any   `json:"failures,omitempty"`
}

func Run(ctx context.Context, engine router.Router, path string) (Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer file.Close()
	report := Report{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		var testCase Case
		if err := json.Unmarshal(scanner.Bytes(), &testCase); err != nil {
			return Report{}, fmt.Errorf("decode case %d: %w", report.Cases+1, err)
		}
		result, err := engine.Route(ctx, testCase.Task, 8)
		if err != nil {
			return Report{}, err
		}
		report.Cases++
		expected := make(map[string]struct{}, len(testCase.ExpectedProjects))
		for _, name := range testCase.ExpectedProjects {
			expected[name] = struct{}{}
		}
		if _, ok := expected[result.PrimaryProject]; ok {
			report.Top1Hits++
		}
		top3 := false
		for index, candidate := range result.Candidates {
			if index >= 3 {
				break
			}
			if _, ok := expected[candidate.Project]; ok {
				top3 = true
			}
		}
		if top3 {
			report.Top3Hits++
		} else {
			report.Failures = append(report.Failures, map[string]any{"task": testCase.Task, "expected": testCase.ExpectedProjects, "actual": result.PrimaryProject})
		}
	}
	if err := scanner.Err(); err != nil {
		return Report{}, err
	}
	if report.Cases > 0 {
		report.Top1Accuracy = float64(report.Top1Hits) / float64(report.Cases)
		report.Top3Accuracy = float64(report.Top3Hits) / float64(report.Cases)
	}
	return report, nil
}
