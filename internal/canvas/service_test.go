package canvas

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/ai"
	"kode-stream/internal/common/models"
	gitadapter "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/workspace/registry"
)

type canvasSessionReader struct{ records []ai.SessionRecordView }

func (reader canvasSessionReader) SessionRecords(workspaceID, branch string) ([]ai.SessionRecordView, error) {
	result := []ai.SessionRecordView{}
	for _, record := range reader.records {
		if record.WorkspaceID == workspaceID && record.RequestedBranch == branch {
			result = append(result, record)
		}
	}
	return result, nil
}

func TestCanvasServiceSeedsProjectsAndKeepsPlacementIndependent(t *testing.T) {
	repository, reg, items, git, workspaceConfig, initialItems, session := canvasServiceFixture(t)
	service := NewService(repository, reg, items, git, canvasSessionReader{records: []ai.SessionRecordView{session}}, nil, models.RuntimeModeLocal, models.AppStateDatastoreDataDir, nil)
	projection, err := service.ResolveDefault("", workspaceConfig.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Nodes) != 4 || len(projection.Connections) != 1 || len(projection.Unplaced) != 0 {
		t.Fatalf("projection=%#v", projection)
	}
	for _, connection := range projection.Connections {
		if connection.SourceOfTruth != "derived" {
			t.Fatalf("connection=%#v", connection)
		}
	}
	workspaceNode := findProjectedNode(t, projection, workspaceNodeID(workspaceConfig.ID))
	planNode := findProjectedNode(t, projection, planNodeID(initialItems[0].ID))
	sessionNode := findProjectedNode(t, projection, sessionNodeID(session.ID))
	if !sessionNode.Collapsed {
		t.Fatal("new session placement was not collapsed")
	}
	planPosition := planNode.Position
	if _, err := service.PatchPlacements("", projection.Layout.ID, []PlacementPatch{{NodeID: "plan:bogus", EntityRef: EntityRef{Kind: EntityPlan, WorkspaceID: workspaceConfig.ID, ItemID: "bogus", ItemPath: "plans/platform/bogus", BranchKey: "main"}, Position: Position{X: 1, Y: 2}}}); err == nil {
		t.Fatal("accepted a non-resolving plan placement")
	}
	projection, err = service.PatchPlacements("", projection.Layout.ID, []PlacementPatch{{NodeID: workspaceNode.ID, EntityRef: workspaceNode.EntityRef, Position: Position{X: 99, Y: 77}, ExpectedRevision: workspaceNode.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	if findProjectedNode(t, projection, planNode.ID).Position != planPosition {
		t.Fatal("moving the workspace moved a plan placement")
	}
	planRevision := findProjectedNode(t, projection, planNode.ID).Revision
	projection, err = service.SaveViewport("", projection.Layout.ID, projection.Layout.Version, Viewport{X: 10, Y: 20, Zoom: 1.2})
	if err != nil {
		t.Fatal(err)
	}
	if findProjectedNode(t, projection, planNode.ID).Revision != planRevision {
		t.Fatal("viewport save changed placement revision")
	}
	projection, err = service.RemovePlacement("", projection.Layout.ID, sessionNode.ID, sessionNode.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range projection.Nodes {
		if node.ID == sessionNode.ID {
			t.Fatal("removed session remained projected")
		}
	}
	for _, ref := range projection.Unplaced {
		if ref.SessionID == session.ID {
			t.Fatal("removed session returned as unplaced")
		}
	}

	newItem := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "item-3", WorkspaceID: workspaceConfig.ID, WorkspaceName: workspaceConfig.Name, Branch: "main", Commit: initialItems[0].Commit, SourceMode: "working_tree", Editable: true, Scope: "platform", Identifier: "PM-003", Title: "New plan", Status: models.StatusDraft, ItemPath: "plans/platform/PM-003"}}
	all := append(append([]models.ItemDetail(nil), initialItems...), newItem)
	if err := items.ReplaceWorkspace(workspaceConfig.ID, all, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	projection, err = service.Project("", projection.Layout.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Unplaced) != 1 || projection.Unplaced[0].ItemID != newItem.ID {
		t.Fatalf("unplaced=%#v", projection.Unplaced)
	}

	if err := items.ReplaceWorkspace(workspaceConfig.ID, []models.ItemDetail{initialItems[0], newItem}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	projection, err = service.Project("", projection.Layout.ID)
	if err != nil {
		t.Fatal(err)
	}
	stale := findProjectedNode(t, projection, planNodeID(initialItems[1].ID))
	if stale.State != NodeStale || stale.Plan != nil {
		t.Fatalf("stale node=%#v", stale)
	}
}

func TestCanvasServiceOnlyProjectsPlanTicketRoots(t *testing.T) {
	repository, reg, items, git, workspaceConfig, initialItems, _ := canvasServiceFixture(t)
	wiki := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "wiki-offer", WorkspaceID: workspaceConfig.ID, Branch: "main", Identifier: "offer", Title: "Offer wiki", Status: models.StatusDraft, ItemPath: "wiki/offer"}}
	nested := models.ItemDetail{ItemSummary: models.ItemSummary{ID: "nested", WorkspaceID: workspaceConfig.ID, Branch: "main", Identifier: "README", Title: "Nested plan document", Status: models.StatusDraft, ItemPath: "plans/platform/PM-001/README.md"}}
	if err := items.ReplaceWorkspace(workspaceConfig.ID, append(initialItems, wiki, nested), nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	service := NewService(repository, reg, items, git, nil, nil, models.RuntimeModeLocal, models.AppStateDatastoreDataDir, nil)
	projection, err := service.ResolveDefault("", workspaceConfig.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Nodes) != 3 { // workspace plus two tickets
		t.Fatalf("nodes=%#v", projection.Nodes)
	}
	for _, node := range projection.Nodes {
		if node.Plan != nil && (node.Plan.Service != "platform" || node.Plan.Status != models.StatusInProgress) {
			t.Fatalf("plan=%#v", node.Plan)
		}
		if node.EntityRef.ItemID == wiki.ID || node.EntityRef.ItemID == nested.ID {
			t.Fatalf("non-ticket item projected: %#v", node)
		}
	}
}

func TestCanvasServiceReturnsForbiddenReferencesWithoutEntityDetails(t *testing.T) {
	repository, reg, items, git, workspaceConfig, _, session := canvasServiceFixture(t)
	allowed := NewService(repository, reg, items, git, canvasSessionReader{records: []ai.SessionRecordView{session}}, nil, models.RuntimeModeLocal, models.AppStateDatastoreDataDir, nil)
	projection, err := allowed.ResolveDefault("owner", workspaceConfig.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := NewService(repository, reg, items, git, canvasSessionReader{records: []ai.SessionRecordView{session}}, nil, models.RuntimeModeLocal, models.AppStateDatastoreDataDir, map[models.Capability]bool{models.CapabilityWrite: true})
	projection, err = forbidden.Project("owner", projection.Layout.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range projection.Nodes {
		if node.State != NodeForbidden || node.Workspace != nil || node.Plan != nil || node.Session != nil || node.EntityRef.Identifier != "" || node.EntityRef.ItemPath != "" {
			t.Fatalf("forbidden node leaked details: %#v", node)
		}
	}
}

func canvasServiceFixture(t *testing.T) (*FileRepository, *registry.Registry, *itemindex.Index, *gitadapter.GitAdapter, models.WorkspaceConfig, []models.ItemDetail, ai.SessionRecordView) {
	t.Helper()
	root := t.TempDir()
	canvasGit(t, root, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "README.md"), []byte("plans"), 0o644); err != nil {
		t.Fatal(err)
	}
	canvasGit(t, root, "add", ".")
	command := exec.Command("git", "-C", root, "commit", "-m", "seed")
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	commit := stringsTrim(canvasGitOutput(t, root, "rev-parse", "HEAD"))
	dataDir := t.TempDir()
	git := gitadapter.New()
	reg := registry.New(filepath.Join(dataDir, "workspaces.yaml"), git)
	workspaceConfig, err := reg.Create(models.WorkspaceInput{Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}})
	if err != nil {
		t.Fatal(err)
	}
	items := itemindex.New(filepath.Join(dataDir, "items.yaml"))
	details := []models.ItemDetail{
		{ItemSummary: models.ItemSummary{ID: "item-1", WorkspaceID: workspaceConfig.ID, WorkspaceName: workspaceConfig.Name, Branch: "main", Commit: commit, SourceMode: "working_tree", Editable: true, Scope: "platform", Identifier: "PM-001", Title: "First", Status: models.StatusInProgress, ItemPath: "plans/platform/PM-001"}},
		{ItemSummary: models.ItemSummary{ID: "item-2", WorkspaceID: workspaceConfig.ID, WorkspaceName: workspaceConfig.Name, Branch: "main", Commit: commit, SourceMode: "working_tree", Editable: true, Scope: "platform", Identifier: "PM-002", Title: "Second", Status: models.StatusInProgress, ItemPath: "plans/platform/PM-002"}},
	}
	if err := items.ReplaceWorkspace(workspaceConfig.ID, details, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	session := ai.SessionRecordView{SessionRecord: ai.SessionRecord{ID: "session-1", WorkspaceID: workspaceConfig.ID, PlanRef: &ai.SessionPlanRef{ItemID: details[0].ID, ItemPath: details[0].ItemPath, Identifier: details[0].Identifier, BranchKey: "main", ObservedCommit: commit}, Provider: "codex", Intent: "card_context", RequestedBranch: "main", ObservedCommit: commit, State: ai.StateExited, StartedAt: now, EndedAt: now, LastKnownAt: now}}
	return NewFileRepository(filepath.Join(dataDir, "canvas.yaml")), reg, items, git, workspaceConfig, details, session
}

func findProjectedNode(t *testing.T, projection Projection, id string) ProjectedNode {
	t.Helper()
	for _, node := range projection.Nodes {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("node %s not found", id)
	return ProjectedNode{}
}

func canvasGit(t *testing.T, root string, args ...string) {
	t.Helper()
	_ = canvasGitOutput(t, root, args...)
}

func canvasGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

func stringsTrim(value string) string {
	for len(value) > 0 && (value[len(value)-1] == '\n' || value[len(value)-1] == '\r' || value[len(value)-1] == ' ') {
		value = value[:len(value)-1]
	}
	return value
}
