// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type refKeys struct {
	Bodies []struct {
		Faces []struct {
			Key   string    `json:"key"`
			Point []float64 `json:"point"`
			Kind  string    `json:"kind"`
		} `json:"faces"`
		Edges []struct {
			Key   string    `json:"key"`
			Point []float64 `json:"point"`
		} `json:"edges"`
	} `json:"bodies"`
}

func dist(a []float64, b [3]float64) float64 {
	if len(a) != 3 {
		return math.Inf(1)
	}
	dx, dy, dz := a[0]-b[0], a[1]-b[1], a[2]-b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// TestEndToEndRefilletArc reproduces mcprefillet case 2 in-process: extrude a 4x3x2 box, fillet a
// vertical edge (a quarter-cylinder with sharp arc caps), then fillet the sharp top arc cap — which
// must now round into a torus + setback end-caps via FilletCylinderArc, not fail the miter path.
func TestEndToEndRefilletArc(t *testing.T) {
	cs := boxWithVerticalFillet(t)

	// Refillet the sharp top arc cap edge (cylinder ∩ top plane) at ~(3.85,2.85,2): must round into a
	// torus + setback end-caps on the analytic body, not get re-faceted into the failing miter path.
	var rk refKeys
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	arc := nearestEdgeKey(t, rk, [3]float64{3.85, 2.85, 2})
	var f2 struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "fillet", "args": map[string]any{"edgeRefs": []string{arc}, "radius": "1 mm"},
	}, &f2)
	if !f2.Healthy {
		t.Fatalf("arc-cap refillet unhealthy: %s", f2.Reason)
	}

	// The result must carry a torus face (the arc blend) and be a valid solid.
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if tor := countFaceKind(rk, "torus"); tor != 1 {
		t.Errorf("torus faces = %d, want 1", tor)
	}
}

// TestEndToEndRefilletSmoothLineRejected refillets the G1-smooth tangent line where the fillet
// cylinder runs into the side plane (~(4,2.7,1)). It has no corner to round, so the feature must
// reject it cleanly as "smooth" on the analytic body — not re-facet it into the misleading
// "invalid solid" the planar path produced before the curved-adjacent route.
func TestEndToEndRefilletSmoothLineRejected(t *testing.T) {
	cs := boxWithVerticalFillet(t)
	var rk refKeys
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	line := nearestEdgeKey(t, rk, [3]float64{4, 2.7, 1})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "add_feature", Arguments: map[string]any{
			"kind": "fillet", "args": map[string]any{"edgeRefs": []string{line}, "radius": "1 mm"},
		}})
	if err != nil {
		t.Fatalf("add_feature: %v", err)
	}
	var f struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	if e := json.Unmarshal([]byte(firstText(t, res)), &f); e != nil {
		t.Fatalf("decode: %v", e)
	}
	if f.Healthy || !strings.Contains(f.Reason, "smooth") {
		t.Fatalf("smooth tangent line should be rejected as smooth, got healthy=%v reason=%q", f.Healthy, f.Reason)
	}
}

// boxWithVerticalFillet builds the seeded 4x3 profile extruded to a 2cm box with its x=4,y=3 vertical
// edge rounded (r=3mm), leaving a quarter-cylinder with sharp arc caps + smooth tangent lines.
func boxWithVerticalFillet(t *testing.T) *mcp.ClientSession {
	t.Helper()
	cs := e2eClient(t, seededSession(t))
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "2 cm"},
	}, nil)
	var rk refKeys
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	vert := nearestEdgeKey(t, rk, [3]float64{4, 3, 1})
	var f1 struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "fillet", "args": map[string]any{"edgeRefs": []string{vert}, "radius": "3 mm"},
	}, &f1)
	if !f1.Healthy {
		t.Fatalf("first fillet unhealthy: %s", f1.Reason)
	}
	return cs
}

func countFaceKind(rk refKeys, kind string) int {
	n := 0
	if len(rk.Bodies) > 0 {
		for _, f := range rk.Bodies[0].Faces {
			if f.Kind == kind {
				n++
			}
		}
	}
	return n
}

func nearestEdgeKey(t *testing.T, rk refKeys, p [3]float64) string {
	t.Helper()
	if len(rk.Bodies) == 0 {
		t.Fatal("no bodies")
	}
	best, bestD := "", math.Inf(1)
	for _, e := range rk.Bodies[0].Edges {
		if d := dist(e.Point, p); d < bestD {
			best, bestD = e.Key, d
		}
	}
	if best == "" {
		t.Fatalf("no edge near %v", p)
	}
	return best
}
