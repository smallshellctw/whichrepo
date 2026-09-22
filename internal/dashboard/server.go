package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/smallshellctw/whichrepo/internal/index"
	"github.com/smallshellctw/whichrepo/internal/model"
	"github.com/smallshellctw/whichrepo/internal/router"
)

//go:embed static/*
var assets embed.FS

type RefreshFunc func(context.Context) (int, error)

type Server struct {
	Router      router.Router
	Store       *index.Store
	Workspace   string
	Provider    string
	DefaultTopK int
	Refresh     RefreshFunc
}

func (s Server) Serve(ctx context.Context, address string, openBrowser bool) (string, error) {
	if address == "" {
		address = "127.0.0.1:0"
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("invalid dashboard address: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return "", fmt.Errorf("dashboard may only bind to a loopback address")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return "", err
	}
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		_ = listener.Close()
		return "", err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/route", s.handleRoute)
	mux.HandleFunc("GET /api/projects", s.handleProjects)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/index/refresh", s.handleRefresh)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	fileServer := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			if _, err := fs.Stat(staticFS, strings.TrimPrefix(request.URL.Path, "/")); err == nil {
				fileServer.ServeHTTP(w, request)
				return
			}
		}
		data, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	httpServer := &http.Server{Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second}
	url := "http://" + listener.Addr().String()
	if openBrowser {
		_ = openURL(url)
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	fmt.Println(url)
	return url, httpServer.Serve(listener)
}

func (s Server) handleRoute(w http.ResponseWriter, request *http.Request) {
	var input struct {
		Task string `json:"task"`
		TopK int    `json:"top_k"`
	}
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil || strings.TrimSpace(input.Task) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task is required"})
		return
	}
	if input.TopK <= 0 {
		input.TopK = s.DefaultTopK
	}
	result, err := s.Router.Route(request.Context(), input.Task, input.TopK)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s Server) handleProjects(w http.ResponseWriter, request *http.Request) {
	projects, err := s.Store.Projects(request.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for i := range projects {
		projects[i].Content = ""
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s Server) handleStatus(w http.ResponseWriter, request *http.Request) {
	projects, err := s.Store.Projects(request.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var latest time.Time
	for _, project := range projects {
		if project.IndexedAt.After(latest) {
			latest = project.IndexedAt
		}
	}
	writeJSON(w, http.StatusOK, model.WorkspaceStatus{Workspace: s.Workspace, ProjectCount: len(projects), LastIndexedAt: latest, DecisionProvider: s.Provider})
}

func (s Server) handleRefresh(w http.ResponseWriter, request *http.Request) {
	if s.Refresh == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "refresh is not configured"})
		return
	}
	count, err := s.Refresh(request.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"indexed_projects": count})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func openURL(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	return command.Start()
}
