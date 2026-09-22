package agentsetup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Options struct {
	Client    string
	Scope     string
	Workspace string
	Binary    string
	DryRun    bool
}

type Result struct {
	Client     string   `json:"client"`
	Status     string   `json:"status"`
	Command    []string `json:"command,omitempty"`
	ConfigPath string   `json:"config_path,omitempty"`
	BackupPath string   `json:"backup_path,omitempty"`
}

func Apply(ctx context.Context, options Options) ([]Result, error) {
	if options.Scope == "" {
		options.Scope = "user"
	}
	if options.Scope != "user" && options.Scope != "project" {
		return nil, fmt.Errorf("scope must be user or project")
	}
	workspace, err := filepath.Abs(options.Workspace)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("workspace directory does not exist: %s", workspace)
	}
	binary, err := filepath.Abs(options.Binary)
	if err != nil {
		return nil, err
	}
	clients := []string{strings.ToLower(options.Client)}
	if len(clients) == 0 || clients[0] == "" || clients[0] == "auto" {
		clients = detectedClients()
	}
	if len(clients) == 0 {
		return nil, fmt.Errorf("no supported client detected; choose codex, claude, or cursor")
	}
	results := make([]Result, 0, len(clients))
	for _, client := range clients {
		var result Result
		switch client {
		case "codex":
			result, err = setupCommand(ctx, options.DryRun, "codex", []string{"mcp", "add", "whichrepo", "--", binary, "mcp", "--workspace", workspace})
		case "claude", "claude-code":
			result, err = setupCommand(ctx, options.DryRun, "claude", []string{"mcp", "add", "whichrepo", "--scope", options.Scope, "--", binary, "mcp", "--workspace", workspace})
		case "cursor":
			result, err = setupCursor(options, binary, workspace)
		default:
			return nil, fmt.Errorf("unsupported client %q; choose auto, codex, claude, or cursor", client)
		}
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func detectedClients() []string {
	clients := []string{}
	for _, name := range []string{"codex", "claude"} {
		if _, err := exec.LookPath(name); err == nil {
			clients = append(clients, name)
		}
	}
	home, _ := os.UserHomeDir()
	if _, err := exec.LookPath("cursor"); err == nil || directoryExists(filepath.Join(home, ".cursor")) {
		clients = append(clients, "cursor")
	}
	return clients
}

func setupCommand(ctx context.Context, dryRun bool, client string, args []string) (Result, error) {
	result := Result{Client: client, Command: append([]string{client}, args...)}
	path, err := exec.LookPath(client)
	if err != nil {
		result.Status = "not_installed"
		return result, fmt.Errorf("%s is not installed or not on PATH", client)
	}
	if dryRun {
		result.Status = "dry_run"
		return result, nil
	}
	command := exec.CommandContext(ctx, path, args...)
	output, err := command.CombinedOutput()
	if err != nil && client == "claude" && strings.Contains(strings.ToLower(string(output)), "already exists") {
		scope := argumentValue(args, "--scope", "user")
		remove := exec.CommandContext(ctx, path, "mcp", "remove", "whichrepo", "--scope", scope)
		removeOutput, removeErr := remove.CombinedOutput()
		if removeErr != nil {
			return result, fmt.Errorf("replace existing claude MCP server: %w: %s", removeErr, strings.TrimSpace(string(removeOutput)))
		}
		command = exec.CommandContext(ctx, path, args...)
		output, err = command.CombinedOutput()
		if err == nil {
			result.Status = "updated"
			return result, nil
		}
	}
	if err != nil {
		return result, fmt.Errorf("configure %s: %w: %s", client, err, strings.TrimSpace(string(output)))
	}
	result.Status = "configured"
	return result, nil
}

func argumentValue(args []string, name, fallback string) string {
	for index, arg := range args {
		if arg == name && index+1 < len(args) {
			return args[index+1]
		}
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimPrefix(arg, name+"=")
		}
	}
	return fallback
}

func setupCursor(options Options, binary, workspace string) (Result, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Result{}, err
	}
	path := filepath.Join(home, ".cursor", "mcp.json")
	if options.Scope == "project" {
		path = filepath.Join(workspace, ".cursor", "mcp.json")
	}
	result := Result{Client: "cursor", ConfigPath: path, Command: []string{binary, "mcp", "--workspace", workspace}}
	if options.DryRun {
		result.Status = "dry_run"
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return result, err
	}
	root := map[string]any{}
	if data, readErr := os.ReadFile(path); readErr == nil {
		cleaned, cleanErr := stripJSONComments(data)
		if cleanErr != nil {
			return result, fmt.Errorf("parse existing Cursor config: %w", cleanErr)
		}
		if len(bytes.TrimSpace(cleaned)) > 0 {
			if err := json.Unmarshal(cleaned, &root); err != nil {
				return result, fmt.Errorf("parse existing Cursor config: %w", err)
			}
		}
		backup := path + ".backup-" + time.Now().UTC().Format("20060102T150405Z")
		if err := os.WriteFile(backup, data, 0o600); err != nil {
			return result, fmt.Errorf("backup Cursor config: %w", err)
		}
		result.BackupPath = backup
	} else if !os.IsNotExist(readErr) {
		return result, readErr
	}
	servers, _ := root["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["whichrepo"] = map[string]any{
		"command": binary,
		"args":    []string{"mcp", "--workspace", workspace},
	}
	root["mcpServers"] = servers
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return result, err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return result, err
	}
	if err := os.Chmod(path, 0o600); err != nil && runtime.GOOS != "windows" {
		return result, err
	}
	result.Status = "configured"
	return result, nil
}

func stripJSONComments(input []byte) ([]byte, error) {
	output := make([]byte, 0, len(input))
	inString := false
	escaped := false
	for index := 0; index < len(input); index++ {
		current := input[index]
		if inString {
			output = append(output, current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			output = append(output, current)
			continue
		}
		if current == '/' && index+1 < len(input) && input[index+1] == '/' {
			for index < len(input) && input[index] != '\n' {
				index++
			}
			output = append(output, '\n')
			continue
		}
		if current == '/' && index+1 < len(input) && input[index+1] == '*' {
			index += 2
			for index+1 < len(input) && !(input[index] == '*' && input[index+1] == '/') {
				if input[index] == '\n' {
					output = append(output, '\n')
				}
				index++
			}
			if index+1 >= len(input) {
				return nil, fmt.Errorf("unterminated block comment")
			}
			index++
			continue
		}
		output = append(output, current)
	}
	if inString {
		return nil, fmt.Errorf("unterminated JSON string")
	}
	return output, nil
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func PlatformInstallCommand() string {
	if runtime.GOOS == "windows" {
		return "irm https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.ps1 | iex"
	}
	return "curl -fsSL https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.sh | sh"
}
