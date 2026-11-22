package knowledge

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/e2eresult"
	"kode-stream/internal/filesystem/pathguard"
)

func (s *KnowledgeService) E2ERunbooksForSources(workspaceID string, sourceRefs []string) (models.E2ERunbookList, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.E2ERunbookList{}, err
	}
	if err := requireKnowledgeEnabled(workspace); err != nil {
		return models.E2ERunbookList{}, err
	}
	wikis, err := s.store.List(workspaceID)
	if err != nil {
		return models.E2ERunbookList{}, err
	}
	matched := make([]models.E2ERunbook, 0)
	for _, wiki := range wikis {
		for _, page := range wiki.Pages {
			if !pageIsJourney(wiki, page) || !matchesE2ESource(page.SourceRefs, sourceRefs) {
				continue
			}
			matched = append(matched, s.e2ERunbook(workspace, wiki.Root, page))
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].Title < matched[j].Title })
	if len(matched) == 0 {
		return models.E2ERunbookList{Runbooks: matched, Diagnostic: "No canonical E2E journey is linked to this plan."}, nil
	}
	return models.E2ERunbookList{Runbooks: matched}, nil
}

func (s *KnowledgeService) E2ERunbook(workspaceID, root, slug string) (models.E2ERunbookList, error) {
	workspace, err := s.workspace(workspaceID)
	if err != nil {
		return models.E2ERunbookList{}, err
	}
	wiki, err := s.wiki(workspaceID, root)
	if err != nil {
		return models.E2ERunbookList{}, err
	}
	for _, page := range wiki.Pages {
		if page.Slug == slug && pageIsJourney(wiki, page) {
			return models.E2ERunbookList{Runbooks: []models.E2ERunbook{s.e2ERunbook(workspace, wiki.Root, page)}}, nil
		}
	}
	return models.E2ERunbookList{Runbooks: []models.E2ERunbook{}, Diagnostic: "The selected Knowledge page is not an E2E journey."}, nil
}

func matchesE2ESource(pageSources, requested []string) bool {
	for _, source := range pageSources {
		for _, candidate := range requested {
			if sourceReferencePath(source) == sourceReferencePath(candidate) {
				return true
			}
		}
	}
	return false
}

func (s *KnowledgeService) e2ERunbook(workspace models.WorkspaceConfig, wikiRoot string, page KnowledgePage) models.E2ERunbook {
	resultRelative := e2EResultPath(workspace.Path, wikiRoot, page)
	runbook := models.E2ERunbook{
		Title:      page.Title,
		Path:       filepath.ToSlash(filepath.Join(wikiRoot, page.Path)),
		Source:     "wiki",
		ResultPath: resultRelative,
	}
	result, err := pathguard.ValidateMarkdownFile(workspace.Path, resultRelative)
	if err == nil {
		data, readErr := e2eresult.ReadFile(result)
		if readErr != nil {
			runbook.Diagnostic = "Latest E2E result could not be read."
			return runbook
		}
		runbookPath, runbookErr := pathguard.ValidateMarkdownFile(workspace.Path, runbook.Path)
		if runbookErr != nil {
			runbook.Diagnostic = "E2E runbook path is unsafe or unsupported."
			return runbook
		}
		runbookData, runbookErr := e2eresult.ReadFileLimit(runbookPath, e2eresult.MaxRunbookBytes)
		if runbookErr != nil {
			runbook.Diagnostic = "E2E runbook could not be read."
			return runbook
		}
		if len(runbookData) > e2eresult.MaxRunbookBytes {
			runbook.Diagnostic = "E2E runbook exceeds the supported size."
			return runbook
		}
		runbook.LatestResult, runbook.Diagnostic = e2eresult.Parse(data, runbookData)
	} else if os.IsNotExist(err) {
		runbook.Diagnostic = "Not run"
	} else {
		runbook.Diagnostic = "Latest E2E result path is unsafe or unsupported."
	}
	return runbook
}

func e2EResultPath(workspaceRoot, wikiRoot string, page KnowledgePage) string {
	for index := len(page.SourceRefs) - 1; index >= 0; index-- {
		source := sourceReferencePath(page.SourceRefs[index])
		automationIndex := strings.Index(source, "/automation/")
		if automationIndex < 0 {
			continue
		}
		automationRoot := source[:automationIndex+len("/automation")]
		candidate := filepath.ToSlash(filepath.Join(automationRoot, "results", "latest.md"))
		if _, err := pathguard.ValidateMarkdownTarget(workspaceRoot, candidate); err == nil {
			return candidate
		}
	}
	fallback := filepath.ToSlash(filepath.Join(wikiRoot, filepath.Dir(page.Path), "automation", "results", "latest.md"))
	if _, err := pathguard.ValidateMarkdownTarget(workspaceRoot, fallback); err == nil {
		return fallback
	}
	return ""
}

func sourceReferencePath(value string) string {
	value = strings.TrimSpace(value)
	if path, _, found := strings.Cut(value, " | "); found {
		value = strings.TrimSpace(path)
	}
	return filepath.ToSlash(filepath.Clean(value))
}
