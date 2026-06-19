package app

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"plan-manager/internal/api"
	"plan-manager/internal/application/health"
	appsearch "plan-manager/internal/application/search"
	"plan-manager/internal/audit"
	"plan-manager/internal/config"
	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/itemwriter"
	"plan-manager/internal/navigation"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
	"plan-manager/internal/systemdialog"
)

//go:embed all:frontend
var frontendFS embed.FS

type Server struct {
	port int
	app  http.Handler
}

func NewServer(port int) (*Server, error) {
	paths, err := config.ResolvePaths()
	if err != nil {
		return nil, err
	}
	if port == 0 {
		port = envPort()
	}
	git := gitadapter.New()
	reg := registry.New(paths.RegistryFile, git)
	idx := itemindex.New(paths.PlanIndexFile)
	scan := scanner.New(git)
	files := fileaccess.New()
	writer := itemwriter.New(files, scan, idx, reg)
	auditStore := audit.New(paths.AuditLogFile)
	healthService := health.New(reg, idx, git)
	searchService := appsearch.New(idx)
	navigationStore := navigation.New(paths.SavedFiltersFile, paths.RecentItemsFile)
	apiHandler := api.NewWithServices(reg, idx, scan, files, writer, git, systemdialog.New(), auditStore, healthService, searchService, navigationStore)

	mux := http.NewServeMux()
	mux.Handle("/api/", apiHandler.Routes())
	mux.Handle("/", spaHandler())
	return &Server{port: port, app: api.Log(mux)}, nil
}

func (s *Server) Serve() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return err
	}
	url := "http://" + listener.Addr().String()
	fmt.Printf("Plan Manager running at %s\n", url)
	return http.Serve(listener, s.app)
}

func envPort() int {
	raw := strings.TrimSpace(os.Getenv("PLAN_MANAGER_PORT"))
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
