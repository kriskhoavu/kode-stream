package scanner

// Package scanner discovers and parses Workspace sources.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/sourceguard"
	gitadapter "kode-stream/internal/git"
)

type Scanner struct {
	git *gitadapter.GitAdapter
}

type ScanData struct {
	Items    []models.ItemDetail
	Warnings []models.ScanWarning
}

type ScanRequest struct {
	Context    context.Context
	Budget     ScanBudget
	Workspace  models.WorkspaceConfig
	Branch     string
	BranchRef  string
	Commit     string
	SourceMode string
	Editable   bool
	Reader     SourceReader
}

type ScanBudget struct {
	MaxFileBytes  int64
	MaxTotalBytes int64
	MaxEntries    int
	MaxDepth      int
	MaxItems      int
	MaxWarnings   int
}

var ErrScanLimit = errors.New("workspace scan limit exceeded")

func DefaultScanBudget() ScanBudget {
	return ScanBudget{MaxFileBytes: 2 << 20, MaxTotalBytes: 64 << 20, MaxEntries: 50000, MaxDepth: 64, MaxItems: 10000, MaxWarnings: 1000}
}

func New(git *gitadapter.GitAdapter) *Scanner {
	return &Scanner{git: git}
}

func (s *Scanner) Scan(workspace models.WorkspaceConfig) (ScanData, error) {
	return s.ScanContext(context.Background(), workspace, DefaultScanBudget())
}

func (s *Scanner) ScanContext(ctx context.Context, workspace models.WorkspaceConfig, budget ScanBudget) (ScanData, error) {
	if _, err := sourceguard.ResolveAll(workspace.Path, workspace.SourcePaths()); err != nil {
		return ScanData{}, fmt.Errorf("configured source boundary is unsafe: %w", err)
	}
	branch, err := s.git.CurrentBranch(workspace.Path)
	if err != nil {
		branch = workspace.BaselineBranch
	}
	return s.ScanWithRequest(ScanRequest{
		Context: ctx, Budget: budget,
		Workspace:  workspace,
		Branch:     branch,
		SourceMode: "working_tree",
		Editable:   true,
		Reader:     NewFilesystemSourceReader(workspace.Path),
	})
}

func (s *Scanner) ScanWithRequest(request ScanRequest) (ScanData, error) {
	return s.ScanWithRequestContext(request.Context, request)
}

func (s *Scanner) ScanWithRequestContext(ctx context.Context, request ScanRequest) (ScanData, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if request.Budget.MaxEntries <= 0 {
		request.Budget = DefaultScanBudget()
	}
	request.Context = ctx
	workspace := request.Workspace
	branch := request.Branch
	if strings.TrimSpace(branch) == "" {
		branch = workspace.BaselineBranch
	}
	reader := request.Reader
	if reader == nil {
		reader = NewFilesystemSourceReader(workspace.Path)
	}
	limited := newBudgetReader(ctx, reader, request.Budget)
	reader = limited
	sourceMode := strings.TrimSpace(request.SourceMode)
	if sourceMode == "" {
		sourceMode = "working_tree"
	}
	request.SourceMode = sourceMode
	request.Branch = branch
	var out ScanData
	for _, source := range workspace.SourcesShownIn(models.SourceSurfaceWorkstream) {
		if err := ctx.Err(); err != nil {
			return ScanData{}, err
		}
		items, warnings := s.scanItemDirectory(request, reader, branch, sourceMode, source)
		out.Items = append(out.Items, items...)
		out.Warnings = append(out.Warnings, warnings...)
		if len(out.Items) > request.Budget.MaxItems || len(out.Warnings) > request.Budget.MaxWarnings {
			return ScanData{}, ErrScanLimit
		}
		if err := limited.Err(); err != nil {
			return ScanData{}, err
		}
	}
	sort.Slice(out.Items, func(i, j int) bool {
		return out.Items[i].UpdatedAt.After(out.Items[j].UpdatedAt)
	})
	return out, nil
}

