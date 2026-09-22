package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	publicanalyzer "github.com/smallshellctw/whichrepo/analyzer"
	"github.com/smallshellctw/whichrepo/internal/config"
	"github.com/smallshellctw/whichrepo/internal/dashboard"
	"github.com/smallshellctw/whichrepo/internal/evaluation"
	"github.com/smallshellctw/whichrepo/internal/gateway"
	"github.com/smallshellctw/whichrepo/internal/index"
	"github.com/smallshellctw/whichrepo/internal/mcp"
	"github.com/smallshellctw/whichrepo/internal/model"
	"github.com/smallshellctw/whichrepo/internal/router"
	"github.com/smallshellctw/whichrepo/internal/scanner"
)

var version = "0.1.0-dev"

type settings struct {
	workspace  string
	dbPath     string
	configPath string
	provider   string
	topK       int
}

func main() {
	ctx := context.Background()
	if len(os.Args) < 2 {
		usage()
		return
	}
	command := os.Args[1]
	args := os.Args[2:]
	known := map[string]bool{"init": true, "index": true, "route": true, "dashboard": true, "mcp": true, "doctor": true, "eval": true, "help": true, "version": true, "-h": true, "--help": true, "--version": true}
	if !known[command] {
		command = "route"
		args = os.Args[1:]
	}
	var err error
	switch command {
	case "init":
		err = runInit(ctx, args)
	case "index":
		err = runIndex(ctx, args)
	case "route":
		err = runRoute(ctx, args)
	case "dashboard":
		err = runDashboard(ctx, args)
	case "mcp":
		err = runMCP(ctx, args)
	case "doctor":
		err = runDoctor(ctx, args)
	case "eval":
		err = runEval(ctx, args)
	case "version", "--version":
		fmt.Println(version)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runInit(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	name := flags.String("name", "", "workspace display name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspace := "."
	if flags.NArg() > 0 {
		workspace = flags.Arg(0)
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return err
	}
	configPath := filepath.Join(absolute, ".whichrepo.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists", configPath)
	}
	workspaceName := *name
	if workspaceName == "" {
		workspaceName = filepath.Base(absolute)
	}
	cfg := config.Config{Version: 1, Workspace: config.Workspace{Name: workspaceName, Exclude: []string{"archive-*", "tmp-*"}}, Projects: map[string]config.ProjectOverride{}}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return err
	}
	settings := defaultSettings(absolute)
	settings.configPath = configPath
	count, err := refresh(ctx, settings)
	if err != nil {
		return err
	}
	return writeJSON(map[string]any{"workspace": absolute, "config": configPath, "indexed_projects": count})
}

func runIndex(ctx context.Context, args []string) error {
	flags, cfgRef := commandFlags("index")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := *cfgRef
	count, err := refresh(ctx, cfg)
	if err != nil {
		return err
	}
	return writeJSON(map[string]any{"indexed_projects": count, "workspace": cfg.workspace, "database": cfg.dbPath})
}

func runRoute(ctx context.Context, args []string) error {
	flags, cfgRef := commandFlags("route")
	flags.IntVar(&cfgRef.topK, "top-k", 8, "number of local candidates passed to the decision provider")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := *cfgRef
	task := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if task == "" {
		return fmt.Errorf("usage: whichrepo route [flags] <task>")
	}
	store, engine, err := openEngine(cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	result, err := engine.Route(ctx, task, cfg.topK)
	if err != nil {
		return err
	}
	return writeJSON(result)
}

func runDashboard(ctx context.Context, args []string) error {
	flags, cfgRef := commandFlags("dashboard")
	address := flags.String("listen", "127.0.0.1:8787", "loopback dashboard address")
	openBrowser := flags.Bool("open", false, "open the dashboard in the default browser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := *cfgRef
	store, engine, err := openEngine(cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	server := dashboard.Server{Router: engine, Store: store, Workspace: cfg.workspace, Provider: cfg.provider, Refresh: func(ctx context.Context) (int, error) { return refreshWithStore(ctx, cfg, store) }}
	_, err = server.Serve(ctx, *address, *openBrowser)
	if err == nil || err.Error() == "http: Server closed" {
		return nil
	}
	return err
}

func runMCP(ctx context.Context, args []string) error {
	flags, cfgRef := commandFlags("mcp")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := *cfgRef
	store, engine, err := openEngine(cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	server := mcp.Server{
		Route: func(ctx context.Context, task string, topK int) (any, error) { return engine.Route(ctx, task, topK) },
		Refresh: func(ctx context.Context) (any, error) {
			count, err := refreshWithStore(ctx, cfg, store)
			return map[string]any{"indexed_projects": count, "workspace": cfg.workspace}, err
		},
		ListProjects: func(ctx context.Context) (any, error) {
			projects, err := store.Projects(ctx)
			if err != nil {
				return nil, err
			}
			for i := range projects {
				projects[i].Content = ""
			}
			return projects, nil
		},
		WorkspaceStatus: func(ctx context.Context) (any, error) { return workspaceStatus(ctx, store, cfg), nil },
	}
	return server.Serve(ctx, os.Stdin, os.Stdout)
}

func runDoctor(ctx context.Context, args []string) error {
	flags, cfgRef := commandFlags("doctor")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := *cfgRef
	store, err := index.Open(cfg.dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	status := workspaceStatus(ctx, store, cfg)
	return writeJSON(map[string]any{
		"version":                 version,
		"built_with_go":           runtime.Version(),
		"runtime_dependencies":    []string{},
		"cgo_required":            false,
		"analyzer_api_version":    publicanalyzer.APIVersion,
		"language_analyzers":      []string{"Go", "JavaScript", "TypeScript", "Python", "Java", "Rust", ".NET", "PHP", "Ruby"},
		"os":                      runtime.GOOS,
		"arch":                    runtime.GOARCH,
		"workspace":               status,
		"config":                  cfg.configPath,
		"database":                cfg.dbPath,
		"provider_key_configured": newDecisionClient(cfg.provider).Available(),
	})
}

func runEval(ctx context.Context, args []string) error {
	flags, cfgRef := commandFlags("eval")
	dataset := flags.String("dataset", "", "JSONL dataset path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg := *cfgRef
	if *dataset == "" {
		return fmt.Errorf("--dataset is required")
	}
	store, engine, err := openEngine(cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	report, err := evaluation.Run(ctx, engine, *dataset)
	if err != nil {
		return err
	}
	return writeJSON(report)
}

func openEngine(cfg settings) (*index.Store, router.Router, error) {
	store, err := index.Open(cfg.dbPath)
	if err != nil {
		return nil, router.Router{}, err
	}
	return store, router.Router{Store: store, Gateway: newDecisionClient(cfg.provider)}, nil
}

func newDecisionClient(provider string) gateway.Client {
	client := gateway.Client{Provider: provider}
	switch provider {
	case "jev-vercel":
		client.APIKey = os.Getenv("AI_GATEWAY_API_KEY")
		client.Endpoint = gateway.VercelEndpoint
		client.Model = "jev-latest"
	case "jev-typesafe":
		client.APIKey = os.Getenv("TYPESAFE_API_KEY")
		client.Endpoint = gateway.TypeSafeEndpoint
		client.Model = "jev-latest"
	case "jev-openrouter":
		client.APIKey = os.Getenv("OPENROUTER_API_KEY")
		client.Endpoint = gateway.OpenRouterEndpoint
		client.Model = "typesafe/jev-1.13"
	default:
		client.Provider = "local"
	}
	if endpoint := os.Getenv("WHICHREPO_PROVIDER_URL"); endpoint != "" {
		client.Endpoint = endpoint
	}
	if modelName := os.Getenv("WHICHREPO_MODEL"); modelName != "" {
		client.Model = modelName
	}
	return client
}

func refresh(ctx context.Context, cfg settings) (int, error) {
	store, err := index.Open(cfg.dbPath)
	if err != nil {
		return 0, err
	}
	defer store.Close()
	return refreshWithStore(ctx, cfg, store)
}
func refreshWithStore(ctx context.Context, cfg settings, store *index.Store) (int, error) {
	configuration, err := config.Load(cfg.configPath)
	if err != nil {
		return 0, fmt.Errorf("load workspace config: %w", err)
	}
	projects, err := (scanner.Scanner{Root: cfg.workspace, Config: configuration}).Scan()
	if err != nil {
		return 0, err
	}
	indexCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := store.ReplaceProjects(indexCtx, projects); err != nil {
		return 0, err
	}
	return len(projects), nil
}

func workspaceStatus(ctx context.Context, store *index.Store, cfg settings) model.WorkspaceStatus {
	projects, _ := store.Projects(ctx)
	var latest time.Time
	for _, project := range projects {
		if project.IndexedAt.After(latest) {
			latest = project.IndexedAt
		}
	}
	return model.WorkspaceStatus{Workspace: cfg.workspace, ProjectCount: len(projects), LastIndexedAt: latest, DecisionProvider: cfg.provider}
}

func commandFlags(name string) (*flag.FlagSet, *settings) {
	cwd, _ := os.Getwd()
	cfg := defaultSettings(cwd)
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.StringVar(&cfg.workspace, "workspace", cfg.workspace, "workspace root")
	flags.StringVar(&cfg.dbPath, "db", cfg.dbPath, "SQLite index path")
	flags.StringVar(&cfg.configPath, "config", cfg.configPath, "workspace configuration")
	flags.StringVar(&cfg.provider, "provider", cfg.provider, "local, jev-vercel, jev-typesafe, or jev-openrouter")
	return flags, &cfg
}

func defaultSettings(workspace string) settings {
	absolute, _ := filepath.Abs(envOr("WHICHREPO_WORKSPACE", workspace))
	return settings{workspace: absolute, dbPath: envOr("WHICHREPO_DB", config.DefaultDBPath()), configPath: config.ResolveConfig(absolute, os.Getenv("WHICHREPO_CONFIG")), provider: envOr("WHICHREPO_PROVIDER", "local")}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
func usage() {
	fmt.Fprintln(os.Stderr, `WhichRepo — Describe the task. Find the repo. Show the evidence.

Commands:
  init [workspace]      create .whichrepo.yaml and build the first index
  index                 rebuild the local index
  route <task>          route a task to candidate projects
  dashboard             open the local signal-map dashboard
  mcp                   run the stdio MCP server
  doctor                inspect local setup
  eval --dataset FILE   evaluate routing accuracy

You can also run: whichrepo "your engineering task"`)
}
