package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

type RouteFunc func(context.Context, string, int) (any, error)
type RefreshFunc func(context.Context) (any, error)
type ListProjectsFunc func(context.Context) (any, error)
type WorkspaceStatusFunc func(context.Context) (any, error)

type Server struct {
	Route           RouteFunc
	Refresh         RefreshFunc
	ListProjects    ListProjectsFunc
	WorkspaceStatus WorkspaceStatusFunc
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (s Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		resp := s.handle(ctx, req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func (s Server) handle(ctx context.Context, req request) *response {
	switch req.Method {
	case "initialize":
		return success(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "whichrepo", "version": "0.1.0"},
			"instructions":    "Route an engineering task before broad repository exploration. Treat scores as evidence, not authorization. Ask the user when needs_clarification is true.",
		})
	case "notifications/initialized":
		return nil
	case "ping":
		return success(req.ID, map[string]any{})
	case "tools/list":
		return success(req.ID, map[string]any{"tools": toolDefinitions()})
	case "tools/call":
		return s.call(ctx, req)
	default:
		return failure(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s Server) call(ctx context.Context, req request) *response {
	var params callParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return failure(req.ID, -32602, "invalid params: "+err.Error())
	}
	var value any
	var err error
	switch params.Name {
	case "route_task":
		task, _ := params.Arguments["task"].(string)
		if task == "" {
			err = fmt.Errorf("task is required")
			break
		}
		topK := intNumber(params.Arguments["top_k"], 8)
		value, err = s.Route(ctx, task, topK)
	case "refresh_index":
		value, err = s.Refresh(ctx)
	case "list_projects":
		value, err = s.ListProjects(ctx)
	case "workspace_status":
		value, err = s.WorkspaceStatus(ctx)
	default:
		err = fmt.Errorf("unknown tool: %s", params.Name)
	}
	if err != nil {
		return success(req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": "error: " + err.Error()}},
			"isError": true,
		})
	}
	encoded, _ := json.MarshalIndent(value, "", "  ")
	return success(req.ID, map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(encoded)}},
	})
}

func toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "route_task",
			"description": "Route a software task to likely projects and return evidence, ambiguity, change type, and risk signals.",
			"inputSchema": map[string]any{
				"type":     "object",
				"required": []string{"task"},
				"properties": map[string]any{
					"task":  map[string]any{"type": "string", "description": "The original engineering task, issue, or user feedback."},
					"top_k": map[string]any{"type": "integer", "minimum": 1, "maximum": 20, "default": 8},
				},
				"additionalProperties": false,
			},
			"annotations": map[string]any{"readOnlyHint": true},
		},
		{
			"name":        "refresh_index",
			"description": "Rescan the configured workspace and replace the local project search index.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
			"annotations": map[string]any{"readOnlyHint": false},
		},
		{
			"name": "list_projects", "description": "List indexed projects and their metadata.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
			"annotations": map[string]any{"readOnlyHint": true},
		},
		{
			"name": "workspace_status", "description": "Return workspace index and decision-provider status.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
			"annotations": map[string]any{"readOnlyHint": true},
		},
	}
}

func success(id json.RawMessage, result any) *response {
	return &response{JSONRPC: "2.0", ID: rawID(id), Result: result}
}

func failure(id json.RawMessage, code int, message string) *response {
	return &response{JSONRPC: "2.0", ID: rawID(id), Error: &rpcError{Code: code, Message: message}}
}

func rawID(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(id, &value); err != nil {
		return string(id)
	}
	return value
}

func intNumber(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
