package system

import (
	"encoding/json"
	"net/http"

	"kode-stream/internal/common/httpx"
)

type SystemController struct{ repository *Dialog }

func NewController(repository *Dialog) *SystemController {
	return &SystemController{repository: repository}
}

func (c *SystemController) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/system/select-directory", c.selectDirectory)
	mux.HandleFunc("POST /api/system/select-file", c.selectFile)
	mux.HandleFunc("POST /api/system/open-path", c.openPath)
	mux.HandleFunc("GET /api/system/config-paths", c.configPaths)
	mux.HandleFunc("PUT /api/system/config-paths", c.updateConfigPaths)
}

func (c *SystemController) selectFile(w http.ResponseWriter, _ *http.Request) {
	path, err := c.repository.SelectYAMLFile()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (c *SystemController) selectDirectory(w http.ResponseWriter, _ *http.Request) {
	path, err := c.repository.SelectDirectory()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (c *SystemController) openPath(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body", nil)
		return
	}
	if err := c.repository.OpenPath(input.Path); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (c *SystemController) configPaths(w http.ResponseWriter, _ *http.Request) {
	paths, err := ResolvePaths()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"dataDir": paths.Dir, "defaultDataDir": paths.DefaultDir, "cloneRootDir": paths.CloneRootDir, "registryFile": paths.RegistryFile})
}

func (c *SystemController) updateConfigPaths(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DataDir string `json:"dataDir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body", nil)
		return
	}
	paths, err := SetDataDir(input.DataDir)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"dataDir": paths.Dir, "defaultDataDir": paths.DefaultDir, "cloneRootDir": paths.CloneRootDir, "registryFile": paths.RegistryFile, "restartRequired": true})
}
