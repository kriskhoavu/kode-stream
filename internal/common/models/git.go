package models

import "time"

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

type GitActivityPath struct {
	Path    string          `json:"path" yaml:"path"`
	OldPath string          `json:"oldPath,omitempty" yaml:"oldPath,omitempty"`
	Status  GitChangeStatus `json:"status" yaml:"status"`
}

type GitActivityEntry struct {
	Commit      string            `json:"commit" yaml:"commit"`
	CommittedAt time.Time         `json:"committedAt" yaml:"committedAt"`
	Author      string            `json:"author" yaml:"author"`
	Message     string            `json:"message" yaml:"message"`
	Paths       []GitActivityPath `json:"paths" yaml:"paths"`
}

type GitStatus struct {
	WorkspaceID string `json:"workspaceId" yaml:"workspaceId"`
	Branch      string `json:"branch" yaml:"branch"`
	Upstream    string `json:"upstream,omitempty" yaml:"upstream,omitempty"`
	Ahead       int    `json:"ahead" yaml:"ahead"`
	Behind      int    `json:"behind" yaml:"behind"`
	// FetchedAt dates the remote-tracking ref that Ahead and Behind are measured
	// against. Zero means nothing has fetched since the repository was cloned, so
	// those counts describe a remote state of unknown age.
	FetchedAt  time.Time   `json:"fetchedAt,omitzero" yaml:"fetchedAt,omitempty"`
	Dirty      bool        `json:"dirty" yaml:"dirty"`
	Conflicted bool        `json:"conflicted" yaml:"conflicted"`
	Changes    []GitChange `json:"changes" yaml:"changes"`
}

type WorkspaceBranches struct {
	WorkspaceID string   `json:"workspaceId" yaml:"workspaceId"`
	Current     string   `json:"current" yaml:"current"`
	Branches    []string `json:"branches" yaml:"branches"`
}

type GitStashEntry struct {
	Ref     string `json:"ref" yaml:"ref"`
	Message string `json:"message" yaml:"message"`
}

type GitCommitInput struct {
	Message string   `json:"message" yaml:"message"`
	Paths   []string `json:"paths" yaml:"paths"`
}

type GitOperationInput struct {
	// Confirm is retained only for Knowledge's explicit pull confirmation;
	// Git HTTP operation routes decode a separate empty request shape.
	Confirm bool `json:"confirm,omitempty" yaml:"confirm,omitempty"`
}

type KnowledgeConfirmationInput struct {
	Confirm bool `json:"confirm" yaml:"confirm"`
}

type BranchCreateInput struct {
	Name       string `json:"name" yaml:"name"`
	StartPoint string `json:"startPoint,omitempty" yaml:"startPoint,omitempty"`
	Checkout   bool   `json:"checkout,omitempty" yaml:"checkout,omitempty"`
}

type BranchSwitchInput struct {
	Name         string `json:"name" yaml:"name"`
	Strategy     string `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	StashMessage string `json:"stashMessage,omitempty" yaml:"stashMessage,omitempty"`
}

type BranchSwitchDecision struct {
	SourceBranch    string `json:"sourceBranch" yaml:"sourceBranch"`
	TargetBranch    string `json:"targetBranch" yaml:"targetBranch"`
	CanCarryChanges bool   `json:"canCarryChanges" yaml:"canCarryChanges"`
}

type GitOperationResult struct {
	OK              bool                  `json:"ok" yaml:"ok"`
	Committed       bool                  `json:"committed,omitempty" yaml:"committed,omitempty"`
	RefreshRequired bool                  `json:"refreshRequired,omitempty" yaml:"refreshRequired,omitempty"`
	RefreshError    string                `json:"refreshError,omitempty" yaml:"refreshError,omitempty"`
	Partial         bool                  `json:"partial,omitempty" yaml:"partial,omitempty"`
	Message         string                `json:"message,omitempty" yaml:"message,omitempty"`
	RecoveryHint    string                `json:"recoveryHint,omitempty" yaml:"recoveryHint,omitempty"`
	Status          GitStatus             `json:"status" yaml:"status"`
	Code            string                `json:"code,omitempty" yaml:"code,omitempty"`
	Details         map[string]string     `json:"details,omitempty" yaml:"details,omitempty"`
	Decision        *BranchSwitchDecision `json:"decision,omitempty" yaml:"decision,omitempty"`
	StashRef        string                `json:"stashRef,omitempty" yaml:"stashRef,omitempty"`
	StashMessage    string                `json:"stashMessage,omitempty" yaml:"stashMessage,omitempty"`
}
