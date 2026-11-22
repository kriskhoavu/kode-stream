package item

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"

	"gopkg.in/yaml.v3"
)

func (s *Service) VerificationTests(id string) (models.ItemVerificationTests, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.ItemVerificationTests{}, err
	}
	selection, err := s.writer.VerificationTests(workspace, item)
	if err != nil {
		return models.ItemVerificationTests{}, err
	}
	discovered, err := DiscoverVerificationSpecs(workspace, item)
	if err != nil {
		return models.ItemVerificationTests{}, err
	}
	return models.ItemVerificationTests{Selection: selection, DiscoveredSpecs: discovered}, nil
}

func (s *Service) SaveVerificationTests(id string, input models.VerificationTestSelection) (models.ItemVerificationTests, error) {
	workspace, item, err := s.workspaceAndItem(id)
	if err != nil {
		return models.ItemVerificationTests{}, err
	}
	if err := s.withItemMutation(workspace, item, func(current models.ItemDetail) error {
		_, err := s.writer.SaveVerificationTests(workspace, current, input)
		return err
	}); err != nil {
		return models.ItemVerificationTests{}, err
	}
	selection, err := s.writer.VerificationTests(workspace, item)
	if err != nil {
		return models.ItemVerificationTests{}, err
	}
	discovered, err := DiscoverVerificationSpecs(workspace, item)
	if err != nil {
		return models.ItemVerificationTests{}, err
	}
	return models.ItemVerificationTests{Selection: selection, DiscoveredSpecs: discovered}, nil
}

func DiscoverVerificationSpecs(workspace models.WorkspaceConfig, item models.ItemDetail) ([]models.DiscoveredVerificationSpec, error) {
	if workspace.Runtime == nil || workspace.Runtime.Automation == nil || !workspace.Runtime.Automation.Enabled {
		return []models.DiscoveredVerificationSpec{}, nil
	}
	specs := []models.DiscoveredVerificationSpec{}
	seen := map[string]struct{}{}
	if item.SourceMode == "snapshot" {
		commit, err := snapshotCommit(item)
		if err != nil {
			return nil, err
		}
		sourcePath := filepath.ToSlash(filepath.Join(item.ItemPath, "plan.yaml"))
		data, err := gitadapter.New().TreeReadFile(workspace.Path, commit, sourcePath)
		if err != nil {
			if snapshotPathMissing(err) {
				return specs, nil
			}
			return nil, err
		}
		return discoverVerificationSpecsInPlanYAMLData(data, sourcePath, workspace.Runtime.Automation.Runner, seen)
	}
	if workspace.Path != "" && item.ItemPath != "" {
		found, err := discoverVerificationSpecsInPlanYAML(workspace.Path, filepath.Join(workspace.Path, item.ItemPath), workspace.Runtime.Automation.Runner, seen)
		if err != nil {
			return nil, err
		}
		specs = append(specs, found...)
	}
	repoPath := strings.TrimSpace(workspace.Runtime.Automation.RepositoryPath)
	if repoPath == "" {
		sort.Slice(specs, func(i, j int) bool { return specs[i].Path < specs[j].Path })
		return specs, nil
	}
	root, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	plansRoot := filepath.Join(root, "plans")
	for _, candidate := range verificationDiscoveryCandidateRoots(plansRoot, item) {
		found, err := discoverVerificationSpecsInPlanYAML(root, candidate, workspace.Runtime.Automation.Runner, seen)
		if err != nil {
			return nil, err
		}
		specs = append(specs, found...)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Path < specs[j].Path })
	return specs, nil
}

func verificationDiscoveryCandidateRoots(plansRoot string, item models.ItemDetail) []string {
	candidates := []string{}
	add := func(parts ...string) {
		cleanParts := []string{plansRoot}
		for _, part := range parts {
			part = strings.Trim(strings.TrimSpace(part), "/")
			if part == "" || part == "." || part == ".." || strings.Contains(part, "/") {
				return
			}
			cleanParts = append(cleanParts, part)
		}
		path := filepath.Join(cleanParts...)
		for _, existing := range candidates {
			if existing == path {
				return
			}
		}
		candidates = append(candidates, path)
	}
	add(item.Identifier)
	add(item.Scope, item.Identifier)
	add(item.ID)
	return candidates
}

type verificationPlanYAML struct {
	AutomationTests []models.AutomationTestPath `yaml:"automation-test"`
}

func discoverVerificationSpecsInPlanYAML(repoRoot, planRoot string, fallbackRunner models.AutomationRunner, seen map[string]struct{}) ([]models.DiscoveredVerificationSpec, error) {
	data, err := os.ReadFile(filepath.Join(planRoot, "plan.yaml"))
	if os.IsNotExist(err) {
		return []models.DiscoveredVerificationSpec{}, nil
	}
	if err != nil {
		return nil, err
	}
	sourcePath, err := filepath.Rel(repoRoot, filepath.Join(planRoot, "plan.yaml"))
	if err != nil {
		sourcePath = filepath.Join(planRoot, "plan.yaml")
	}
	return discoverVerificationSpecsInPlanYAMLData(data, filepath.ToSlash(sourcePath), fallbackRunner, seen)
}

func discoverVerificationSpecsInPlanYAMLData(data []byte, sourcePath string, fallbackRunner models.AutomationRunner, seen map[string]struct{}) ([]models.DiscoveredVerificationSpec, error) {
	var meta verificationPlanYAML
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	specs := []models.DiscoveredVerificationSpec{}
	for _, entry := range meta.AutomationTests {
		specPath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(entry.Path)))
		if specPath == "." || specPath == "" {
			continue
		}
		if _, ok := seen[specPath]; ok {
			continue
		}
		seen[specPath] = struct{}{}
		specs = append(specs, models.DiscoveredVerificationSpec{
			Path:       specPath,
			Runner:     string(runnerForSpecPath(specPath, fallbackRunner)),
			SourcePath: filepath.ToSlash(sourcePath),
		})
	}
	return specs, nil
}

func snapshotPathMissing(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "does not exist in") || strings.Contains(message, "exists on disk, but not in") || strings.Contains(message, "unknown revision or path")
}

func runnerForSpecPath(path string, fallback models.AutomationRunner) models.AutomationRunner {
	if strings.HasPrefix(path, "cypress/") || strings.Contains(path, ".cy.") {
		return models.AutomationRunnerCypress
	}
	if strings.HasPrefix(path, "playwright/") {
		return models.AutomationRunnerPlaywright
	}
	return fallback
}
