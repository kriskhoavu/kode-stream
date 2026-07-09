package api

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "kode-stream/internal/common"
	"kode-stream/internal/common/models"
	appruntime "kode-stream/internal/runtime"
	appverification "kode-stream/internal/verification"
)

func (a *API) workspaceRuntime(w http.ResponseWriter, r *http.Request) {
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

func (a *API) saveWorkspaceRuntime(w http.ResponseWriter, r *http.Request) {
	if a.workspaces == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace service unavailable")
		return
	}
	var input models.WorkspaceRuntimeConfig
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
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

func (a *API) createVerificationJob(w http.ResponseWriter, r *http.Request) {
	if a.verification == nil {
		writeError(w, http.StatusServiceUnavailable, "verification service unavailable")
		return
	}
	var input appverification.CreateInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
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
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *API) verificationJob(w http.ResponseWriter, r *http.Request) {
	if a.verification == nil {
		writeError(w, http.StatusServiceUnavailable, "verification service unavailable")
		return
	}
	job, ok := a.verification.Get(r.PathValue("id"), r.PathValue("jobId"))
	if !ok {
		writeError(w, http.StatusNotFound, "verification job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *API) verificationArtifacts(w http.ResponseWriter, r *http.Request) {
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

func (a *API) rerunVerificationJob(w http.ResponseWriter, r *http.Request) {
	if a.verification == nil {
		writeError(w, http.StatusServiceUnavailable, "verification service unavailable")
		return
	}
	var input struct {
		Profile appruntime.VerifyProfile `json:"profile"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	job, err := a.verification.Rerun(r.PathValue("id"), r.PathValue("jobId"), input.Profile)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *API) ingestVerificationCheckpoint(w http.ResponseWriter, r *http.Request) {
	if a.verification == nil {
		writeError(w, http.StatusServiceUnavailable, "verification service unavailable")
		return
	}
	var input appverification.CheckpointEvent
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	job, err := a.verification.IngestCheckpoint(r.PathValue("id"), input)
	if err != nil {
		if err.Error() == "workspace not found" {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}
