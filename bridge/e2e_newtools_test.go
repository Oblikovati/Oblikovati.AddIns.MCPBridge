// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"

	"oblikovati.org/app"
)

// callText calls a tool and returns its text content (for tools that return a human summary
// rather than raw JSON).
func callText(t *testing.T, cs *mcp.ClientSession, name string) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s tool error: %s", name, firstText(t, res))
	}
	return firstText(t, res)
}

// TestEndToEndRibbonReflectsDocument: get_ribbon returns the Part ribbon for the seeded part,
// with its modeling tabs — the discovery surface an add-in reads.
func TestEndToEndRibbonReflectsDocument(t *testing.T) {
	cs := e2eClient(t, seededSession(t))
	var r wire.ListRibbonResult
	callJSON(t, cs, "get_ribbon", nil, &r)
	if r.Key != types.PartRibbon {
		t.Fatalf("ribbon key = %q, want Part", r.Key)
	}
	found := false
	for _, tab := range r.Tabs {
		if tab.Name == "Create & Modify" {
			found = true
		}
	}
	if !found {
		t.Errorf("Part ribbon over MCP has no Create & Modify tab: %+v", r.Tabs)
	}
}

// TestEndToEndRibbonAuthoring drives the add-in flow: with no document open the ribbon is
// ZeroDoc; create_command places a button on it, and get_ribbon then reports it.
func TestEndToEndRibbonAuthoring(t *testing.T) {
	cs := e2eClient(t, app.NewSession())
	callJSON(t, cs, "create_command", map[string]any{
		"id": "acme.start", "displayName": "Acme Start",
		"ribbon": "ZeroDoc", "tab": "Get Started", "category": "Acme",
	}, nil)

	var r wire.ListRibbonResult
	callJSON(t, cs, "get_ribbon", nil, &r)
	if r.Key != types.ZeroDocRibbon {
		t.Fatalf("ribbon key = %q, want ZeroDoc", r.Key)
	}
	for _, tab := range r.Tabs {
		for _, p := range tab.Panels {
			for _, ctl := range p.Controls {
				if ctl.CommandID == "acme.start" {
					return // the add-in's button is on the ZeroDoc ribbon
				}
			}
		}
	}
	t.Fatalf("acme.start not found on the ZeroDoc ribbon: %+v", r)
}

// TestEndToEndWorkPlanesAndMaterials: the newly forwarded list tools reach the live model —
// a seeded part exposes its origin work planes and the document's material/appearance sets.
func TestEndToEndWorkPlanesAndMaterials(t *testing.T) {
	cs := e2eClient(t, seededSession(t))

	var wp wire.ListWorkPlanesResult
	callJSON(t, cs, "list_work_planes", nil, &wp)
	if len(wp.Planes) == 0 {
		t.Error("list_work_planes returned none; a part has origin planes")
	}

	// list_materials / list_appearances return a human digest (not raw JSON); the round-trip
	// through the router is what proves the forwarding + summary path.
	if txt := callText(t, cs, "list_materials"); !strings.Contains(strings.ToLower(txt), "material") {
		t.Errorf("list_materials summary = %q, want a material digest", txt)
	}
	if txt := callText(t, cs, "list_appearances"); !strings.Contains(strings.ToLower(txt), "appearance") {
		t.Errorf("list_appearances summary = %q, want an appearance digest", txt)
	}
}

// TestEndToEndThemeSummary: get_active_theme returns a one-line digest, not raw JSON.
func TestEndToEndThemeSummary(t *testing.T) {
	cs := e2eClient(t, seededSession(t))
	if txt := callText(t, cs, "get_active_theme"); !strings.Contains(txt, "Active theme:") {
		t.Errorf("get_active_theme = %q, want an 'Active theme:' digest", txt)
	}
}
