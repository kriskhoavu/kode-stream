package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"kode-stream/internal/ai"
	"kode-stream/internal/audit"
	"kode-stream/internal/common/models"
	appgit "kode-stream/internal/git"
	itemindex "kode-stream/internal/item/index"
	"kode-stream/internal/navigation"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace/registry"
)

const (
	SyncDataDirToDatabase = "datadir_to_database"
	SyncDatabaseToDataDir = "database_to_datadir"
)

type StorageSyncRequest struct {
	Direction string `json:"direction"`
	Confirm   bool   `json:"confirm"`
}

type StorageSyncResult struct {
	OK            bool           `json:"ok"`
	Direction     string         `json:"direction"`
	BackupPath    string         `json:"backupPath"`
	Summary       map[string]int `json:"summary"`
	Warnings      []string       `json:"warnings"`
	SkippedStores []string       `json:"skippedStores"`
}

type StorageSyncService struct {
	config  Config
	paths   system.Paths
	runtime system.RuntimeConfig
	git     *appgit.GitAdapter
}

type storageSnapshot struct {
	Workspaces []models.WorkspaceConfig
	Items      []models.ItemDetail
	Scans      []models.BranchScanMetadata
	Audit      []models.AuditEvent
	Filters    []models.SavedFilter
	Recents    []models.RecentItem
	AISettings ai.Settings
}

type fileIndexState struct {
	Items       []models.ItemDetail                             `yaml:"items"`
	Warnings    []models.ScanWarning                            `yaml:"warnings"`
	Scans       map[string]time.Time                            `yaml:"scans"`
	BranchScans map[string]map[string]models.BranchScanMetadata `yaml:"branchScans"`
}

func NewStorageSyncService(config Config, paths system.Paths, runtime system.RuntimeConfig, git *appgit.GitAdapter) *StorageSyncService {
	return &StorageSyncService{config: config, paths: paths, runtime: runtime, git: git}
}

func (s *StorageSyncService) Sync(ctx context.Context, request StorageSyncRequest) (StorageSyncResult, error) {
	direction := strings.ToLower(strings.TrimSpace(request.Direction))
	if direction != SyncDataDirToDatabase && direction != SyncDatabaseToDataDir {
		return StorageSyncResult{}, fmt.Errorf("direction must be %s or %s", SyncDataDirToDatabase, SyncDatabaseToDataDir)
	}
	if !request.Confirm {
		return StorageSyncResult{}, errors.New("storage sync requires confirmation")
	}
	if s.runtime.Mode == models.RuntimeModeCloud {
		return StorageSyncResult{}, errors.New("storage sync is only available in local mode")
	}
	if err := ctx.Err(); err != nil {
		return StorageSyncResult{}, err
	}
	result := StorageSyncResult{OK: true, Direction: direction, Summary: map[string]int{}, Warnings: []string{}, SkippedStores: []string{"knowledge"}}
	var snapshot storageSnapshot
	var err error
	switch direction {
	case SyncDataDirToDatabase:
		snapshot, err = readDataDirSnapshot(s.paths)
		if err != nil {
			return StorageSyncResult{}, err
		}
		result.BackupPath, err = backupDatabaseTarget(s.paths, direction, time.Now().UTC())
		if err != nil {
			return StorageSyncResult{}, err
		}
		err = s.writeDatabaseSnapshot(snapshot)
	case SyncDatabaseToDataDir:
		snapshot, err = s.readDatabaseSnapshot()
		if err != nil {
			return StorageSyncResult{}, err
		}
		result.BackupPath, err = backupDataDirTarget(s.paths, direction, time.Now().UTC())
		if err != nil {
			return StorageSyncResult{}, err
		}
		err = writeDataDirSnapshot(s.paths, snapshot)
	}
	if err != nil {
		return StorageSyncResult{}, err
	}
	result.Summary = snapshot.summary()
	return result, nil
}

func (s storageSnapshot) summary() map[string]int {
	return map[string]int{
		"workspaces":   len(s.Workspaces),
		"items":        len(s.Items),
		"branchScans":  len(s.Scans),
		"auditEvents":  len(s.Audit),
		"savedFilters": len(s.Filters),
		"recentItems":  len(s.Recents),
	}
}

