package server

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"kode-stream/internal/ai"
	"kode-stream/internal/audit"
	"kode-stream/internal/filesystem/content"
	appgit "kode-stream/internal/git"
	"kode-stream/internal/item/index"
	"kode-stream/internal/item/writer"
	appjira "kode-stream/internal/jira"
	"kode-stream/internal/knowledge"
	"kode-stream/internal/navigation"
	appsearch "kode-stream/internal/search"
	"kode-stream/internal/server/api"
	"kode-stream/internal/system"
	"kode-stream/internal/workspace"
	"kode-stream/internal/workspace/registry"
	"kode-stream/internal/workspace/scanner"
)

//go:embed all:frontend
var frontendFS embed.FS

type Server struct {
	port        int
	bindAddress string
	app         http.Handler
	sessions    *ai.Manager
}

func NewServer(port int) (*Server, error) {
	paths, err := system.ResolvePaths()
	if err != nil {
		return nil, err
	}
	runtimeConfig, err := system.ResolveRuntimeConfig()
	if err != nil {
		return nil, err
	}
	if port == 0 {
		port = envPort()
	}
	git := appgit.New()
	reg := registry.New(paths.RegistryFile, git)
	idx := itemindex.New(paths.PlanIndexFile)
	scan := scanner.New(git)
	files := fileaccess.New()
	writer := itemwriter.New(files, scan, idx, reg)
	auditStore := audit.New(paths.AuditLogFile)
	healthService := workspace.NewHealthService(reg, idx, git)
	searchService := appsearch.New(idx)
	navigationStore := navigation.New(paths.SavedFiltersFile, paths.RecentItemsFile)
	sessionManager := ai.NewTerminalManager(ai.Config{})
	aiSessionService := ai.New(ai.NewSettingsRepository(paths.AISettingsFile)).ConfigureLaunch(reg, idx, auditStore, os.TempDir()).ConfigureEmbedded(sessionManager)
	jiraService := appjira.NewService(reg, idx, appjira.New())
	knowledgeService := knowledge.NewService(reg, knowledge.NewStore(paths.KnowledgeIndexFile)).ConfigureActions(knowledge.NewDetector(), appgit.NewService(reg, writer, git), auditStore)
	apiHandler := api.NewWithServices(reg, idx, scan, files, writer, git, system.New(), auditStore, healthService, searchService, navigationStore).WithRuntimeConfig(runtimeConfig).WithAISessions(aiSessionService).WithJira(jiraService).WithKnowledge(knowledgeService)

	mux := http.NewServeMux()
	mux.Handle("/api/", apiHandler.Routes())
	mux.Handle("/", spaHandler())
	return &Server{port: port, bindAddress: runtimeConfig.BindAddress, app: api.Log(mux), sessions: sessionManager}, nil
}

func (s *Server) Close() error {
	if s.sessions != nil {
		return s.sessions.Close()
	}
	return nil
}

func (s *Server) Serve() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.bindAddress, s.port))
	if err != nil {
		return err
	}
	url := "http://" + listener.Addr().String()
	fmt.Printf("Kode Stream running at %s\n", url)
	stopping := make(chan os.Signal, 1)
	signal.Notify(stopping, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopping)
	go func() { <-stopping; _ = s.Close(); _ = listener.Close() }()
	err = http.Serve(listener, s.app)
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func envPort() int {
	raw := strings.TrimSpace(os.Getenv("KODE_STREAM_PORT"))
	if raw == "" {
		return 4317
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 {
		return 4317
	}
	return port
}

func spaHandler() http.Handler {
	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deprecatedSPAPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/" {
			if _, err := fs.Stat(sub, strings.TrimPrefix(r.URL.Path, "/")); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}

func deprecatedSPAPath(path string) bool {
	return path == "/kanban" ||
		strings.HasPrefix(path, "/kanban/") ||
		path == "/workbench" ||
		strings.HasPrefix(path, "/workbench/") ||
		path == "/workspace" ||
		strings.HasPrefix(path, "/workspace/")
}
