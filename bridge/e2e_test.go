// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/addin/opregistry"
	"oblikovati.org/addin/router"
	"oblikovati.org/app"
	"oblikovati.org/math"
	"oblikovati.org/model/compdef"
	"oblikovati.org/model/doc"
	"oblikovati.org/model/sketch"
)

// routerHost is a HostCaller backed by the real router over a live session — the
// same code path the C-ABI host runs, minus the cgo/dispatcher hop. It lets the test
// drive the whole logical stack (MCP client -> tools -> router -> model) in-process.
type routerHost struct {
	r *router.Router
	s *app.Session
}

func (h routerHost) Call(method string, req []byte) ([]byte, error) {
	return h.r.Handle(h.s, method, req)
}

// seededSession builds a session with an active part that has a "width" parameter
// and one sketch holding a closed 4x3 rectangle profile (as the head seeds).
func seededSession(t *testing.T) *app.Session {
	t.Helper()
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	def := compdef.NewPartComponentDefinition()
	d, err := s.Workspace().Add(doc.Part, "e2e.obk", true)
	if err != nil {
		t.Fatalf("add document: %v", err)
	}
	d.SetContent(def)
	if _, err := def.Parameters().AddUserParameter("width", "4 cm"); err != nil {
		t.Fatalf("add parameter: %v", err)
	}
	sk := def.Sketches().Add(sketch.XYPlane())
	c0 := sk.Points().Add(math.P2(0, 0))
	c1 := sk.Points().Add(math.P2(4, 0))
	c2 := sk.Points().Add(math.P2(4, 3))
	c3 := sk.Points().Add(math.P2(0, 3))
	sk.Lines().Add(c0, c1)
	sk.Lines().Add(c1, c2)
	sk.Lines().Add(c2, c3)
	sk.Lines().Add(c3, c0)
	def.Recompute()
	return s
}

// e2eClient connects an in-memory MCP client to a bridge server whose HostCaller is
// the real router over the seeded session.
func e2eClient(t *testing.T, s *app.Session) *mcp.ClientSession {
	t.Helper()
	srv, err := NewServer(routerHost{r: router.New(opregistry.Default()), s: s})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.mcp.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// callJSON calls a tool and decodes its text result into v.
func callJSON(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, v any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s tool error: %s", name, firstText(t, res))
	}
	if v != nil {
		if err := json.Unmarshal([]byte(firstText(t, res)), v); err != nil {
			t.Fatalf("%s: decode %q: %v", name, firstText(t, res), err)
		}
	}
}

// TestEndToEndBuildSolid drives the full stack: an MCP client extrudes the seeded
// profile into a solid and reads the model tree back — proving tools -> router ->
// live model end to end.
func TestEndToEndBuildSolid(t *testing.T) {
	cs := e2eClient(t, seededSession(t))

	var kinds struct {
		Kinds []struct {
			Kind string `json:"kind"`
		} `json:"kinds"`
	}
	callJSON(t, cs, "list_feature_kinds", nil, &kinds)
	if len(kinds.Kinds) == 0 || kinds.Kinds[0].Kind != "extrude" {
		t.Fatalf("list_feature_kinds = %+v, want extrude", kinds.Kinds)
	}

	var added struct {
		Kind   string `json:"kind"`
		Bodies int    `json:"bodies"`
	}
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "5 cm"},
	}, &added)
	if added.Kind != "extrude" || added.Bodies != 1 {
		t.Fatalf("add_feature result = %+v, want extrude/1 body", added)
	}

	var tree struct {
		Sketches int `json:"sketches"`
		Bodies   int `json:"bodies"`
		Features []struct {
			Kind string `json:"kind"`
		} `json:"features"`
	}
	callJSON(t, cs, "get_model_tree", nil, &tree)
	if tree.Sketches != 1 || tree.Bodies != 1 || len(tree.Features) != 1 || tree.Features[0].Kind != "extrude" {
		t.Fatalf("get_model_tree = %+v, want 1 sketch / 1 body / 1 extrude", tree)
	}

	// Drive a parameter and confirm it round-trips.
	var set struct {
		Expression string `json:"expression"`
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "width", "expression": "6 cm"}, &set)
	if set.Expression != "6 cm" {
		t.Fatalf("set_parameter width = %q, want 6 cm", set.Expression)
	}
}

// TestEndToEndSketchFromScratch builds a solid from an empty part entirely over MCP:
// create a part, create a sketch, draw a rectangle, extrude it.
func TestEndToEndSketchFromScratch(t *testing.T) {
	cs := e2eClient(t, app.NewSession())

	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "scratch.obk"}, nil)

	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, &sk)

	var rect struct {
		Profiles int `json:"profiles"`
	}
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": sk.SketchIndex, "width": "40 mm", "height": "30 mm"}, &rect)
	if rect.Profiles != 1 {
		t.Fatalf("sketch_rectangle profiles = %d, want 1", rect.Profiles)
	}

	var ext struct {
		Bodies int `json:"bodies"`
	}
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude",
		"args": map[string]any{"sketchIndex": sk.SketchIndex, "profileIndex": 0, "distance": "50 mm"},
	}, &ext)
	if ext.Bodies != 1 {
		t.Fatalf("extrude over MCP from scratch = %d bodies, want 1", ext.Bodies)
	}
}

// TestEndToEndDocuments drives document creation/listing over MCP.
func TestEndToEndDocuments(t *testing.T) {
	cs := e2eClient(t, seededSession(t))
	var created struct {
		Active bool   `json:"active"`
		Type   string `json:"type"`
	}
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "second.obk"}, &created)
	if !created.Active || created.Type != "part" {
		t.Fatalf("create_document = %+v, want active part", created)
	}
	var list struct {
		Documents []json.RawMessage `json:"documents"`
	}
	callJSON(t, cs, "list_documents", nil, &list)
	if len(list.Documents) != 2 {
		t.Fatalf("list_documents = %d, want 2", len(list.Documents))
	}
}