func (s *StorageSyncService) writeDatabaseSnapshot(snapshot storageSnapshot) error {
	config := s.config
	config.StorageOption = StorageOptionDatabase
	config.Driver = StorageDriverSQLite
	config.DatabaseURL = ""
	if config.SQLitePath == "" {
		config.SQLitePath = s.paths.SQLiteDatabaseFile
	}
	store, err := openSQLStore(config)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := clearSQLAppState(store); err != nil {
		return err
	}
	workspaces := newSQLiteWorkspaceRepository(store, s.paths, s.git)
	for _, workspace := range snapshot.Workspaces {
		if err := workspaces.upsert(workspace); err != nil {
			return err
		}
	}
	items := &SQLiteItemRepository{db: store.db, driver: store.driver}
	grouped := map[string]map[string][]models.ItemDetail{}
	for _, item := range snapshot.Items {
		if grouped[item.WorkspaceID] == nil {
			grouped[item.WorkspaceID] = map[string][]models.ItemDetail{}
		}
		grouped[item.WorkspaceID][item.Branch] = append(grouped[item.WorkspaceID][item.Branch], item)
	}
	for _, scan := range snapshot.Scans {
		if err := items.ReplaceWorkspaceBranch(scan.WorkspaceID, scan.Branch, grouped[scan.WorkspaceID][scan.Branch], scan); err != nil {
			return err
		}
		delete(grouped[scan.WorkspaceID], scan.Branch)
	}
	for workspaceID, branches := range grouped {
		for branch, branchItems := range branches {
			if err := items.ReplaceWorkspaceBranch(workspaceID, branch, branchItems, models.BranchScanMetadata{WorkspaceID: workspaceID, Branch: branch, SourceMode: "working_tree", Editable: true, ScannedAt: time.Now().UTC()}); err != nil {
				return err
			}
		}
	}
	auditStore := &SQLiteAuditRepository{db: store.db, driver: store.driver, now: time.Now}
	for i := len(snapshot.Audit) - 1; i >= 0; i-- {
		if _, err := auditStore.Append(snapshot.Audit[i]); err != nil {
			return err
		}
	}
	navigationStore := &SQLiteNavigationRepository{db: store.db, driver: store.driver, now: time.Now}
	for _, filter := range snapshot.Filters {
		if _, err := navigationStore.SaveFilter(filter); err != nil {
			return err
		}
	}
	for _, recent := range snapshot.Recents {
		if err := navigationStore.RecordRecent(recent); err != nil {
			return err
		}
	}
	_, err = (&SQLiteAISettingsRepository{db: store.db, driver: store.driver}).Save(snapshot.AISettings)
	if err != nil {
		return err
	}
	status := &SQLiteImportStatusRepository{db: store.db, driver: store.driver}
	for _, source := range []string{"workspaces.yaml", "item-index.yaml", "audit-log.jsonl", "navigation", "ai-settings.yaml"} {
		if err := status.MarkImportCompleted(source, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (s *StorageSyncService) readDatabaseSnapshot() (storageSnapshot, error) {
	config := s.config
	config.StorageOption = StorageOptionDatabase
	config.Driver = StorageDriverSQLite
	config.DatabaseURL = ""
	if config.SQLitePath == "" {
		config.SQLitePath = s.paths.SQLiteDatabaseFile
	}
	store, err := openSQLStore(config)
	if err != nil {
		return storageSnapshot{}, err
	}
	defer store.Close()
	return readRepositorySnapshot(RepositoryBundle{
		Workspaces: newSQLiteWorkspaceRepository(store, s.paths, s.git),
		Items:      &SQLiteItemRepository{db: store.db, driver: store.driver},
		Audit:      &SQLiteAuditRepository{db: store.db, driver: store.driver, now: time.Now},
		Navigation: &SQLiteNavigationRepository{db: store.db, driver: store.driver, now: time.Now},
		AISettings: &SQLiteAISettingsRepository{db: store.db, driver: store.driver},
	})
}

func readDataDirSnapshot(paths system.Paths) (storageSnapshot, error) {
	snapshot, err := readRepositorySnapshot(RepositoryBundle{
		Workspaces: registry.New(paths.RegistryFile, nil),
		Items:      itemindex.New(paths.PlanIndexFile),
		Audit:      audit.New(paths.AuditLogFile),
		Navigation: navigation.New(paths.SavedFiltersFile, paths.RecentItemsFile),
		AISettings: ai.NewSettingsRepository(paths.AISettingsFile),
	})
	if err != nil {
		return storageSnapshot{}, err
	}
	items, scans, err := readDataDirItemSnapshot(paths.PlanIndexFile)
	if err != nil {
		return storageSnapshot{}, err
	}
	snapshot.Items = items
	snapshot.Scans = scans
	return snapshot, nil
}

func readRepositorySnapshot(repositories RepositoryBundle) (storageSnapshot, error) {
	workspaces, err := repositories.Workspaces.List()
	if err != nil {
		return storageSnapshot{}, err
	}
	items, scans, err := readItemSnapshot(repositories.Items)
	if err != nil {
		return storageSnapshot{}, err
	}
	events, err := repositories.Audit.Recent(0)
	if err != nil {
		return storageSnapshot{}, err
	}
	filters, err := repositories.Navigation.Filters()
	if err != nil {
		return storageSnapshot{}, err
	}
	recents, err := repositories.Navigation.Recents(0)
	if err != nil {
		return storageSnapshot{}, err
	}
	settings, err := repositories.AISettings.Load()
	if err != nil {
		return storageSnapshot{}, err
	}
	return storageSnapshot{Workspaces: workspaces, Items: items, Scans: scans, Audit: events, Filters: filters, Recents: recents, AISettings: settings}, nil
}

func readItemSnapshot(repository itemindex.Repository) ([]models.ItemDetail, []models.BranchScanMetadata, error) {
	if sqlItems, ok := repository.(*SQLiteItemRepository); ok {
		items, err := sqlItems.queryDetails(itemindex.Query{})
		if err != nil {
			return nil, nil, err
		}
		scans, err := sqlItems.allBranchScans()
		return items, scans, err
	}
	summaries, err := repository.Query(itemindex.Query{})
	if err != nil {
		return nil, nil, err
	}
	items := make([]models.ItemDetail, 0, len(summaries))
	scanKeys := map[string]bool{}
	for _, summary := range summaries {
		item, ok, err := repository.Get(summary.ID)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		items = append(items, item)
		scanKeys[item.WorkspaceID+"\x00"+item.Branch] = true
	}
	scans := make([]models.BranchScanMetadata, 0, len(scanKeys))
	for key := range scanKeys {
		parts := strings.SplitN(key, "\x00", 2)
		scan, ok, err := repository.BranchScan(parts[0], parts[1])
		if err != nil {
			return nil, nil, err
		}
		if ok {
			scans = append(scans, scan)
		}
	}
	sort.Slice(scans, func(i, j int) bool {
		if scans[i].WorkspaceID == scans[j].WorkspaceID {
			return scans[i].Branch < scans[j].Branch
		}
		return scans[i].WorkspaceID < scans[j].WorkspaceID
	})
	return items, scans, nil
}

func readDataDirItemSnapshot(path string) ([]models.ItemDetail, []models.BranchScanMetadata, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []models.ItemDetail{}, []models.BranchScanMetadata{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var index fileIndexState
	if err := yaml.Unmarshal(data, &index); err != nil {
		return nil, nil, err
	}
	scans := []models.BranchScanMetadata{}
	for workspaceID, branches := range index.BranchScans {
		for branch, scan := range branches {
			scan.WorkspaceID = workspaceID
			scan.Branch = branch
			scans = append(scans, scan)
		}
	}
	sort.Slice(scans, func(i, j int) bool {
		if scans[i].WorkspaceID == scans[j].WorkspaceID {
			return scans[i].Branch < scans[j].Branch
		}
		return scans[i].WorkspaceID < scans[j].WorkspaceID
	})
	if index.Items == nil {
		index.Items = []models.ItemDetail{}
	}
	return index.Items, scans, nil
}

func (r *SQLiteItemRepository) allBranchScans() ([]models.BranchScanMetadata, error) {
	rows, err := querySQL(r.db, r.driver, `SELECT workspace_id, branch FROM branch_scans ORDER BY workspace_id, branch`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type branchKey struct{ workspaceID, branch string }
	keys := []branchKey{}
	for rows.Next() {
		var key branchKey
		if err := rows.Scan(&key.workspaceID, &key.branch); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	scans := make([]models.BranchScanMetadata, 0, len(keys))
	for _, key := range keys {
		scan, ok, err := r.BranchScan(key.workspaceID, key.branch)
		if err != nil {
			return nil, err
		}
		if ok {
			scans = append(scans, scan)
		}
	}
	return scans, nil
}

func clearSQLAppState(store *SQLStore) error {
	for _, table := range []string{"workspaces", "indexed_items", "branch_scans", "scan_warnings", "audit_events", "saved_filters", "recent_items", "ai_settings", "knowledge_indexes", "import_status"} {
		if _, err := execSQL(store.db, store.driver, "DELETE FROM "+table); err != nil {
			return err
		}
	}
	return nil
}

func writeDataDirSnapshot(paths system.Paths, snapshot storageSnapshot) error {
	if err := writeYAMLFile(paths.RegistryFile, snapshot.Workspaces); err != nil {
		return err
	}
	index := fileIndexState{Items: snapshot.Items, Warnings: []models.ScanWarning{}, Scans: map[string]time.Time{}, BranchScans: map[string]map[string]models.BranchScanMetadata{}}
	for _, scan := range snapshot.Scans {
		if index.BranchScans[scan.WorkspaceID] == nil {
			index.BranchScans[scan.WorkspaceID] = map[string]models.BranchScanMetadata{}
		}
		index.BranchScans[scan.WorkspaceID][scan.Branch] = scan
		index.Scans[scan.WorkspaceID] = scan.ScannedAt
		for _, warning := range scan.Warnings {
			warning.ItemPath = scan.WorkspaceID + ":" + scan.Branch + ":" + warning.ItemPath
			index.Warnings = append(index.Warnings, warning)
		}
	}
	if err := writeYAMLFile(paths.PlanIndexFile, index); err != nil {
		return err
	}
	if err := writeAuditLog(paths.AuditLogFile, snapshot.Audit); err != nil {
		return err
	}
	if err := writeYAMLFile(paths.SavedFiltersFile, snapshot.Filters); err != nil {
		return err
	}
	if err := writeYAMLFile(paths.RecentItemsFile, snapshot.Recents); err != nil {
		return err
	}
	return writeYAMLFile(paths.AISettingsFile, snapshot.AISettings)
}

func writeYAMLFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func writeAuditLog(path string, events []models.AuditEvent) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for i := len(events) - 1; i >= 0; i-- {
		data, err := json.Marshal(events[i])
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return file.Sync()
}

func backupDatabaseTarget(paths system.Paths, direction string, now time.Time) (string, error) {
	backupPath := storageBackupPath(paths, direction, now)
	if err := os.MkdirAll(backupPath, 0o755); err != nil {
		return "", err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		source := paths.SQLiteDatabaseFile + suffix
		if err := copyIfExists(source, filepath.Join(backupPath, filepath.Base(source))); err != nil {
			return "", err
		}
	}
	return backupPath, nil
}

func backupDataDirTarget(paths system.Paths, direction string, now time.Time) (string, error) {
	backupPath := storageBackupPath(paths, direction, now)
	if err := os.MkdirAll(backupPath, 0o755); err != nil {
		return "", err
	}
	for _, source := range []string{paths.RegistryFile, paths.PlanIndexFile, paths.AuditLogFile, paths.SavedFiltersFile, paths.RecentItemsFile, paths.AISettingsFile, paths.KnowledgeIndexFile} {
		if err := copyIfExists(source, filepath.Join(backupPath, filepath.Base(source))); err != nil {
			return "", err
		}
	}
	return backupPath, nil
}

func storageBackupPath(paths system.Paths, direction string, now time.Time) string {
	return filepath.Join(paths.Dir, "backups", "storage-sync", now.Format("20060102-150405")+"-"+direction)
}

func copyIfExists(source, target string) error {
	in, err := os.Open(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
