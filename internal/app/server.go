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
	"plan-manager/internal/config"
	"plan-manager/internal/fileaccess"
	"plan-manager/internal/gitadapter"
	"plan-manager/internal/planindex"
	"plan-manager/internal/registry"
	"plan-manager/internal/scanner"
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
	idx := planindex.New(paths.PlanIndexFile)
	apiHandler := api.New(reg, idx, scanner.New(git), fileaccess.New(), git)

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
