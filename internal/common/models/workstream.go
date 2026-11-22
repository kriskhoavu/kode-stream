package models

import "time"

type WorkstreamBranchLoadInput struct {
	Branch string `json:"branch,omitempty" yaml:"branch,omitempty"`
	Force  bool   `json:"force,omitempty" yaml:"force,omitempty"`
}

type ReviewedPlanImportInput struct {
	SourceBranch           string `json:"sourceBranch" yaml:"sourceBranch"`
	ExpectedCommit         string `json:"expectedCommit" yaml:"expectedCommit"`
	ExpectedCheckoutBranch string `json:"expectedCheckoutBranch" yaml:"expectedCheckoutBranch"`
	ItemID                 string `json:"itemId" yaml:"itemId"`
}

type WorkstreamBranchLoadResult struct {
	WorkspaceID           string        `json:"workspaceId" yaml:"workspaceId"`
	Branch                string        `json:"branch" yaml:"branch"`
	SelectedBranch        string        `json:"selectedBranch" yaml:"selectedBranch"`
	BranchRef             string        `json:"branchRef" yaml:"branchRef"`
	Commit                string        `json:"commit" yaml:"commit"`
	CurrentCheckoutBranch string        `json:"currentCheckoutBranch" yaml:"currentCheckoutBranch"`
	SourceMode            string        `json:"sourceMode" yaml:"sourceMode"`
	Mode                  string        `json:"mode" yaml:"mode"`
	Editable              bool          `json:"editable" yaml:"editable"`
	ScannedAt             time.Time     `json:"scannedAt" yaml:"scannedAt"`
	ItemCount             int           `json:"itemCount" yaml:"itemCount"`
	Warnings              []ScanWarning `json:"warnings" yaml:"warnings"`
	Items                 []ItemSummary `json:"items" yaml:"items"`
}

type FileSaveInput struct {
	FileID       string `json:"fileId" yaml:"fileId"`
	Content      string `json:"content" yaml:"content"`
	ExpectedHash string `json:"expectedHash,omitempty" yaml:"expectedHash,omitempty"`
}

type ItemMetadataUpdateInput struct {
	ExpectedRevision string     `json:"expectedRevision,omitempty" yaml:"expectedRevision,omitempty"`
	Title            string     `json:"title,omitempty" yaml:"title,omitempty"`
	Scope            string     `json:"scope,omitempty" yaml:"scope,omitempty"`
	Identifier       string     `json:"identifier,omitempty" yaml:"identifier,omitempty"`
	Status           ItemStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Owner            string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags             []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type VerificationTestSelection struct {
	ExpectedRevision string                `json:"expectedRevision,omitempty" yaml:"expectedRevision,omitempty"`
	Revision         string                `json:"revision,omitempty" yaml:"revision,omitempty"`
	SelectedSpecs    []string              `json:"selectedSpecs" yaml:"selectedSpecs"`
	Environment      string                `json:"environment,omitempty" yaml:"environment,omitempty"`
	DisplayMode      AutomationDisplayMode `json:"displayMode,omitempty" yaml:"displayMode,omitempty"`
	UpdatedAt        time.Time             `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

type AutomationTestPath struct {
	Path string `json:"path" yaml:"path"`
}

type DiscoveredVerificationSpec struct {
	Path       string `json:"path" yaml:"path"`
	Runner     string `json:"runner" yaml:"runner"`
	SourcePath string `json:"sourcePath,omitempty" yaml:"sourcePath,omitempty"`
}

type ItemVerificationTests struct {
	Selection       VerificationTestSelection    `json:"selection" yaml:"selection"`
	DiscoveredSpecs []DiscoveredVerificationSpec `json:"discoveredSpecs" yaml:"discoveredSpecs"`
}

type E2ELatestResult struct {
	Status      string    `json:"status" yaml:"status"`
	Freshness   string    `json:"freshness,omitempty" yaml:"freshness,omitempty"`
	RecordedAt  time.Time `json:"recordedAt,omitempty" yaml:"recordedAt,omitempty"`
	Provider    string    `json:"provider,omitempty" yaml:"provider,omitempty"`
	Environment string    `json:"environment,omitempty" yaml:"environment,omitempty"`
	FailedStep  string    `json:"failedStep,omitempty" yaml:"failedStep,omitempty"`
	Evidence    []string  `json:"evidence" yaml:"evidence"`
}

type E2ERunbook struct {
	Title        string           `json:"title" yaml:"title"`
	Path         string           `json:"path" yaml:"path"`
	Source       string           `json:"source" yaml:"source"`
	ResultPath   string           `json:"resultPath,omitempty" yaml:"resultPath,omitempty"`
	LatestResult *E2ELatestResult `json:"latestResult,omitempty" yaml:"latestResult,omitempty"`
	Diagnostic   string           `json:"diagnostic,omitempty" yaml:"diagnostic,omitempty"`
}

type E2ERunbookList struct {
	Runbooks   []E2ERunbook `json:"runbooks" yaml:"runbooks"`
	Diagnostic string       `json:"diagnostic,omitempty" yaml:"diagnostic,omitempty"`
}

type ItemStatusUpdateInput struct {
	ExpectedRevision string     `json:"expectedRevision,omitempty" yaml:"expectedRevision,omitempty"`
	Status           ItemStatus `json:"status" yaml:"status"`
}

type NewItemInput struct {
	WorkspaceID   string     `json:"workspaceId" yaml:"workspaceId"`
	Source        string     `json:"source" yaml:"source"`
	Scope         string     `json:"scope" yaml:"scope"`
	Identifier    string     `json:"identifier" yaml:"identifier"`
	Title         string     `json:"title" yaml:"title"`
	Status        ItemStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Owner         string     `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags          []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
	JiraKey       string     `json:"jiraKey,omitempty" yaml:"jiraKey,omitempty"`
	InitialReadme string     `json:"initialReadme,omitempty" yaml:"initialReadme,omitempty"`
}

type WriteResult struct {
	Item            ItemDetail `json:"item" yaml:"item"`
	ScannedAt       time.Time  `json:"scannedAt" yaml:"scannedAt"`
	Committed       bool       `json:"committed" yaml:"committed"`
	RefreshRequired bool       `json:"refreshRequired,omitempty" yaml:"refreshRequired,omitempty"`
	RefreshError    string     `json:"refreshError,omitempty" yaml:"refreshError,omitempty"`
}
