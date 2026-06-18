package models

import "time"

type PlanStatus string

const (
	StatusUnsorted   PlanStatus = "unsorted"
	StatusIdeas      PlanStatus = "ideas"
	StatusDraft      PlanStatus = "draft"
	StatusInProgress PlanStatus = "in_progress"
	StatusReview     PlanStatus = "review"
	StatusDone       PlanStatus = "done"
)

var StatusOrder = []PlanStatus{StatusUnsorted, StatusIdeas, StatusDraft, StatusInProgress, StatusReview, StatusDone}

type RepositoryConfig struct {
	ID              string    `json:"id" yaml:"id"`
	Name            string    `json:"name" yaml:"name"`
	Path            string    `json:"path" yaml:"path"`
	BaselineBranch  string    `json:"baselineBranch" yaml:"baselineBranch"`
	PlanDirectories []string  `json:"planDirectories" yaml:"planDirectories"`
	CreatedAt       time.Time `json:"createdAt" yaml:"createdAt"`
	LastScannedAt   time.Time `json:"lastScannedAt,omitempty" yaml:"lastScannedAt,omitempty"`
}

type RepositoryInput struct {
	Name            string   `json:"name" yaml:"name"`
	Path            string   `json:"path" yaml:"path"`
	BaselineBranch  string   `json:"baselineBranch" yaml:"baselineBranch"`
	PlanDirectories []string `json:"planDirectories" yaml:"planDirectories"`
}

type RepositorySettings struct {
	Version int                      `json:"version" yaml:"version"`
	Cards   []RepositorySettingsCard `json:"cards" yaml:"cards"`
}

type RepositorySettingsCard struct {
	PathPattern string                   `json:"pathPattern" yaml:"pathPattern"`
	Fields      RepositorySettingsFields `json:"fields" yaml:"fields"`
}

type RepositorySettingsFields struct {
	Service string   `json:"service" yaml:"service"`
	Ticket  string   `json:"ticket" yaml:"ticket"`
	Title   string   `json:"title,omitempty" yaml:"title,omitempty"`
	Status  string   `json:"status,omitempty" yaml:"status,omitempty"`
	Owner   string   `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags    []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type SourceSettingsResult struct {
	Directory string             `json:"directory" yaml:"directory"`
	Exists    bool               `json:"exists" yaml:"exists"`
	Settings  RepositorySettings `json:"settings" yaml:"settings"`
	Warnings  []ScanWarning      `json:"warnings" yaml:"warnings"`
}

type PlanSummary struct {
	ID             string     `json:"id" yaml:"id"`
	RepositoryID   string     `json:"repositoryId" yaml:"repositoryId"`
	RepositoryName string     `json:"repositoryName" yaml:"repositoryName"`
	Branch         string     `json:"branch" yaml:"branch"`
	Service        string     `json:"service" yaml:"service"`
	Ticket         string     `json:"ticket" yaml:"ticket"`
	Title          string     `json:"title" yaml:"title"`
	Status         PlanStatus `json:"status" yaml:"status"`
	Owner          string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Author         string     `json:"author,omitempty" yaml:"author,omitempty"`
	Tags           []string   `json:"tags" yaml:"tags"`
	UpdatedAt      time.Time  `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	Description    string     `json:"description,omitempty" yaml:"description,omitempty"`
	MetadataSource string     `json:"metadataSource" yaml:"metadataSource"`
	PlanRoot       string     `json:"planRoot,omitempty" yaml:"planRoot,omitempty"`
}

