package planwriter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"plan-manager/internal/fileaccess"
	"plan-manager/internal/models"
	"plan-manager/internal/planindex"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
	"plan-manager/internal/writeguard"
)

type Writer struct {
	files    *fileaccess.Access
	scanner  *scanner.Scanner
	index    *planindex.Index
	registry *registry.Registry
}

func New(files *fileaccess.Access, scan *scanner.Scanner, idx *planindex.Index, reg *registry.Registry) *Writer {
	return &Writer{files: files, scanner: scan, index: idx, registry: reg}
}

func (w *Writer) SaveMarkdown(repo models.RepositoryConfig, plan models.PlanDetail, input models.FileSaveInput) (models.WriteResult, error) {
	if strings.TrimSpace(input.FileID) == "" {
		return models.WriteResult{}, fmt.Errorf("file ID is required")
	}
	if _, err := w.files.WriteMarkdown(repo, plan, input); err != nil {
		return models.WriteResult{}, err
	}
	return w.refresh(repo, plan.PlanRoot)
}

func (w *Writer) SaveMetadata(repo models.RepositoryConfig, plan models.PlanDetail, input models.PlanMetadataUpdateInput) (models.WriteResult, error) {
	if isDocsRoot(plan) {
		return models.WriteResult{}, fmt.Errorf("freestyle docs roots do not support plan metadata")
	}
	if input.Status != "" {
		if err := writeguard.ValidateStatus(input.Status); err != nil {
			return models.WriteResult{}, err
		}
	}
	if input.Service != "" {
		if err := writeguard.ValidateServiceName(input.Service); err != nil {
			return models.WriteResult{}, err
		}
	}
	if input.Ticket != "" {
		if err := writeguard.ValidateTicketName(input.Ticket); err != nil {
			return models.WriteResult{}, err
		}
	}
	meta, err := readPlanMetadata(repo, plan)
	if err != nil {
		return models.WriteResult{}, err
	}
	applyMetadata(&meta, plan, input)
	if err := writePlanMetadata(repo, plan, meta); err != nil {
		return models.WriteResult{}, err
	}
	return w.refresh(repo, plan.PlanRoot)
}

func (w *Writer) UpdateStatus(repo models.RepositoryConfig, plan models.PlanDetail, input models.PlanStatusUpdateInput) (models.WriteResult, error) {
	return w.SaveMetadata(repo, plan, models.PlanMetadataUpdateInput{Status: input.Status})
}

func (w *Writer) CreatePlan(repo models.RepositoryConfig, input models.NewPlanInput) (models.WriteResult, error) {
	planDir, err := validatePlanDirectory(repo, input.PlanDirectory)
	if err != nil {
		return models.WriteResult{}, err
	}
	if err := writeguard.ValidateServiceName(input.Service); err != nil {
		return models.WriteResult{}, err
	}
	if err := writeguard.ValidateTicketName(input.Ticket); err != nil {
		return models.WriteResult{}, err
	}
	status := input.Status
	if status == "" {
		status = models.StatusDraft
	}
	if err := writeguard.ValidateStatus(status); err != nil {
		return models.WriteResult{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = input.Ticket
	}
	planRoot := filepath.ToSlash(filepath.Join(planDir, input.Service, input.Ticket))
	fullRoot, err := safeJoin(repo.Path, planRoot)
	if err != nil {
		return models.WriteResult{}, err
	}
	if _, err := os.Stat(fullRoot); err == nil {
		return models.WriteResult{}, fmt.Errorf("plan already exists")
	} else if !os.IsNotExist(err) {
		return models.WriteResult{}, err
	}
	if err := os.MkdirAll(filepath.Join(fullRoot, "scenario"), 0o755); err != nil {
		return models.WriteResult{}, err
	}
	if err := os.MkdirAll(filepath.Join(fullRoot, "design"), 0o755); err != nil {
		return models.WriteResult{}, err
	}
	files := map[string]string{
		"README.md":                        "# " + input.Ticket + ": " + title + "\n\n## Overview\n\n",
		"scenario/scenario-00-overview.md": "# Scenario Overview\n\n",
		"design/design-01-backend.md":      "# Backend Design\n\n",
		"design/design-02-frontend.md":     "# Frontend Design\n\n",
		"implementation-plan.md":           "# Implementation Plan\n\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(fullRoot, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			return models.WriteResult{}, err
		}
	}
	meta := planYAML{
		Plan: planFields{
			Ticket:  input.Ticket,
			Title:   title,
			Service: input.Service,
			Status:  string(status),
			Owner:   strings.TrimSpace(input.Owner),
			Tags:    cleanTags(input.Tags),
		},
		Documents: starterDocuments(),
	}
	if err := writePlanMetadataAt(fullRoot, meta); err != nil {
		return models.WriteResult{}, err
	}
	return w.refresh(repo, planRoot)
}