func (s *Scanner) scanItemDirectory(request ScanRequest, reader SourceReader, branch, sourceMode, source string) ([]models.ItemDetail, []models.ScanWarning) {
	var items []models.ItemDetail
	var warnings []models.ScanWarning
	entries, err := reader.ReadDir(source)
	if err != nil {
		return items, []models.ScanWarning{{ItemPath: source, Message: err.Error()}}
	}
	settings, hasSettings, settingsWarnings := ReadSourceStructureSettingsFromReader(reader, source)
	if hasSettings {
		warnings = append(warnings, settingsWarnings...)
		if len(settingsWarnings) == 0 {
			configuredItems, configuredWarnings := s.scanConfiguredItemDirectory(request, reader, branch, source, settings)
			warnings = append(warnings, configuredWarnings...)
			if len(configuredItems) > 0 {
				return configuredItems, warnings
			}
			warnings = append(warnings, models.ScanWarning{ItemPath: source, Message: "workspace settings did not match any card directories; using fallback scan"})
		}
	}
	if shouldScanAsDocumentCollection(reader, source, entries) {
		detail, itemWarnings, err := s.parseItem(request, reader, branch, filepath.Base(source), filepath.Base(source), filepath.ToSlash(source), source)
		if err != nil {
			return items, []models.ScanWarning{{ItemPath: source, Message: err.Error()}}
		}
		detail.MetadataSource = documentationMetadataSource(source)
		detail.Status = models.StatusUnsorted
		if detail.Title == titleFromIdentifier(filepath.Base(source)) {
			detail.Title = titleFromDocumentRoot(source)
		}
		detail.Tags = append(detail.Tags, detail.MetadataSource)
		return []models.ItemDetail{detail}, append(warnings, itemWarnings...)
	}
	for _, scopeEntry := range entries {
		if !scopeEntry.IsDir() || strings.HasPrefix(scopeEntry.Name(), ".") {
			continue
		}
		scopeRoot := filepath.ToSlash(filepath.Join(source, scopeEntry.Name()))
		tickets, err := reader.ReadDir(scopeRoot)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{ItemPath: filepath.ToSlash(filepath.Join(source, scopeEntry.Name())), Message: err.Error()})
			continue
		}
		// The writer allows creating an item directly under the source root
		// (scope == source, see writer.isSourceRootScope). Such a directory is an
		// item, not a scope, so recognise it before falling through to the
		// scope/identifier walk.
		if isSourceRootItem(reader, scopeRoot, scopeEntry.Name(), tickets) {
			detail, itemWarnings, err := s.parseItem(request, reader, branch, filepath.Base(source), scopeEntry.Name(), scopeRoot, scopeRoot)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{ItemPath: scopeRoot, Message: err.Error()})
				continue
			}
			warnings = append(warnings, itemWarnings...)
			items = append(items, detail)
			continue
		}
		for _, identifierEntry := range tickets {
			if !identifierEntry.IsDir() || strings.HasPrefix(identifierEntry.Name(), ".") {
				continue
			}
			itemRoot := filepath.ToSlash(filepath.Join(scopeRoot, identifierEntry.Name()))
			relItemPath := filepath.ToSlash(filepath.Join(source, scopeEntry.Name(), identifierEntry.Name()))
			detail, itemWarnings, err := s.parseItem(request, reader, branch, scopeEntry.Name(), identifierEntry.Name(), relItemPath, itemRoot)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{ItemPath: relItemPath, Message: err.Error()})
				continue
			}
			warnings = append(warnings, itemWarnings...)
			items = append(items, detail)
		}
	}
	return items, warnings
}

func documentationMetadataSource(source string) string {
	if strings.EqualFold(filepath.Base(filepath.Clean(source)), "wiki") {
		return "wiki"
	}
	return "docs"
}

