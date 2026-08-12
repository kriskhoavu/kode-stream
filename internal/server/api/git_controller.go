package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"kode-stream/internal/audit"
	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	appitem "kode-stream/internal/item"
)

type gitController struct {
	gitOps *appgit.Service
	audit  auditRecorder
}

func (a *gitController) gitStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.gitOps.StatusContext(r.Context(), r.PathValue("id"))
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, status, err)
}

func (a *gitController) gitActivity(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	entries, err := a.gitOps.ActivityContext(r.Context(), r.PathValue("id"), r.URL.Query().Get("path"), limit)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, entries, err)
}

func (a *gitController) gitBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := a.gitOps.BranchesContext(r.Context(), r.PathValue("id"))
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, branches, err)
}

func (a *gitController) gitStashes(w http.ResponseWriter, r *http.Request) {
	stashes, err := a.gitOps.StashesContext(r.Context(), r.PathValue("id"))
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, stashes, err)
}

func (a *gitController) gitApplyStash(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	result := withRecoveryHint(a.gitOps.ApplyStashContext(r.Context(), r.PathValue("id"), r.PathValue("ref")))
	a.recordGit(r.PathValue("id"), "git_apply_stash", nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitFetch(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !decodeLimitedJSON(w, r, &body, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.FetchContext(r.Context(), r.PathValue("id")))
	a.recordGit(r.PathValue("id"), "git_fetch", nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitPull(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !decodeLimitedJSON(w, r, &body, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.PullContext(r.Context(), r.PathValue("id"), models.GitOperationInput{}))
	a.recordGit(r.PathValue("id"), "git_pull", nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitPush(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !decodeLimitedJSON(w, r, &body, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.PushContext(r.Context(), r.PathValue("id")))
	a.recordGit(r.PathValue("id"), "git_push", nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitCommit(w http.ResponseWriter, r *http.Request) {
	var input models.GitCommitInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.CommitContext(r.Context(), r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), "git_commit", input.Paths, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitCreateBranch(w http.ResponseWriter, r *http.Request) {
	var input models.BranchCreateInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.CreateBranchContext(r.Context(), r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), "git_create_branch", nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitSwitchBranch(w http.ResponseWriter, r *http.Request) {
	var input models.BranchSwitchInput
	if !decodeLimitedJSON(w, r, &input, maxWorkspaceMutationBodyBytes, true) {
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.SwitchBranchContext(r.Context(), r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), "git_switch_branch", nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) recordGit(workspaceID, operation string, paths []string, started time.Time, result models.GitOperationResult) {
	if result.Committed && result.RefreshRequired {
		if a.audit.repository != nil {
			_, _ = a.audit.repository.Append(models.AuditEvent{WorkspaceID: workspaceID, Operation: operation, Status: models.AuditStatusSuccess, Message: "Git operation completed; workspace refresh is required.", Paths: paths, DurationMS: time.Since(started).Milliseconds(), Error: result.RefreshError})
		}
		return
	}
	var err error
	if !result.OK {
		err = errors.New(result.Message)
	}
	a.audit.record(workspaceID, "", operation, "Git operation completed.", paths, started, err)
}

func (a auditRecorder) record(workspaceID, itemID, operation, message string, paths []string, started time.Time, opErr error) {
	if a.repository == nil {
		return
	}
	status := models.AuditStatusSuccess
	errorMessage := ""
	if opErr != nil {
		status = models.AuditStatusFailed
		errorMessage = opErr.Error()
		message = "Operation failed."
		if errors.Is(opErr, appitem.ErrSnapshotReadOnly) || recoveryHint(errorMessage) != "" {
			status = models.AuditStatusBlocked
			message = "Operation blocked."
		}
	}
	if _, err := a.repository.Append(models.AuditEvent{WorkspaceID: workspaceID, ItemID: itemID, Operation: operation, Status: status, Message: message, Paths: paths, DurationMS: time.Since(started).Milliseconds(), Error: errorMessage}); err == nil {
		if _, centralized := a.repository.(*audit.Recorder); centralized {
			return
		}
		if invalidator, ok := a.reader.(interface{ Invalidate() }); ok {
			invalidator.Invalidate()
		}
	}
}