type PlanDetail struct {
	PlanSummary
	Documents []PlanDocument      `json:"documents" yaml:"documents"`
	Metadata  map[string]any      `json:"metadata" yaml:"metadata"`
	Warnings  []ScanWarning       `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Counts    PlanWorkspaceCounts `json:"counts" yaml:"counts"`
}

type PlanWorkspaceCounts struct {
	Files int `json:"files" yaml:"files"`
}

type PlanDocument struct {
	ID    string `json:"id" yaml:"id"`
	Role  string `json:"role" yaml:"role"`
	Track string `json:"track,omitempty" yaml:"track,omitempty"`
	Path  string `json:"path" yaml:"path"`
	Label string `json:"label" yaml:"label"`
}

type FileNode struct {
	ID       string     `json:"id" yaml:"id"`
	Name     string     `json:"name" yaml:"name"`
	Path     string     `json:"path" yaml:"path"`
	Type     string     `json:"type" yaml:"type"`
	Children []FileNode `json:"children,omitempty" yaml:"children,omitempty"`
}

type FileContent struct {
	ID       string `json:"id" yaml:"id"`
	Path     string `json:"path" yaml:"path"`
	Content  string `json:"content" yaml:"content"`
	Language string `json:"language" yaml:"language"`
	Hash     string `json:"hash" yaml:"hash"`
}

type ScanWarning struct {
	PlanPath string `json:"planPath,omitempty" yaml:"planPath,omitempty"`
	Message  string `json:"message" yaml:"message"`
}

type ScanResult struct {
	RepositoryID string        `json:"repositoryId" yaml:"repositoryId"`
	ScannedAt    time.Time     `json:"scannedAt" yaml:"scannedAt"`
	PlanCount    int           `json:"planCount" yaml:"planCount"`
	Warnings     []ScanWarning `json:"warnings" yaml:"warnings"`
}

type FileSaveInput struct {
	FileID       string `json:"fileId" yaml:"fileId"`
	Content      string `json:"content" yaml:"content"`
	ExpectedHash string `json:"expectedHash,omitempty" yaml:"expectedHash,omitempty"`
}

type PlanMetadataUpdateInput struct {
	Title   string     `json:"title,omitempty" yaml:"title,omitempty"`
	Service string     `json:"service,omitempty" yaml:"service,omitempty"`
	Ticket  string     `json:"ticket,omitempty" yaml:"ticket,omitempty"`
	Status  PlanStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Owner   string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags    []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type PlanStatusUpdateInput struct {
	Status PlanStatus `json:"status" yaml:"status"`
}

type NewPlanInput struct {
	RepositoryID  string     `json:"repositoryId" yaml:"repositoryId"`
	PlanDirectory string     `json:"planDirectory" yaml:"planDirectory"`
	Service       string     `json:"service" yaml:"service"`
	Ticket        string     `json:"ticket" yaml:"ticket"`
	Title         string     `json:"title" yaml:"title"`
	Status        PlanStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Owner         string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags          []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type WriteResult struct {
	Plan      PlanDetail `json:"plan" yaml:"plan"`
	ScannedAt time.Time  `json:"scannedAt" yaml:"scannedAt"`
}

type GitChangeStatus string

const (
	GitChangeModified   GitChangeStatus = "modified"
	GitChangeAdded      GitChangeStatus = "added"
	GitChangeDeleted    GitChangeStatus = "deleted"
	GitChangeRenamed    GitChangeStatus = "renamed"
	GitChangeCopied     GitChangeStatus = "copied"
	GitChangeUntracked  GitChangeStatus = "untracked"
	GitChangeConflicted GitChangeStatus = "conflicted"
)

type GitChange struct {
	Path     string          `json:"path" yaml:"path"`
	OldPath  string          `json:"oldPath,omitempty" yaml:"oldPath,omitempty"`
	Status   GitChangeStatus `json:"status" yaml:"status"`
	Staged   bool            `json:"staged" yaml:"staged"`
	Conflict bool            `json:"conflict" yaml:"conflict"`
}

type GitStatus struct {
	RepositoryID string      `json:"repositoryId" yaml:"repositoryId"`
	Branch       string      `json:"branch" yaml:"branch"`
	Upstream     string      `json:"upstream,omitempty" yaml:"upstream,omitempty"`
	Ahead        int         `json:"ahead" yaml:"ahead"`
	Behind       int         `json:"behind" yaml:"behind"`
	Dirty        bool        `json:"dirty" yaml:"dirty"`
	Conflicted   bool        `json:"conflicted" yaml:"conflicted"`
	Changes      []GitChange `json:"changes" yaml:"changes"`
}

type GitCommitInput struct {
	Message string   `json:"message" yaml:"message"`
	Paths   []string `json:"paths" yaml:"paths"`
}

type GitOperationInput struct {
	Confirm bool `json:"confirm,omitempty" yaml:"confirm,omitempty"`
}

type BranchCreateInput struct {
	Name       string `json:"name" yaml:"name"`
	StartPoint string `json:"startPoint,omitempty" yaml:"startPoint,omitempty"`
	Checkout   bool   `json:"checkout,omitempty" yaml:"checkout,omitempty"`
}

type BranchSwitchInput struct {
	Name    string `json:"name" yaml:"name"`
	Confirm bool   `json:"confirm,omitempty" yaml:"confirm,omitempty"`
}

type GitOperationResult struct {
	OK      bool      `json:"ok" yaml:"ok"`
	Message string    `json:"message,omitempty" yaml:"message,omitempty"`
	Status  GitStatus `json:"status" yaml:"status"`
}
