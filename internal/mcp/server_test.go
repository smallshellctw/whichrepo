package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestInitializeAndListTools(t *testing.T) {
	input := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\"}\n")
	var output bytes.Buffer
	server := Server{}
	if err := server.Serve(context.Background(), input, &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "whichrepo") || !strings.Contains(text, "route_task") || !strings.Contains(text, "refresh_index") {
		t.Fatalf("unexpected output: %s", text)
	}
}
