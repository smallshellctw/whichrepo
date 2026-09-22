package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	publicanalyzer "github.com/smallshellctw/whichrepo/analyzer"
	"github.com/smallshellctw/whichrepo/internal/agentsetup"
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
	workspace       string
	dbPath          string
	configPath      string
	provider        string
	model           string
	providerURL     string
	providerTimeout time.Duration
	topK            int
	dashboardListen string
	dashboardOpen   bool
	explicitConfig  string
}

func main() {
	ctx := context.Background()
	if len(os.Args) < 2 {
		usage()
		return
	}
	command := os.Args[1]
	args := os.Args[2:]
	known := map[string]bool{"init": true, "index": true, "route": true, "dashboard": true, "mcp": true, "doctor": true, "eval": true, "config": true, "setup": true, "help": true, "version": true, "-h": true, "--help": true, "--version": true}
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
	case "config":
		err = runConfig(args)
	case "setup":
		err = runSetup(args)
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
	data, err := config.Render(workspaceName)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return err
	}
	settings, err := defaultSettings(absolute, "")
	if err != nil {
		return err
	}
	count, err := refresh(ctx, settings)
	if err != nil {
		return err
	}
	return writeJSON(map[string]any{"workspace": absolute, "config": configPath, "indexed_projects": count})
}

