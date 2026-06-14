// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/addin/opregistry"
	"oblikovati.org/addin/router"
)

// TestHTTPTransportEndToEnd exercises the real streamable HTTP/SSE transport (not the
// in-memory one): start the server on an ephemeral port, connect a streamable HTTP
// client, and build a solid via add_feature — the same path a remote LLM takes,
// minus the GUI host (which needs a display/Vulkan unavailable in CI).
func TestHTTPTransportEndToEnd(t *testing.T) {
	s, err := NewServer(routerHost{r: router.New(opregistry.Default()), s: seededSession(t)})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := s.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "http-e2e", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://" + s.Addr() + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			t.Fatalf("close client session: %v", closeErr)
		}
	}()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "add_feature",
		Arguments: map[string]any{"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "distance": "5 cm"}},
	})
	if err != nil {
		t.Fatalf("CallTool over HTTP: %v", err)
	}
	if res.IsError {
		t.Fatalf("add_feature tool error: %s", firstText(t, res))
	}
	var added struct {
		Bodies int `json:"bodies"`
	}
	if err := json.Unmarshal([]byte(firstText(t, res)), &added); err != nil || added.Bodies != 1 {
		t.Fatalf("add_feature over HTTP = %s (err %v), want 1 body", firstText(t, res), err)
	}
}
