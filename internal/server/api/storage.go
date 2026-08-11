package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/storage"
	"kode-stream/internal/system"
)

type storageController struct {
	status storageStatusService
	sync   storageSyncService
}

func (a *storageController) storageStatusRoute(w http.ResponseWriter, r *http.Request) {
	if a.status == nil {
		writeError(w, http.StatusServiceUnavailable, "storage status is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, a.status.Status(r.Context()))
}

func (a *storageController) storageSyncRoute(w http.ResponseWriter, r *http.Request) {
	if a.sync == nil {
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
	result, err := a.sync.Sync(r.Context(), input)
	if err != nil {
		status := http.StatusInternalServerError
		var syncErr *storage.SyncError
		if errors.As(err, &syncErr) {
			switch syncErr.Kind {
			case storage.SyncErrorValidation:
				status = http.StatusBadRequest
			case storage.SyncErrorConflict:
				status = http.StatusConflict
			}
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *storageController) storageOptionRoute(w http.ResponseWriter, r *http.Request) {
	if a.status == nil {
		writeError(w, http.StatusServiceUnavailable, "storage status is unavailable")
		return
	}
	status := a.status.Status(r.Context())
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
