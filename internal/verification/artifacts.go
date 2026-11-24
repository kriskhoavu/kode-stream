package verification

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	appruntime "kode-stream/internal/runtime"
)

func (s *Service) Artifacts(workspaceID, jobID string) ([]Artifact, error) {
	job, ok := s.Get(workspaceID, jobID)
	if !ok {
		return nil, errors.New("verification job not found")
	}
	artifacts := append([]Artifact(nil), job.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

func (s *Service) collectArtifacts(jobID, root string) {
	artifacts, warning := collectArtifacts(root)
	for index := range artifacts {
		artifacts[index].Root = "workspace"
	}
	s.mutate(jobID, func(job *Job) {
		job.Artifacts = append(job.Artifacts, artifacts...)
		if warning != "" {
			job.ArtifactWarning = warning
		}
	})
}

func collectArtifacts(root string) ([]Artifact, string) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, "Artifact collection root is unavailable."
	}
	canonicalRoot, err = filepath.Abs(canonicalRoot)
	if err != nil {
		return nil, "Artifact collection root is unavailable."
	}
	walkRoot := canonicalRoot
	if info, statErr := os.Stat(canonicalRoot); statErr != nil {
		return nil, "Artifact collection root is unavailable."
	} else if !info.IsDir() {
		walkRoot = filepath.Dir(canonicalRoot)
	}
	artifacts := []Artifact{}
	var total int64
	warning := ""
	_ = filepath.WalkDir(canonicalRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			warning = "Some artifacts could not be collected."
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(walkRoot, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			warning = "Unsafe artifact path was skipped."
			return nil
		}
		if len(strings.Split(filepath.ToSlash(rel), "/")) > maxArtifactDepth {
			warning = "Artifact collection reached its depth limit."
			return nil
		}
		if len(artifacts) >= maxArtifactFiles || total+info.Size() > maxArtifactBytes {
			warning = "Artifact collection reached its retention limit."
			return filepath.SkipDir
		}
		total += info.Size()
		artifact := Artifact{
			Kind:      classifyArtifact(rel),
			Path:      filepath.ToSlash(rel),
			SizeBytes: info.Size(),
			CreatedAt: time.Now().UTC(),
		}
		artifacts = append(artifacts, artifact)
		return nil
	})
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, warning
}

func (s *Service) collectAutomationArtifacts(jobID string) {
	job, ok := s.snapshotAny(jobID)
	if !ok {
		return
	}
	if job.Mode != JobModeAutomation || job.Runtime == nil || job.Runtime.Automation == nil {
		return
	}
	root := strings.TrimSpace(job.AutomationRepoPath)
	if root == "" {
		return
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		s.mutate(jobID, func(state *Job) { state.ArtifactWarning = "Automation artifact root is unavailable." })
		return
	}
	for _, relRoot := range job.Runtime.Automation.ArtifactPaths {
		if err := appruntime.ValidateArtifactRelativePath(relRoot); err != nil {
			s.mutate(jobID, func(state *Job) { state.ArtifactWarning = "Unsafe automation artifact path was skipped." })
			continue
		}
		artifactPath := filepath.Join(root, filepath.FromSlash(relRoot))
		canonicalPath, resolveErr := filepath.EvalSymlinks(artifactPath)
		if resolveErr == nil && !pathWithin(canonicalRoot, canonicalPath) {
			s.mutate(jobID, func(state *Job) { state.ArtifactWarning = "Unsafe automation artifact path was skipped." })
			continue
		}
		artifacts, warning := collectArtifacts(artifactPath)
		includeRoot := true
		if info, statErr := os.Stat(artifactPath); statErr == nil && !info.IsDir() {
			includeRoot = false
		}
		for index := range artifacts {
			artifacts[index].Root = "automation"
			if includeRoot {
				artifacts[index].Path = filepath.ToSlash(filepath.Join(relRoot, artifacts[index].Path))
			}
		}
		s.mutate(jobID, func(state *Job) {
			state.Artifacts = append(state.Artifacts, artifacts...)
			if warning != "" {
				state.ArtifactWarning = warning
			}
		})
	}
}

func pathWithin(root, path string) bool {
	root, rootErr := filepath.Abs(root)
	path, pathErr := filepath.Abs(path)
	if rootErr != nil || pathErr != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func classifyArtifact(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	name := filepath.Base(lower)
	switch {
	case name == "automation.log":
		return "automation_log"
	case name == "verify.log":
		return "verify_log"
	case name == "runtime.log":
		return "runtime_log"
	case strings.Contains(lower, "trace"):
		return "playwright_trace"
	case strings.Contains(lower, "video"):
		return "playwright_video"
	case strings.Contains(lower, "screenshot") || strings.HasSuffix(lower, ".png"):
		return "playwright_screenshot"
	case strings.Contains(lower, "report"):
		return "playwright_report"
	case strings.Contains(lower, "verify"):
		return "verify_log"
	default:
		return "runtime_log"
	}
}

func (s *Service) removeArtifactRoot(workspaceID, jobID string) error {
	workspace, found, err := s.registry.Get(workspaceID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(workspace.Path) == "" || strings.Contains(jobID, string(filepath.Separator)) {
		return errors.New("verification artifact cleanup target is unavailable")
	}
	root := filepath.Join(workspace.Path, ".artifacts", "verification")
	target := filepath.Join(root, jobID)
	if !pathWithin(root, target) {
		return errors.New("verification artifact cleanup target is unsafe")
	}
	return s.removeArtifact(target)
}

// Job state is process-local, so an owned verification generation found at startup has no owner.
func (s *Service) cleanupOrphanArtifacts() {
	workspaces, err := s.registry.List()
	if err != nil {
		s.recordCleanupWarning(fmt.Sprintf("could not inspect orphan verification artifacts: %v", err))
		return
	}
	for _, workspace := range workspaces {
		root := filepath.Join(workspace.Path, ".artifacts", "verification")
		entries, readErr := os.ReadDir(root)
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			s.recordCleanupWarning(fmt.Sprintf("could not inspect orphan verification artifacts: %v", readErr))
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasPrefix(entry.Name(), "verify-") {
				continue
			}
			target := filepath.Join(root, entry.Name())
			if !pathWithin(root, target) {
				continue
			}
			if removeErr := s.removeArtifact(target); removeErr != nil {
				s.recordCleanupWarning(fmt.Sprintf("could not remove orphan verification artifacts for %s: %v", entry.Name(), removeErr))
			}
		}
	}
}
