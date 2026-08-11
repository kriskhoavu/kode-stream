package api

import (
	"encoding/json"
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
	status, err := a.gitOps.Status(r.PathValue("id"))
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
	entries, err := a.gitOps.Activity(r.PathValue("id"), r.URL.Query().Get("path"), limit)
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, entries, err)
}

func (a *gitController) gitBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := a.gitOps.Branches(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, branches, err)
}

func (a *gitController) gitStashes(w http.ResponseWriter, r *http.Request) {
	stashes, err := a.gitOps.Stashes(r.PathValue("id"))
	if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	respond(w, stashes, err)
}

func (a *gitController) gitApplyStash(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	result := withRecoveryHint(a.gitOps.ApplyStash(r.PathValue("id"), r.PathValue("ref")))
	a.recordGit(r.PathValue("id"), "git_apply_stash", nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitFetch(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, "git_fetch", a.gitOps.Fetch)
}

func (a *gitController) gitPull(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, "git_pull", a.gitOps.Pull)
}

func (a *gitController) gitPush(w http.ResponseWriter, r *http.Request) {
	a.gitOperation(w, r, "git_push", a.gitOps.Push)
}

func (a *gitController) gitCommit(w http.ResponseWriter, r *http.Request) {
	var input models.GitCommitInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.Commit(r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), "git_commit", input.Paths, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitCreateBranch(w http.ResponseWriter, r *http.Request) {
	var input models.BranchCreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.CreateBranch(r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), "git_create_branch", nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitSwitchBranch(w http.ResponseWriter, r *http.Request) {
	var input models.BranchSwitchInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	started := time.Now()
	result := withRecoveryHint(a.gitOps.SwitchBranch(r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), "git_switch_branch", nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) gitOperation(w http.ResponseWriter, r *http.Request, operation string, run func(string, models.GitOperationInput) models.GitOperationResult) {
	var input models.GitOperationInput
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	started := time.Now()
	result := withRecoveryHint(run(r.PathValue("id"), input))
	a.recordGit(r.PathValue("id"), operation, nil, started, result)
	respondGitResult(w, result)
}

func (a *gitController) recordGit(workspaceID, operation string, paths []string, started time.Time, result models.GitOperationResult) {
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
