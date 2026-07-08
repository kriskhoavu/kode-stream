package scanner

// Package scanner discovers and parses Workspace sources.

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
)

type planYAML struct {
	Plan struct {
		Identifier string   `yaml:"identifier"`
		Ticket     string   `yaml:"ticket"`
		Title      string   `yaml:"title"`
		Scope      string   `yaml:"scope"`
		Service    string   `yaml:"service"`
		Status     string   `yaml:"status"`
		Owner      string   `yaml:"owner"`
		Tags       []string `yaml:"tags"`
	} `yaml:"plan"`
	Documents []models.ItemDocument `yaml:"documents"`
}

func readPlanYAML(reader SourceReader, root string) ([]byte, string, error) {
	data, err := reader.ReadFile(filepath.ToSlash(filepath.Join(root, "plan.yaml")))
	if err != nil {
		if isMissingPlanYAMLError(err) {
			return nil, "", os.ErrNotExist
		}
		return nil, "", err
	}
	return data, "plan.yaml", nil
}

func isMissingPlanYAMLError(err error) bool {
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "does not exist in") || strings.Contains(message, "exists on disk, but not in")
}

func NormalizeStatus(raw string) models.ItemStatus {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	switch s {
	case "unsorted", "unstructured":
		return models.StatusUnsorted
	case "in_progress", "progress", "doing", "active":
		return models.StatusInProgress
	case "review", "in_review":
		return models.StatusReview
	case "done", "complete", "completed", "closed":
		return models.StatusDone
	default:
		return models.StatusDraft
	}
}

func normalizeDocuments(docs []models.ItemDocument) []models.ItemDocument {
	for i := range docs {
		inferred := inferDocument(docs[i].Path)
		if docs[i].ID == "" {
			docs[i].ID = inferred.ID
		}
		if docs[i].Role == "" {
			docs[i].Role = inferred.Role
		}
		if docs[i].Track == "" {
			docs[i].Track = inferred.Track
		}
		if docs[i].Label == "" {
			docs[i].Label = inferred.Label
		}
	}
	sort.SliceStable(docs, func(i, j int) bool { return documentLess(docs[i], docs[j]) })
	return docs
}

func parsePlanYAML(data string) (planYAML, error) {
	var parsed planYAML
	if err := yaml.Unmarshal([]byte(data), &parsed); err != nil {
		return planYAML{}, err
	}
	if parsed.Plan.Identifier == "" {
		parsed.Plan.Identifier = parsed.Plan.Ticket
	}
	if parsed.Plan.Scope == "" {
		parsed.Plan.Scope = parsed.Plan.Service
	}
	return parsed, nil
}