func (s *Scanner) scanConfiguredItemDirectory(request ScanRequest, reader SourceReader, branch, source string, settings models.SourceStructureSettings) ([]models.ItemDetail, []models.ScanWarning) {
	var items []models.ItemDetail
	var warnings []models.ScanWarning
	seen := map[string]bool{}
	for _, card := range settings.Cards {
		segments, err := parsePathPattern(card.PathPattern)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{ItemPath: source, Message: err.Error()})
			continue
		}
		for _, match := range matchPatternDirectories(reader, source, segments) {
			if seen[match.path] {
				continue
			}
			seen[match.path] = true
			scope := renderSettingsTemplate(card.Fields.Source, match.captures)
			identifier := renderSettingsTemplate(card.Fields.Item, match.captures)
			if strings.TrimSpace(scope) == "" || strings.TrimSpace(identifier) == "" {
				warnings = append(warnings, models.ScanWarning{ItemPath: filepath.ToSlash(match.path), Message: "workspace settings produced an empty scope or identifier"})
				continue
			}
			relFromRoot, err := filepath.Rel(filepath.FromSlash(source), filepath.FromSlash(match.path))
			if err != nil {
				warnings = append(warnings, models.ScanWarning{ItemPath: filepath.ToSlash(match.path), Message: err.Error()})
				continue
			}
			relItemPath := filepath.ToSlash(filepath.Join(source, relFromRoot))
			detail, itemWarnings, err := s.parseItem(request, reader, branch, scope, identifier, relItemPath, match.path)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{ItemPath: relItemPath, Message: err.Error()})
				continue
			}
			warnings = append(warnings, itemWarnings...)
			if detail.MetadataSource != "plan.yaml" {
				applySourceStructureSettings(&detail, card, match.captures)
			}
			items = append(items, detail)
		}
	}
	return items, warnings
}

func shouldScanAsDocumentCollection(reader SourceReader, root string, entries []DirEntry) bool {
	if hasMarkdownFiles(reader, root) && !hasStructuredItemChildren(reader, root, entries) {
		return true
	}
	return false
}

func hasStructuredItemChildren(reader SourceReader, root string, entries []DirEntry) bool {
	for _, scopeEntry := range entries {
		if !scopeEntry.IsDir() || strings.HasPrefix(scopeEntry.Name(), ".") {
			continue
		}
		tickets, err := reader.ReadDir(filepath.ToSlash(filepath.Join(root, scopeEntry.Name())))
		if err != nil {
			continue
		}
		for _, identifierEntry := range tickets {
			if identifierEntry.IsDir() && !strings.HasPrefix(identifierEntry.Name(), ".") && isItemFolder(reader, filepath.ToSlash(filepath.Join(root, scopeEntry.Name(), identifierEntry.Name())), identifierEntry.Name()) {
				return true
			}
		}
	}
	return false
}

// isSourceRootItem reports whether a directory sitting directly under a source
// root is an item rather than a scope. It is an item when it looks like one and
// holds no item directories of its own, so real scopes such as plans/api keep
// being walked as scopes.
func isSourceRootItem(reader SourceReader, path, name string, entries []DirEntry) bool {
	if !isItemFolder(reader, path, name) {
		return false
	}
	return !hasStructuredItemChildren(reader, path, entries)
}

func isItemFolder(reader SourceReader, path, name string) bool {
	if _, err := reader.Stat(filepath.ToSlash(filepath.Join(path, "plan.yaml"))); err == nil {
		return true
	}
	return regexp.MustCompile(`^[A-Z]+-\d+$`).MatchString(strings.ToUpper(name))
}

func stablePlanID(repoID, branch, relItemPath string) string {
	// The former 32-bit FNV suffix merged valid plans in the same branch.
	// A versioned SHA-256 identity is deterministic, URL-safe, and keeps the
	// workspace prefix useful when inspecting logs without relying on it for
	// uniqueness.  Old persisted indexes are rebuilt on their next scan.
	sum := sha256.Sum256([]byte(repoID + "\x00" + branch + "\x00" + filepath.ToSlash(relItemPath)))
	return fmt.Sprintf("v2-%s-%s", repoID, hex.EncodeToString(sum[:]))
}

func countMarkdownFiles(reader SourceReader, root string) int {
	count := 0
	_ = reader.WalkDir(root, func(path string, d DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			count++
		}
		return nil
	})
	return count
}

func hasMarkdownFiles(reader SourceReader, root string) bool {
	found := false
	_ = reader.WalkDir(root, func(path string, d DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func latestModTime(reader SourceReader, root string) time.Time {
	var latest time.Time
	_ = reader.WalkDir(root, func(path string, d DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest.UTC()
}
