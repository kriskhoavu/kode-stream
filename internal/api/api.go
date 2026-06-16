package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/models"
	"plan-manager/internal/planindex"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
)

type API struct {
	registry *registry.Registry
	index    *planindex.Index
	scanner  *scanner.Scanner
	files    *fileaccess.Access
	git      *gitadapter.GitAdapter
}

func New(reg *registry.Registry, idx *planindex.Index, scan *scanner.Scanner, files *fileaccess.Access, git *gitadapter.GitAdapter) *API {
	return &API{registry: reg, index: idx, scanner: scan, files: files, git: git}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/repositories", a.listRepositories)
	mux.HandleFunc("POST /api/repositories", a.createRepository)
	mux.HandleFunc("POST /api/repositories/{id}/scan", a.scanRepository)
	mux.HandleFunc("GET /api/plans", a.listPlans)
	mux.HandleFunc("GET /api/plans/{id}", a.planDetail)
	mux.HandleFunc("GET /api/plans/{id}/files", a.planFiles)
	mux.HandleFunc("GET /api/plans/{id}/files/{fileID}", a.planFileContent)
	mux.HandleFunc("GET /api/plans/{id}/diff", a.planDiff)
	return mux
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) listRepositories(w http.ResponseWriter, r *http.Request) {
	repos, err := a.registry.List()
	respond(w, repos, err)
}

func (a *API) createRepository(w http.ResponseWriter, r *http.Request) {
	var input models.RepositoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	repo, err := a.registry.Create(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, repo)
}

func (a *API) scanRepository(w http.ResponseWriter, r *http.Request) {
	repo, ok, err := a.registry.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	data, err := a.scanner.Scan(repo)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	scannedAt := time.Now().UTC()
	if err := a.index.ReplaceRepository(repo.ID, data.Plans, data.Warnings, scannedAt); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = a.registry.TouchScanned(repo.ID, scannedAt)
	writeJSON(w, http.StatusOK, models.ScanResult{
		RepositoryID: repo.ID,
		ScannedAt:    scannedAt,
		PlanCount:    len(data.Plans),
		Warnings:     data.Warnings,
	})
}

func (a *API) listPlans(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	plans, err := a.index.Query(planindex.Query{
		RepositoryID: q.Get("repositoryId"),
		Branch:       q.Get("branch"),
		Status:       q.Get("status"),
		Text:         q.Get("q"),
	})
	respond(w, plans, err)
}

func (a *API) planDetail(w http.ResponseWriter, r *http.Request) {
	repo, plan, ok, err := a.repoAndPlan(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	plan.Description = fullReadmeDescription(repo, plan)
	writeJSON(w, http.StatusOK, plan)
}

func (a *API) planFiles(w http.ResponseWriter, r *http.Request) {
	repo, plan, ok, err := a.repoAndPlan(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	tree, err := a.files.Tree(repo, plan)
	respond(w, tree, err)
}

func (a *API) planFileContent(w http.ResponseWriter, r *http.Request) {
	repo, plan, ok, err := a.repoAndPlan(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	content, err := a.files.Read(repo, plan, r.PathValue("fileID"))
	respond(w, content, err)
}

func (a *API) planDiff(w http.ResponseWriter, r *http.Request) {
	repo, plan, ok, err := a.repoAndPlan(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	diff, err := a.git.Diff(repo.Path, plan.PlanRoot)
	if err != nil {
		writeError(w, http.StatusBadRequest, "diff unavailable: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (a *API) repoAndPlan(planID string) (models.RepositoryConfig, models.PlanDetail, bool, error) {
	plan, ok, err := a.index.Get(planID)
	if err != nil || !ok {
		return models.RepositoryConfig{}, models.PlanDetail{}, ok, err
	}
	repo, ok, err := a.registry.Get(plan.RepositoryID)
	if err != nil || !ok {
		return repo, plan, ok, err
	}
	if plan.PlanRoot == "" {
		plan.PlanRoot = fallbackPlanRoot(repo, plan)
	}
	return repo, plan, ok, err
}

func fallbackPlanRoot(repo models.RepositoryConfig, plan models.PlanDetail) string {
	if len(repo.PlanDirectories) == 0 || plan.Service == "" || plan.Ticket == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(repo.PlanDirectories[0], plan.Service, plan.Ticket))
}

func fullReadmeDescription(repo models.RepositoryConfig, plan models.PlanDetail) string {
	if plan.PlanRoot == "" {
		return plan.Description
	}
	readme := filepath.Join(repo.Path, filepath.FromSlash(plan.PlanRoot), "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		return plan.Description
	}
	if description := firstMarkdownParagraph(string(data)); description != "" {
		return description
	}
	return plan.Description
}

func firstMarkdownParagraph(markdown string) string {
	for _, block := range strings.Split(markdown, "\n\n") {
		clean := strings.TrimSpace(block)
		if clean == "" || strings.HasPrefix(clean, "#") || strings.HasPrefix(clean, "|") {
			continue
		}
		return regexp.MustCompile(`\s+`).ReplaceAllString(clean, " ")
	}
	return ""
}

func respond(w http.ResponseWriter, data any, err error) {
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
	})
}
