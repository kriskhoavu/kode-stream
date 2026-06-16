package models

import "time"

type PlanStatus string

const (
	StatusIdeas      PlanStatus = "ideas"
	StatusDraft      PlanStatus = "draft"
	StatusInProgress PlanStatus = "in_progress"
	StatusReview     PlanStatus = "review"
	StatusDone       PlanStatus = "done"
)

var StatusOrder = []PlanStatus{StatusIdeas, StatusDraft, StatusInProgress, StatusReview, StatusDone}

type RepositoryConfig struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Path            string    `json:"path"`
	BaselineBranch  string    `json:"baselineBranch"`
	PlanDirectories []string  `json:"planDirectories"`
	CreatedAt       time.Time `json:"createdAt"`
	LastScannedAt   time.Time `json:"lastScannedAt,omitempty"`
}

type RepositoryInput struct {
	Name            string   `json:"name"`
	Path            string   `json:"path"`
	BaselineBranch  string   `json:"baselineBranch"`
	PlanDirectories []string `json:"planDirectories"`
}

type PlanSummary struct {
	ID             string     `json:"id"`
	RepositoryID   string     `json:"repositoryId"`
	RepositoryName string     `json:"repositoryName"`
	Branch         string     `json:"branch"`
	Service        string     `json:"service"`
	Ticket         string     `json:"ticket"`
	Title          string     `json:"title"`
	Status         PlanStatus `json:"status"`
	Owner          string     `json:"owner,omitempty"`
	Author         string     `json:"author,omitempty"`
	Tags           []string   `json:"tags"`
	UpdatedAt      time.Time  `json:"updatedAt,omitempty"`
	Description    string     `json:"description,omitempty"`
	MetadataSource string     `json:"metadataSource"`
	PlanRoot       string     `json:"planRoot,omitempty"`
}

type PlanDetail struct {
	PlanSummary
	Documents []PlanDocument      `json:"documents"`
	Metadata  map[string]any      `json:"metadata"`
	Warnings  []ScanWarning       `json:"warnings,omitempty"`
	Counts    PlanWorkspaceCounts `json:"counts"`
}

type PlanWorkspaceCounts struct {
	Files int `json:"files"`
}

type PlanDocument struct {
	ID    string `json:"id"`
	Role  string `json:"role"`
	Track string `json:"track,omitempty"`
	Path  string `json:"path"`
	Label string `json:"label"`
}

type FileNode struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Type     string     `json:"type"`
	Children []FileNode `json:"children,omitempty"`
}

type FileContent struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
}

type ScanWarning struct {
	PlanPath string `json:"planPath,omitempty"`
	Message  string `json:"message"`
}

type ScanResult struct {
	RepositoryID string        `json:"repositoryId"`
	ScannedAt    time.Time     `json:"scannedAt"`
	PlanCount    int           `json:"planCount"`
	Warnings     []ScanWarning `json:"warnings"`
}
