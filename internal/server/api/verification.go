package api

import (
	"errors"
	"net/http"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	appruntime "kode-stream/internal/runtime"
	appverification "kode-stream/internal/verification"
	appworkspace "kode-stream/internal/workspace"
)

const maxVerificationMutationBodyBytes int64 = 256 << 10

type verificationController struct {
	workspaces   *appworkspace.Service
	verification *appverification.Service
}

func (a *verificationController) workspaceRuntime(w http.ResponseWriter, r *http.Request) {
	if a.workspaces == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace service unavailable")
		return
	}
	runtimeConfig, err := a.workspaces.Runtime(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if runtimeConfig == nil {
		writeJSON(w, http.StatusOK, map[string]any{"runtime": nil})
		return
	}
	writeJSON(w, http.StatusOK, runtimeConfig)
}

func (a *verificationController) saveWorkspaceRuntime(w http.ResponseWriter, r *http.Request) {
	if a.workspaces == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace service unavailable")
		return
	}
	var input models.WorkspaceRuntimeConfig
	if !decodeLimitedJSON(w, r, &input, maxVerificationMutationBodyBytes, true) {
		return
	}
	runtimeConfig, err := a.workspaces.SaveRuntime(r.PathValue("id"), &input)
	if err != nil {
		if errors.Is(err, apperrors.ErrWorkspaceNotFound) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runtimeConfig)
}

func (a *verificationController) createVerificationJob(w http.ResponseWriter, r *http.Request) {
	if a.verification == nil {
		writeError(w, http.StatusServiceUnavailable, "verification service unavailable")
		return
	}
	var input appverification.CreateInput
	if !decodeLimitedJSON(w, r, &input, maxVerificationMutationBodyBytes, true) {
		return
	}
	if input.Profile == "" {
		input.Profile = appruntime.VerifyProfileSmoke
	}
	job, err := a.verification.Start(r.PathValue("id"), input)
	if err != nil {
		if err.Error() == "workspace not found" {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err.Error() == "verification queue is full" {
			writeError(w, http.StatusTooManyRequests, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *verificationController) verificationJob(w http.ResponseWriter, r *http.Request) {
	if a.verification == nil {
		writeError(w, http.StatusServiceUnavailable, "verification service unavailable")
		return
	}
	job, ok := a.verification.GetContext(r.Context(), r.PathValue("id"), r.PathValue("jobId"))
	if !ok {
		writeError(w, http.StatusNotFound, "verification job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *verificationController) verificationArtifacts(w http.ResponseWriter, r *http.Request) {
	if a.verification == nil {
		writeError(w, http.StatusServiceUnavailable, "verification service unavailable")
		return
	}
	artifacts, err := a.verification.Artifacts(r.PathValue("id"), r.PathValue("jobId"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, artifacts)
}

func (a *verificationController) rerunVerificationJob(w http.ResponseWriter, r *http.Request) {
	if a.verification == nil {
		writeError(w, http.StatusServiceUnavailable, "verification service unavailable")
		return
	}
	var input struct {
		Profile appruntime.VerifyProfile `json:"profile"`
	}
	if !decodeLimitedJSON(w, r, &input, maxVerificationMutationBodyBytes, true) {
		return
	}
	job, err := a.verification.Rerun(r.PathValue("id"), r.PathValue("jobId"), input.Profile)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *verificationController) ingestVerificationCheckpoint(w http.ResponseWriter, r *http.Request) {
	if a.verification == nil {
		writeError(w, http.StatusServiceUnavailable, "verification service unavailable")
		return
	}
	var input appverification.CheckpointEvent
	if !decodeLimitedJSON(w, r, &input, maxVerificationMutationBodyBytes, true) {
		return
	}
	job, err := a.verification.IngestCheckpoint(r.PathValue("id"), input)
	if err != nil {
		if err.Error() == "workspace not found" {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err.Error() == "verification queue is full" {
			writeError(w, http.StatusTooManyRequests, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}
