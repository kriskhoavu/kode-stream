package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/storage"
	"kode-stream/internal/system"
)

func (a *API) storageStatusRoute(w http.ResponseWriter, r *http.Request) {
	if a.storageStatus == nil {
		writeError(w, http.StatusServiceUnavailable, "storage status is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, a.storageStatus.Status(r.Context()))
}

func (a *API) storageSyncRoute(w http.ResponseWriter, r *http.Request) {
	if a.storageSync == nil {
		writeError(w, http.StatusServiceUnavailable, "storage sync is unavailable")
		return
	}
	var input storage.StorageSyncRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.storageSync.Sync(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) storageOptionRoute(w http.ResponseWriter, r *http.Request) {
	if a.storageStatus == nil {
		writeError(w, http.StatusServiceUnavailable, "storage status is unavailable")
		return
	}
	status := a.storageStatus.Status(r.Context())
	if status.EnvironmentLocked {
		writeError(w, http.StatusBadRequest, "storage option is controlled by environment variables")
		return
	}
	if status.Mode == models.RuntimeModeCloud {
		writeError(w, http.StatusBadRequest, "cloud mode requires database storage")
		return
	}
	var input struct {
		StorageOption string `json:"storageOption"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	option := strings.ToLower(strings.TrimSpace(input.StorageOption))
	if option != storage.StorageOptionDatabase && option != storage.StorageOptionDataDir {
		writeError(w, http.StatusBadRequest, "storageOption must be database or datadir")
		return
	}
	if err := system.SetStorageOption(option); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"storageOption": option, "restartRequired": true})
}