func runIndex(ctx context.Context, args []string) error {
	flags, cfgRef, err := commandFlags("index", args)
	if err != nil {
		return err
	}
	if err := parseSettingsFlags(flags, args, cfgRef); err != nil {
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
	flags, cfgRef, err := commandFlags("route", args)
	if err != nil {
		return err
	}
	flags.IntVar(&cfgRef.topK, "top-k", cfgRef.topK, "number of local candidates passed to the decision provider")
	if err := parseSettingsFlags(flags, args, cfgRef); err != nil {
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
	flags, cfgRef, err := commandFlags("dashboard", args)
	if err != nil {
		return err
	}
	flags.StringVar(&cfgRef.dashboardListen, "listen", cfgRef.dashboardListen, "loopback dashboard address")
	flags.BoolVar(&cfgRef.dashboardOpen, "open", cfgRef.dashboardOpen, "open the dashboard in the default browser")
	if err := parseSettingsFlags(flags, args, cfgRef); err != nil {
		return err
	}
	cfg := *cfgRef
	store, engine, err := openEngine(cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	server := dashboard.Server{Router: engine, Store: store, Workspace: cfg.workspace, Provider: cfg.provider, DefaultTopK: cfg.topK, Refresh: func(ctx context.Context) (int, error) { return refreshWithStore(ctx, cfg, store) }}
	_, err = server.Serve(ctx, cfg.dashboardListen, cfg.dashboardOpen)
	if err == nil || err.Error() == "http: Server closed" {
		return nil
	}
	return err
}

func runMCP(ctx context.Context, args []string) error {
	flags, cfgRef, err := commandFlags("mcp", args)
	if err != nil {
		return err
	}
	if err := parseSettingsFlags(flags, args, cfgRef); err != nil {
		return err
	}
	cfg := *cfgRef
	store, engine, err := openEngine(cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	server := mcp.Server{
		Route: func(ctx context.Context, task string, topK int) (any, error) {
			if topK <= 0 {
				topK = cfg.topK
			}
			return engine.Route(ctx, task, topK)
		},
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
	flags, cfgRef, err := commandFlags("doctor", args)
	if err != nil {
		return err
	}
	if err := parseSettingsFlags(flags, args, cfgRef); err != nil {
		return err
	}
	cfg := *cfgRef
	store, err := index.Open(cfg.dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	status := workspaceStatus(ctx, store, cfg)
	warnings := []string{}
	if info, statErr := os.Stat(cfg.workspace); statErr != nil || !info.IsDir() {
		warnings = append(warnings, "workspace directory does not exist")
	}
	configExists := false
	for _, path := range config.ConfigPaths(cfg.workspace, cfg.explicitConfig) {
		if _, statErr := os.Stat(path); statErr == nil {
			configExists = true
		}
	}
	if !configExists {
		warnings = append(warnings, "no workspace configuration found; run whichrepo init <workspace>")
	}
	if status.ProjectCount == 0 {
		warnings = append(warnings, "index contains no projects; run whichrepo index")
	}
	providerConfigured := newDecisionClient(cfg).Available()
	if cfg.provider != "local" && !providerConfigured {
		warnings = append(warnings, "decision provider selected but its API key is not configured; routing will fall back to local")
	}
	return writeJSON(map[string]any{
		"healthy":                 len(warnings) == 0,
		"warnings":                warnings,
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
		"config_sources":          config.ConfigPaths(cfg.workspace, cfg.explicitConfig),
		"database":                cfg.dbPath,
		"provider":                cfg.provider,
		"provider_key_configured": providerConfigured,
		"recommended_next_step":   "whichrepo setup auto --workspace " + cfg.workspace,
	})
}

func runEval(ctx context.Context, args []string) error {
	flags, cfgRef, err := commandFlags("eval", args)
	if err != nil {
		return err
	}
	dataset := flags.String("dataset", "", "JSONL dataset path")
	if err := parseSettingsFlags(flags, args, cfgRef); err != nil {
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

func runConfig(args []string) error {
	action := "show"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action = args[0]
		args = args[1:]
	}
	flags, cfgRef, err := commandFlags("config", args)
	if err != nil {
		return err
	}
	if err := parseSettingsFlags(flags, args, cfgRef); err != nil {
		return err
	}
	cfg := *cfgRef
	switch action {
	case "show":
		configuration, err := config.LoadWorkspace(cfg.workspace, cfg.explicitConfig)
		if err != nil {
			return err
		}
		data, err := yaml.Marshal(configuration)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	case "paths":
		paths := config.ConfigPaths(cfg.workspace, cfg.explicitConfig)
		items := make([]map[string]any, 0, len(paths))
		for _, path := range paths {
			_, statErr := os.Stat(path)
			items = append(items, map[string]any{"path": path, "exists": statErr == nil})
		}
		return writeJSON(items)
	case "defaults":
		data, err := config.Render(filepath.Base(cfg.workspace))
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	case "validate":
		configuration, err := config.LoadWorkspace(cfg.workspace, cfg.explicitConfig)
		if err != nil {
			return err
		}
		return writeJSON(map[string]any{
			"valid":   true,
			"version": configuration.Version,
			"sources": config.ConfigPaths(cfg.workspace, cfg.explicitConfig),
		})
	default:
		return fmt.Errorf("usage: whichrepo config [show|paths|defaults|validate] [flags]")
	}
}

func runSetup(args []string) error {
	client := "auto"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		client = args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet("setup", flag.ContinueOnError)
	cwd, _ := os.Getwd()
	workspace := flags.String("workspace", envOr("WHICHREPO_WORKSPACE", cwd), "workspace root")
	scope := flags.String("scope", "user", "user or project")
	dryRun := flags.Bool("dry-run", false, "show the planned client changes without applying them")
	if err := flags.Parse(args); err != nil {
		return err
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	absoluteWorkspace, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	results, err := agentsetup.Apply(context.Background(), agentsetup.Options{
		Client: client, Scope: *scope, Workspace: absoluteWorkspace, Binary: binary, DryRun: *dryRun,
	})
	if err != nil {
		return err
	}
	return writeJSON(map[string]any{
		"workspace": absoluteWorkspace,
		"results":   results,
		"next":      "restart the configured client, then ask it to call whichrepo.workspace_status",
	})
}

func openEngine(cfg settings) (*index.Store, router.Router, error) {
	store, err := index.Open(cfg.dbPath)
	if err != nil {
		return nil, router.Router{}, err
	}
	return store, router.Router{Store: store, Gateway: newDecisionClient(cfg)}, nil
}

func newDecisionClient(cfg settings) gateway.Client {
	client := gateway.Client{Provider: cfg.provider, Model: cfg.model, Endpoint: cfg.providerURL, HTTPClient: &http.Client{Timeout: cfg.providerTimeout}}
	switch cfg.provider {
	case "jev-vercel":
		client.APIKey = os.Getenv("AI_GATEWAY_API_KEY")
		if client.Endpoint == "" {
			client.Endpoint = gateway.VercelEndpoint
		}
		if client.Model == "" {
			client.Model = "jev-latest"
		}
	case "jev-typesafe":
		client.APIKey = os.Getenv("TYPESAFE_API_KEY")
		if client.Endpoint == "" {
			client.Endpoint = gateway.TypeSafeEndpoint
		}
		if client.Model == "" {
			client.Model = "jev-latest"
		}
	case "jev-openrouter":
		client.APIKey = os.Getenv("OPENROUTER_API_KEY")
		if client.Endpoint == "" {
			client.Endpoint = gateway.OpenRouterEndpoint
		}
		if client.Model == "" {
			client.Model = "typesafe/jev-1.13"
		}
	default:
		client.Provider = "local"
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
	configuration, err := config.LoadWorkspace(cfg.workspace, cfg.explicitConfig)
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

func commandFlags(name string, args []string) (*flag.FlagSet, *settings, error) {
	cwd, _ := os.Getwd()
	workspace := envOr("WHICHREPO_WORKSPACE", flagValue(args, "workspace", cwd))
	explicitConfig := envOr("WHICHREPO_CONFIG", flagValue(args, "config", ""))
	cfg, err := defaultSettings(workspace, explicitConfig)
	if err != nil {
		return nil, nil, err
	}
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.StringVar(&cfg.workspace, "workspace", cfg.workspace, "workspace root")
	flags.StringVar(&cfg.dbPath, "db", cfg.dbPath, "SQLite index path")
	flags.StringVar(&cfg.configPath, "config", cfg.configPath, "workspace configuration")
	flags.StringVar(&cfg.provider, "provider", cfg.provider, "local, jev-vercel, jev-typesafe, or jev-openrouter")
	flags.StringVar(&cfg.model, "model", cfg.model, "decision provider model")
	flags.StringVar(&cfg.providerURL, "provider-url", cfg.providerURL, "decision provider endpoint")
	flags.DurationVar(&cfg.providerTimeout, "provider-timeout", cfg.providerTimeout, "decision provider HTTP timeout")
	return flags, &cfg, nil
}

func parseSettingsFlags(flags *flag.FlagSet, args []string, cfg *settings) error {
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspace, err := filepath.Abs(cfg.workspace)
	if err != nil {
		return err
	}
	cfg.workspace = workspace
	if cfg.explicitConfig != "" {
		cfg.explicitConfig, err = filepath.Abs(cfg.explicitConfig)
		if err != nil {
			return err
		}
	}
	cfg.configPath = config.ResolveConfig(cfg.workspace, cfg.explicitConfig)
	if !filepath.IsAbs(cfg.dbPath) {
		cfg.dbPath, err = filepath.Abs(cfg.dbPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func defaultSettings(workspace, explicitConfig string) (settings, error) {
	absolute, _ := filepath.Abs(envOr("WHICHREPO_WORKSPACE", workspace))
	configuration, err := config.LoadWorkspace(absolute, explicitConfig)
	if err != nil {
		return settings{}, err
	}
	timeout, err := time.ParseDuration(envOr("WHICHREPO_PROVIDER_TIMEOUT", configuration.Routing.ProviderTimeout))
	if err != nil || timeout <= 0 {
		return settings{}, fmt.Errorf("invalid routing.provider_timeout %q", configuration.Routing.ProviderTimeout)
	}
	dbPath := configuration.Index.Database
	if dbPath == "" {
		dbPath = config.DefaultDBPath()
	} else if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(absolute, dbPath)
	}
	return settings{
		workspace:       absolute,
		dbPath:          envOr("WHICHREPO_DB", dbPath),
		configPath:      config.ResolveConfig(absolute, explicitConfig),
		explicitConfig:  explicitConfig,
		provider:        envOr("WHICHREPO_PROVIDER", configuration.Routing.Provider),
		model:           envOr("WHICHREPO_MODEL", configuration.Routing.Model),
		providerURL:     envOr("WHICHREPO_PROVIDER_URL", configuration.Routing.ProviderURL),
		providerTimeout: timeout,
		topK:            configuration.Routing.TopK,
		dashboardListen: configuration.Dashboard.Listen,
		dashboardOpen:   configuration.Dashboard.Open,
	}, nil
}

func flagValue(args []string, name, fallback string) string {
	long := "--" + name
	short := "-" + name
	for index, arg := range args {
		if arg == long || arg == short {
			if index+1 < len(args) {
				return args[index+1]
			}
		}
		for _, prefix := range []string{long + "=", short + "="} {
			if strings.HasPrefix(arg, prefix) {
				return strings.TrimPrefix(arg, prefix)
			}
		}
	}
	return fallback
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
  config <action>       show, validate, or locate configuration
  setup <client>        configure Codex, Claude Code, or Cursor MCP

You can also run: whichrepo "your engineering task"`)
}
