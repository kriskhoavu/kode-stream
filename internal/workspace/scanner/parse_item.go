package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"kode-stream/internal/common/models"
)

// itemFacts is what an item's plan.yaml and README.md say about it, before
// Git adds who changed it and when.
type itemFacts struct {
	scope, identifier, title, owner, description string
	status                                       models.ItemStatus
	tags                                         []string
	documents                                    []models.ItemDocument
	metadata                                     map[string]any
	metadataSource, metadataRevision             string
	warnings                                     []models.ScanWarning
}

func (s *Scanner) parseItem(request ScanRequest, reader SourceReader, branch, scope, identifier, relItemPath, itemRoot string) (models.ItemDetail, []models.ScanWarning, error) {
	workspace := request.Workspace
	facts := &itemFacts{scope: scope, identifier: identifier, title: titleFromIdentifier(identifier), status: models.StatusDraft, tags: []string{}, documents: []models.ItemDocument{}, metadata: map[string]any{}, metadataSource: "fallback"}
	facts.readPlan(reader, itemRoot, relItemPath)
	facts.readReadme(reader, itemRoot)
	if len(facts.documents) == 0 {
		facts.documents = fallbackDocuments(reader, itemRoot)
	}
	updated, author := s.gitFacts(request, reader, relItemPath, itemRoot)
	if author == "" && facts.owner != "" {
		author = facts.owner
	}
	summary := models.ItemSummary{
		ID:               stablePlanID(workspace.ID, branch, relItemPath),
		WorkspaceID:      workspace.ID,
		WorkspaceName:    workspace.Name,
		Branch:           branch,
		BranchRef:        request.BranchRef,
		Commit:           request.Commit,
		SourceMode:       request.SourceMode,
		Editable:         request.Editable,
		Scope:            facts.scope,
		Identifier:       facts.identifier,
		Title:            facts.title,
		Status:           facts.status,
		Owner:            facts.owner,
		Author:           author,
		Tags:             facts.tags,
		UpdatedAt:        updated,
		Description:      facts.description,
		MetadataSource:   facts.metadataSource,
		ItemPath:         relItemPath,
		MetadataRevision: facts.metadataRevision,
	}
	return models.ItemDetail{
		ItemSummary: summary,
		Documents:   facts.documents,
		Metadata:    facts.metadata,
		Warnings:    facts.warnings,
		Counts:      models.ItemWorkspaceCounts{Files: countMarkdownFiles(reader, itemRoot)},
	}, facts.warnings, nil
}

// readPlan takes the item's identity, status and documents from its
// plan.yaml; a plan.yaml that does not parse is a warning.
func (f *itemFacts) readPlan(reader SourceReader, itemRoot, relItemPath string) {
	data, source, err := readPlanYAML(reader, itemRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			f.warnings = append(f.warnings, models.ScanWarning{ItemPath: relItemPath, Message: err.Error()})
		}
		return
	}
	sum := sha256.Sum256(data)
	f.metadataRevision = hex.EncodeToString(sum[:])
	parsed, err := parsePlanYAML(string(data))
	if err != nil {
		f.warnings = append(f.warnings, models.ScanWarning{ItemPath: relItemPath, Message: err.Error()})
		return
	}
	f.metadataSource = source
	if parsed.Plan.Identifier != "" {
		f.identifier = parsed.Plan.Identifier
	}
	if parsed.Plan.Scope != "" {
		f.scope = parsed.Plan.Scope
	}
	if parsed.Plan.Title != "" {
		f.title = parsed.Plan.Title
	}
	f.owner = parsed.Plan.Owner
	f.status = NormalizeStatus(parsed.Plan.Status)
	if parsed.Plan.Tags != nil {
		f.tags = parsed.Plan.Tags
	}
	f.documents = resolveDocuments(reader, itemRoot, parsed.Documents)
	f.metadata["plan"] = parsed.Plan
}

// readReadme takes a title from the README's heading when the plan gave
// none, its first paragraph as the description, and, without a plan, the
// status the folder implies.
func (f *itemFacts) readReadme(reader SourceReader, itemRoot string) {
	data, err := reader.ReadFile(filepath.ToSlash(filepath.Join(itemRoot, "README.md")))
	if err != nil {
		return
	}
	if inferred := titleFromHeading(firstHeading(string(data)), f.identifier); inferred != "" && f.title == titleFromIdentifier(f.identifier) {
		f.title = inferred
	}
	if f.metadataSource == "fallback" {
		f.status = inferStatus(reader, itemRoot)
	}
	f.description = firstParagraph(string(data))
}

// gitFacts says when the item last changed and who changed it, at the
// scanned ref, falling back to the files' times.
func (s *Scanner) gitFacts(request ScanRequest, reader SourceReader, relItemPath, itemRoot string) (time.Time, string) {
	path, rel := request.Workspace.Path, filepath.ToSlash(relItemPath)
	ref := request.BranchRef
	if request.SourceMode == "snapshot" && request.Commit != "" {
		ref = request.Commit
	}
	var updated time.Time
	var author string
	if ref != "" {
		updated, author = s.git.LastUpdateAtRef(path, ref, rel), s.git.LastAuthorAtRef(path, ref, rel)
	} else {
		updated, author = s.git.LastUpdate(path, rel), s.git.LastAuthor(path, rel)
	}
	if updated.IsZero() {
		updated = latestModTime(reader, itemRoot)
	}
	return updated, author
}
