package item

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/pathguard"

	"gopkg.in/yaml.v3"
)

type e2ePlanMetadata struct {
	Plan struct {
		E2ERunbook bool `yaml:"e2e-runbook"`
	} `yaml:"plan"`
}

func (s *Service) E2ERunbooks(id string) (models.E2ERunbookList, []string, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.E2ERunbookList{}, nil, err
	}
	root, err := pathguard.ResolveExisting(workspace.Path, item.ItemPath)
	if err != nil {
		return models.E2ERunbookList{}, nil, err
	}
	metadata, err := readE2EPlanMetadata(filepath.Join(root, "plan.yaml"))
	if err != nil {
		return models.E2ERunbookList{}, nil, err
	}
	if !metadata.Plan.E2ERunbook {
		return models.E2ERunbookList{Runbooks: []models.E2ERunbook{}, Diagnostic: "This plan does not declare reusable E2E coverage."}, []string{}, nil
	}
	automationRoot := filepath.Join(root, "automation")
	entries, err := os.ReadDir(automationRoot)
	if os.IsNotExist(err) {
		return models.E2ERunbookList{Runbooks: []models.E2ERunbook{}, Diagnostic: "No ticket-local E2E runbooks were found."}, []string{}, nil
	}
	if err != nil {
		return models.E2ERunbookList{}, nil, err
	}
	runbooks := make([]models.E2ERunbook, 0)
	sources := make([]string, 0)
	hubRelative := filepath.ToSlash(filepath.Join(item.ItemPath, "automation", "README.md"))
	if _, hubErr := pathguard.ValidateMarkdownFile(workspace.Path, hubRelative); hubErr == nil {
		sources = append(sources, hubRelative)
	}
	for _, entry := range entries {
		if entry.IsDir() || !isE2EScenarioRunbook(entry.Name()) {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(item.ItemPath, "automation", entry.Name()))
		fullPath, pathErr := pathguard.ValidateMarkdownFile(workspace.Path, rel)
		if pathErr != nil {
			continue
		}
		resultRelative := filepath.ToSlash(filepath.Join(filepath.Dir(rel), "results", "latest.md"))
		runbooks = append(runbooks, readE2ERunbook(workspace.Path, fullPath, rel, resultRelative, "plan"))
		sources = append(sources, rel)
	}
	sort.Slice(runbooks, func(i, j int) bool { return runbooks[i].Path < runbooks[j].Path })
	if len(runbooks) == 0 {
		return models.E2ERunbookList{Runbooks: runbooks, Diagnostic: "No ticket-local E2E runbooks were found."}, sources, nil
	}
	return models.E2ERunbookList{Runbooks: runbooks}, sources, nil
}

func isE2EScenarioRunbook(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(name, "scenario-") && strings.HasSuffix(name, ".md")
}

func readE2EPlanMetadata(path string) (e2ePlanMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return e2ePlanMetadata{}, err
	}
	var metadata e2ePlanMetadata
	return metadata, yaml.Unmarshal(data, &metadata)
}

func readE2ERunbook(workspaceRoot, fullPath, relativePath, resultRelativePath, source string) models.E2ERunbook {
	runbook := models.E2ERunbook{
		Title:      strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath)),
		Path:       relativePath,
		Source:     source,
		ResultPath: resultRelativePath,
	}
	if file, err := os.Open(fullPath); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "# ") {
				runbook.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}
		_ = file.Close()
	}
	resultPath, err := pathguard.ValidateMarkdownFile(workspaceRoot, resultRelativePath)
	if err == nil {
		data, readErr := os.ReadFile(resultPath)
		if readErr != nil {
			runbook.Diagnostic = "Latest E2E result could not be read."
			return runbook
		}
		runbook.LatestResult = parseE2ELatestResult(string(data))
	} else if os.IsNotExist(err) {
		runbook.Diagnostic = "Not run"
	} else {
		runbook.Diagnostic = "Latest E2E result path is unsafe or unsupported."
	}
	return runbook
}

func parseE2ELatestResult(content string) *models.E2ELatestResult {
	result := &models.E2ELatestResult{Status: "not run", Evidence: []string{}}
	for _, line := range strings.Split(content, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "status":
			result.Status = strings.ToLower(strings.TrimSpace(value))
		case "provider":
			result.Provider = strings.TrimSpace(value)
		case "environment":
			result.Environment = strings.TrimSpace(value)
		case "failed step":
			result.FailedStep = strings.TrimSpace(value)
		case "evidence":
			if evidence := strings.TrimSpace(value); evidence != "" {
				result.Evidence = append(result.Evidence, evidence)
			}
		}
	}
	if !validE2EStatus(result.Status) {
		result.Status = "not run"
	}
	return result
}

func validE2EStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "passed", "failed", "blocked", "not run":
		return true
	default:
		return false
	}
}