func (w *Writer) refresh(repo models.RepositoryConfig, planRoot string) (models.WriteResult, error) {
	scannedAt := time.Now().UTC()
	if w.scanner == nil || w.index == nil {
		return models.WriteResult{ScannedAt: scannedAt}, nil
	}
	data, err := w.scanner.Scan(repo)
	if err != nil {
		return models.WriteResult{}, err
	}
	if err := w.index.ReplaceRepository(repo.ID, data.Plans, data.Warnings, scannedAt); err != nil {
		return models.WriteResult{}, err
	}
	if w.registry != nil {
		_ = w.registry.TouchScanned(repo.ID, scannedAt)
	}
	for _, plan := range data.Plans {
		if plan.PlanRoot == planRoot {
			return models.WriteResult{Plan: plan, ScannedAt: scannedAt}, nil
		}
	}
	return models.WriteResult{ScannedAt: scannedAt}, nil
}

type planYAML struct {
	Plan      planFields            `yaml:"plan"`
	Documents []models.PlanDocument `yaml:"documents,omitempty"`
}

type planFields struct {
	Ticket  string   `yaml:"ticket,omitempty"`
	Title   string   `yaml:"title,omitempty"`
	Service string   `yaml:"service,omitempty"`
	Status  string   `yaml:"status,omitempty"`
	Owner   string   `yaml:"owner,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
}

func readPlanMetadata(repo models.RepositoryConfig, plan models.PlanDetail) (planYAML, error) {
	root, err := safePlanRoot(repo, plan)
	if err != nil {
		return planYAML{}, err
	}
	var meta planYAML
	data, err := os.ReadFile(filepath.Join(root, "plan.yaml"))
	if os.IsNotExist(err) {
		meta.Documents = plan.Documents
		return meta, nil
	}
	if err != nil {
		return meta, err
	}
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func applyMetadata(meta *planYAML, plan models.PlanDetail, input models.PlanMetadataUpdateInput) {
	if meta.Plan.Ticket == "" {
		meta.Plan.Ticket = plan.Ticket
	}
	if meta.Plan.Title == "" {
		meta.Plan.Title = plan.Title
	}
	if meta.Plan.Service == "" {
		meta.Plan.Service = plan.Service
	}
	if meta.Plan.Status == "" {
		meta.Plan.Status = string(plan.Status)
	}
	if input.Ticket != "" {
		meta.Plan.Ticket = strings.TrimSpace(input.Ticket)
	}
	if input.Title != "" {
		meta.Plan.Title = strings.TrimSpace(input.Title)
	}
	if input.Service != "" {
		meta.Plan.Service = strings.TrimSpace(input.Service)
	}
	if input.Status != "" {
		meta.Plan.Status = string(input.Status)
	}
	if input.Owner != "" {
		meta.Plan.Owner = strings.TrimSpace(input.Owner)
	}
	if input.Tags != nil {
		meta.Plan.Tags = cleanTags(input.Tags)
	}
	if len(meta.Documents) == 0 {
		meta.Documents = plan.Documents
	}
}

func writePlanMetadata(repo models.RepositoryConfig, plan models.PlanDetail, meta planYAML) error {
	root, err := safePlanRoot(repo, plan)
	if err != nil {
		return err
	}
	return writePlanMetadataAt(root, meta)
}

func writePlanMetadataAt(root string, meta planYAML) error {
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "plan.yaml"), data, 0o644)
}

func starterDocuments() []models.PlanDocument {
	return []models.PlanDocument{
		{ID: "README_md", Role: "overview", Path: "README.md", Label: "README"},
		{ID: "scenario__scenario-00-overview_md", Role: "scenario", Path: "scenario/scenario-00-overview.md", Label: "Scenario Overview"},
		{ID: "design__design-01-backend_md", Role: "design", Track: "backend", Path: "design/design-01-backend.md", Label: "Backend Design"},
		{ID: "design__design-02-frontend_md", Role: "design", Track: "frontend", Path: "design/design-02-frontend.md", Label: "Frontend Design"},
		{ID: "implementation-plan_md", Role: "implementation", Path: "implementation-plan.md", Label: "Implementation Plan"},
	}
}

func validatePlanDirectory(repo models.RepositoryConfig, dir string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(dir)))
	for _, allowed := range repo.PlanDirectories {
		if clean == allowed {
			return clean, nil
		}
	}
	return "", fmt.Errorf("plan directory is not registered")
}

func safePlanRoot(repo models.RepositoryConfig, plan models.PlanDetail) (string, error) {
	return safeJoin(repo.Path, plan.PlanRoot)
}

func safeJoin(root, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid path")
	}
	full := filepath.Join(root, clean)
	absRoot, _ := filepath.Abs(root)
	absFull, _ := filepath.Abs(full)
	if absFull != absRoot && !strings.HasPrefix(absFull, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root")
	}
	return absFull, nil
}

func isDocsRoot(plan models.PlanDetail) bool {
	return plan.MetadataSource == "docs"
}

func cleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out
}
