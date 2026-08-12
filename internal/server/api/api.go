package api

// Package api provides the Server HTTP transport.

import (
	"context"
	"net/http"
	"time"

	appaisession "kode-stream/internal/ai"
	"kode-stream/internal/audit"
	appcanvas "kode-stream/internal/canvas"
	"kode-stream/internal/cloudstate"
	fileaccess "kode-stream/internal/filesystem/content"
	appgit "kode-stream/internal/git"
	appitem "kode-stream/internal/item"
	"kode-stream/internal/item/index"
	"kode-stream/internal/item/writer"
	appjira "kode-stream/internal/jira"
	knowledgeindex "kode-stream/internal/knowledge"
	"kode-stream/internal/navigation"
	appruntime "kode-stream/internal/runtime"
	appsearch "kode-stream/internal/search"
	"kode-stream/internal/storage"
	"kode-stream/internal/system"
	appverification "kode-stream/internal/verification"
	appworkspace "kode-stream/internal/workspace"
	workspacehealth "kode-stream/internal/workspace"
	workspaceaccess "kode-stream/internal/workspace/files"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
	appworkstream "kode-stream/internal/workstream"
)

type API struct {
	workspace     *workspaceController
	item          *itemController
	git           *gitController
	health        *healthController
	state         *stateController
	search        *searchController
	audit         *auditController
	navigation    navigationRoutes
	system        systemRoutes
	aiSessions    *aiController
	jira          *jiraController
	knowledge     *knowledgeController
	verification  *verificationController
	canvas        *canvasController
	storage       *storageController
	cloud         *cloudController
	runtimeConfig system.RuntimeConfig
}

type navigationRoutes interface {
	Filters(http.ResponseWriter, *http.Request)
	SaveFilter(http.ResponseWriter, *http.Request)
	DeleteFilter(http.ResponseWriter, *http.Request)
	Recents(http.ResponseWriter, *http.Request)
	RecordRecent(http.ResponseWriter, *http.Request)
}

type systemRoutes interface {
	SelectDirectory(http.ResponseWriter, *http.Request)
	SelectFile(http.ResponseWriter, *http.Request)
	OpenPath(http.ResponseWriter, *http.Request)
	ConfigPaths(http.ResponseWriter, *http.Request)
	UpdateConfigPaths(http.ResponseWriter, *http.Request)
}

// Dependencies is the complete, immutable input to the HTTP transport. The
// production composition root builds this value once; API construction never
// mutates after routes have been registered.
type Dependencies struct {
	WorkspaceRepository registry.Repository
	ItemRepository      itemindex.Repository
	Scanner             *scanner.Scanner
	FileAccess          *fileaccess.Access
	ItemWriter          *itemwriter.Writer
	Git                 *appgit.GitAdapter
	Dialog              *system.Dialog
	Audit               audit.Repository
	WorkspaceHealth     *workspacehealth.HealthService
	Search              *appsearch.SearchService
	Navigation          navigation.Repository
	AISessions          *appaisession.Service
	Jira                *appjira.Service
	Knowledge           *knowledgeindex.KnowledgeService
	Verification        *appverification.Service
	Canvas              *appcanvas.Service
	RuntimeConfig       system.RuntimeConfig
	DatabaseHealth      databaseHealthChecker
	StorageStatus       storageStatusService
	StorageSync         storageSyncService
	CloudPersistence    cloudstate.Repository
	WorkspaceCloner     appworkspace.ClonePort
}

type databaseHealthChecker interface {
	Health(context.Context) storage.DatabaseHealth
}

type storageStatusService interface {
	Status(context.Context) storage.StorageStatus
}

type storageSyncService interface {
	Sync(context.Context, storage.StorageSyncRequest) (storage.StorageSyncResult, error)
}

