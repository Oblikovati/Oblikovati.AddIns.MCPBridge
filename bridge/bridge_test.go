// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeHost records the last call and returns a canned reply or error, standing in
// for the real C-ABI host so the bridge is testable without cgo or a running app.
type fakeHost struct {
	lastMethod string
	lastReq    []byte
	reply      []byte
	err        error
}

func (f *fakeHost) Call(method string, req []byte) ([]byte, error) {
	f.lastMethod, f.lastReq = method, req
	return f.reply, f.err
}

// connect builds the bridge server and an in-memory MCP client session against it.
func connect(t *testing.T, host HostCaller) *mcp.ClientSession {
	t.Helper()
	s, err := NewServer(host)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := s.mcp.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestNewServerNilCaller(t *testing.T) {
	if _, err := NewServer(nil); err == nil {
		t.Fatal("expected error for nil HostCaller")
	}
}

func TestToolsRegistered(t *testing.T) {
	cs := connect(t, &fakeHost{reply: []byte("{}")})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := make([]string, len(res.Tools))
	for i, tool := range res.Tools {
		got[i] = tool.Name
	}
	for _, want := range []string{
		"list_commands", "execute_command", "create_document", "set_parameter",
		"get_model_tree", "list_feature_kinds", "add_feature",
		"list_work_planes", "create_work_plane", "redefine_work_plane", "create_work_point", "list_materials", "assign_material",
		"create_command", "get_ribbon", "get_active_theme",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("tool %q not registered (have %v)", want, got)
		}
	}
}

// TestSummarizedToolRendersDigest checks a summary tool returns a human one-liner (not raw
// JSON) while passing the full reply through as structured content.
func TestSummarizedToolRendersDigest(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"materials":[{"id":"mat.steel","displayName":"Steel"},{"id":"mat.alu","displayName":"Aluminum"}]}`)}
	cs := connect(t, host)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_materials"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := firstText(t, res)
	if !strings.Contains(text, "2 material(s)") || !strings.Contains(text, "Steel [mat.steel]") {
		t.Errorf("digest = %q, want a count + name[id] list", text)
	}
	if res.StructuredContent == nil {
		t.Error("summary tool should still carry the full reply as structured content")
	}
}

func TestCallToolForwardsToHost(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"ok":true}`)}
	cs := connect(t, host)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "execute_command",
		Arguments: map[string]any{"id": "View.Home"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	if host.lastMethod != "commands.execute" {
		t.Fatalf("forwarded method = %q, want commands.execute", host.lastMethod)
	}
	var arg map[string]string
	if err := json.Unmarshal(host.lastReq, &arg); err != nil || arg["id"] != "View.Home" {
		t.Fatalf("forwarded req = %s, want {\"id\":\"View.Home\"}", host.lastReq)
	}
	if text := firstText(t, res); text != `{"ok":true}` {
		t.Fatalf("tool result text = %q, want host reply", text)
	}
}

func TestCallToolHostErrorIsToolError(t *testing.T) {
	host := &fakeHost{err: errors.New("no active document")}
	cs := connect(t, host)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_parameters"})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected a tool error result when the host fails")
	}
}

func TestAddFeatureForwardsKindAndArgs(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"feature":"Extrusion1","kind":"extrude","bodies":1}`)}
	cs := connect(t, host)
	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "add_feature",
		Arguments: map[string]any{
			"kind": "extrude",
			"args": map[string]any{"sketchIndex": 0, "distance": "5 cm"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if host.lastMethod != "features.add" {
		t.Fatalf("method = %q, want features.add", host.lastMethod)
	}
	var got struct {
		Kind string `json:"kind"`
		Args struct {
			SketchIndex int    `json:"sketchIndex"`
			Distance    string `json:"distance"`
		} `json:"args"`
	}
	if err := json.Unmarshal(host.lastReq, &got); err != nil {
		t.Fatalf("unmarshal forwarded req: %v (%s)", err, host.lastReq)
	}
	if got.Kind != "extrude" || got.Args.Distance != "5 cm" || got.Args.SketchIndex != 0 {
		t.Fatalf("forwarded args = %s, want extrude/sketch0/5cm", host.lastReq)
	}
}

// firstText returns the first text-content string of a tool result.
func firstText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	return tc.Text
}

func TestRedefineWorkPlaneForwardsArgs(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"plane":{"index":3,"name":"Work Plane1","healthy":true}}`)}
	cs := connect(t, host)
	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "redefine_work_plane",
		Arguments: map[string]any{
			"index":   3,
			"scalars": []map[string]any{{"index": 0, "value": "50 mm"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if host.lastMethod != "workPlanes.redefine" {
		t.Fatalf("method = %q, want workPlanes.redefine", host.lastMethod)
	}
	var arg map[string]any
	if err := json.Unmarshal(host.lastReq, &arg); err != nil {
		t.Fatalf("forwarded req not JSON: %v", err)
	}
	if arg["index"].(float64) != 3 {
		t.Errorf("forwarded index = %v, want 3", arg["index"])
	}
}