// New is the sole API constructor.
func New(deps Dependencies) *API {
	var refresher appworkspace.Refresher
	if deps.ItemWriter != nil {
		refresher = deps.ItemWriter
	}
	var auditReader auditEventReader
	if deps.Audit != nil {
		auditReader = audit.NewCachedEventReader(deps.Audit, 2*time.Second, time.Now)
		if recorder, ok := deps.Audit.(*audit.Recorder); ok {
			recorder.AddInvalidator(auditReader.(*audit.CachedEventReader).Invalidate)
		}
	}
	workspaceFileAccess := workspaceaccess.New()
	workspaceCloner := deps.WorkspaceCloner
	if workspaceCloner == nil {
		workspaceCloner = deps.Git
	}
	workspaceService := appworkspace.New(appworkspace.ServiceDependencies{Registry: deps.WorkspaceRepository, Index: deps.ItemRepository, Scanner: deps.Scanner, Writer: deps.ItemWriter, Cloner: workspaceCloner, Audit: deps.Audit, JiraCache: deps.Jira})
	runtimeService := appruntime.NewService()
	runtimeConfig := deps.RuntimeConfig
	if runtimeConfig.Mode == "" {
		runtimeConfig, _ = system.ResolveRuntimeConfigFromEnv(func(string) string { return "" })
	}
	verificationService := deps.Verification
	if verificationService == nil {
		verificationService = appverification.NewService(deps.WorkspaceRepository, runtimeService)
	}
	cloudWorkspaces := newCloudWorkspaceStore(deps.CloudPersistence)
	workstreamService := appworkstream.New(deps.WorkspaceRepository, deps.ItemRepository, deps.Scanner, deps.Git)
	itemService := appitem.New(deps.WorkspaceRepository, deps.ItemRepository, deps.FileAccess, deps.ItemWriter, deps.Git)
	workspaceFiles := appworkspace.NewWorkspaceFileService(deps.WorkspaceRepository, workspaceFileAccess, deps.Git, deps.Audit, refresher)
	contentSearch := appsearch.NewContentSearchService(deps.WorkspaceRepository, deps.ItemRepository, workspaceFileAccess)
	recorder := auditRecorder{repository: deps.Audit, reader: auditReader}
	cloud := &cloudController{runtimeConfig: runtimeConfig, agents: newCloudAgentStore(time.Now, deps.CloudPersistence), workspaces: cloudWorkspaces, providers: newCloudProviderStore(deps.CloudPersistence, runtimeConfig.CookieSecret), audit: deps.Audit, available: true}
	knowledge := &knowledgeController{knowledge: deps.Knowledge}
	return &API{
		workspace: &workspaceController{workspaces: workspaceService, workstream: workstreamService, health: deps.WorkspaceHealth, files: workspaceFiles, contentSearch: contentSearch, runtimeConfig: runtimeConfig, cloudWorkspaces: cloudWorkspaces, cloudProviders: cloud.providers, audit: recorder},
		item:      &itemController{items: itemService, contentSearch: contentSearch, knowledge: deps.Knowledge, audit: recorder},
		git:       &gitController{gitOps: appgit.NewService(deps.WorkspaceRepository, deps.ItemWriter, deps.Git), audit: recorder},
		health:    &healthController{database: deps.DatabaseHealth},
		state:     &stateController{workspaces: workspaceService, runtimeConfig: runtimeConfig},
		search:    &searchController{search: deps.Search},
		audit:     &auditController{reader: auditReader},
		navigation: navigation.NewController(deps.Navigation, itemService, func(ctx context.Context) string {
			if session, ok := cloudSessionFromContext(ctx); ok {
				return session.User.ID
			}
			return navigation.LocalOwner
		}),
		system:        system.NewController(deps.Dialog),
		canvas:        &canvasController{canvas: deps.Canvas, workstream: appworkstream.New(deps.WorkspaceRepository, deps.ItemRepository, deps.Scanner, deps.Git)},
		aiSessions:    &aiController{aiSessions: deps.AISessions, runtimeConfig: runtimeConfig, cloudWorkspaces: cloudWorkspaces},
		jira:          &jiraController{jira: deps.Jira},
		knowledge:     knowledge,
		verification:  &verificationController{workspaces: workspaceService, verification: verificationService},
		runtimeConfig: runtimeConfig,
		storage:       &storageController{status: deps.StorageStatus, sync: deps.StorageSync},
		cloud:         cloud,
	}
}

func (a *API) Routes() http.Handler {
	a.initializeControllers()
	return newTransport(a.runtimeConfig, a.registerGinRoutes)
}

// initializeControllers keeps deliberately partial API fixtures safe. Each
// controller owns its unavailable-service response instead of relying on a
// constructor compatibility path.
func (a *API) initializeControllers() {
	if a.cloud == nil {
		a.cloud = &cloudController{runtimeConfig: a.runtimeConfig, agents: newCloudAgentStore(time.Now), workspaces: newCloudWorkspaceStore(), providers: newCloudProviderStore(nil, "")}
	}
	if a.workspace == nil {
		a.workspace = &workspaceController{runtimeConfig: a.runtimeConfig, cloudWorkspaces: a.cloud.workspaces, cloudProviders: a.cloud.providers}
	}
	if a.item == nil {
		a.item = &itemController{}
	}
	if a.git == nil {
		a.git = &gitController{}
	}
	if a.health == nil {
		a.health = &healthController{}
	}
	if a.state == nil {
		a.state = &stateController{runtimeConfig: a.runtimeConfig}
	}
	if a.search == nil {
		a.search = &searchController{}
	}
	if a.audit == nil {
		a.audit = &auditController{}
	}
	if a.navigation == nil {
		a.navigation = navigation.NewController(nil, nil)
	}
	if a.system == nil {
		a.system = system.NewController(nil)
	}
	if a.aiSessions == nil {
		a.aiSessions = &aiController{runtimeConfig: a.runtimeConfig, cloudWorkspaces: a.cloud.workspaces}
	}
	if a.jira == nil {
		a.jira = &jiraController{}
	}
	if a.knowledge == nil {
		a.knowledge = &knowledgeController{}
	}
	if a.verification == nil {
		a.verification = &verificationController{}
	}
	if a.canvas == nil {
		a.canvas = &canvasController{}
	}
	if a.storage == nil {
		a.storage = &storageController{}
	}
}
