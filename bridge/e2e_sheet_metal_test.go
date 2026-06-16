// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sheetMetalPart creates a sheet-metal part, lays a 40×30 mm base Face, and returns the
// session — the fixture the wall/bend e2e checks fold from. The base Face proves the
// environment entry (the subtype seeds the rule) and the rule-driven thickening.
func sheetMetalPart(t *testing.T) *mcp.ClientSession {
	t.Helper()
	cs := freshPart(t)
	var doc struct {
		ID uint64 `json:"id"`
	}
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "panel.obk", "subType": "org.oblikovati.part.sheetMetal"}, &doc)
	callJSON(t, cs, "activate_document", map[string]any{"id": doc.ID}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	if healthy, reason := applyFeature(t, cs, "sheetMetalFace", map[string]any{"sketchIndex": 0}); !healthy {
		t.Fatalf("base Face unhealthy: %s", reason)
	}
	return cs
}

// addLineSketch adds a sketch on plane with one line through the two points, returning its
// index — used for bend/fold bend lines and contour-flange profiles.
func addLineSketch(t *testing.T, cs *mcp.ClientSession, plane string, pts ...[]float64) int {
	t.Helper()
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": plane}, &sk)
	for i := 0; i+1 < len(pts); i++ {
		callJSON(t, cs, "add_sketch_entity", map[string]any{
			"sketchIndex": sk.SketchIndex, "kind": "line", "points": [][]float64{pts[i], pts[i+1]},
		}, nil)
	}
	return sk.SketchIndex
}

// TestE2ESheetMetalStyle drives the F01 rule surface: a part created sheet-metal reports its
// rule, edits it, and previews a bend allowance — all over MCP.
func TestE2ESheetMetalStyle(t *testing.T) {
	cs := sheetMetalPart(t)
	var style struct {
		Style struct {
			UnfoldMethod string  `json:"unfoldMethod"`
			KFactor      float64 `json:"kFactor"`
		} `json:"style"`
	}
	callJSON(t, cs, "get_sheet_metal_style", nil, &style)
	if style.Style.UnfoldMethod != "kFactor" {
		t.Errorf("default unfold method = %q, want kFactor", style.Style.UnfoldMethod)
	}
	callJSON(t, cs, "set_sheet_metal_style", map[string]any{"thickness": "2 mm", "kFactor": 0.5}, &style)
	if style.Style.KFactor != 0.5 {
		t.Errorf("K-factor after setStyle = %v, want 0.5", style.Style.KFactor)
	}
	var ba struct {
		BendAllowance float64 `json:"bendAllowance"`
	}
	callJSON(t, cs, "sheet_metal_bend_allowance", map[string]any{"angle": "90 deg", "radius": "2 mm"}, &ba)
	if ba.BendAllowance <= 0 {
		t.Errorf("bend allowance = %v, want positive", ba.BendAllowance)
	}
}

// TestE2ESheetMetalFeatures deep-tests the M13-F02 wall/bend/corner features over the bridge:
// each builds a healthy result on a fresh sheet-metal part. (The base Face is exercised by the
// sheetMetalPart fixture; the contour roll's centerline-axis setup is not expressible over the
// current sketch wire and is deep-tested in the source model/opregistry suites.)
func TestE2ESheetMetalFeatures(t *testing.T) {
	// Edge-driven walls take a single `edge` key.
	for _, c := range []struct {
		name, kind string
		extra      map[string]any
	}{
		{"flange", "sheetMetalFlange", map[string]any{"height": "10 mm"}},
		{"hem", "sheetMetalHem", map[string]any{"length": "6 mm"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			cs := sheetMetalPart(t)
			edges, _ := topology(t, cs)
			args := map[string]any{"edge": edges[0]}
			for k, v := range c.extra {
				args[k] = v
			}
			if healthy, reason := applyFeature(t, cs, c.kind, args); !healthy {
				t.Fatalf("%s unhealthy: %s", c.kind, reason)
			}
		})
	}

	// Corner-driven features take an `edges` array.
	for _, c := range []struct {
		name, kind string
		extra      map[string]any
	}{
		{"corner", "sheetMetalCorner", map[string]any{"treatment": "chamfer", "size": "3 mm"}},
		{"cornerSeam", "sheetMetalCornerSeam", map[string]any{"gap": "2 mm"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			cs := sheetMetalPart(t)
			edges, _ := topology(t, cs)
			args := map[string]any{"edges": []string{edges[0]}}
			for k, v := range c.extra {
				args[k] = v
			}
			if healthy, reason := applyFeature(t, cs, c.kind, args); !healthy {
				t.Fatalf("%s unhealthy: %s", c.kind, reason)
			}
		})
	}

	// Bend-line features fold along a sketch line across the sheet.
	for _, c := range []struct {
		name, kind string
		extra      map[string]any
	}{
		{"bend", "sheetMetalBend", nil},
		{"fold", "sheetMetalFold", map[string]any{"location": "centerline"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			cs := sheetMetalPart(t)
			// The base Face is a 4×3 cm sheet (40×30 mm) at the origin; fold across its middle.
			line := addLineSketch(t, cs, "XY", []float64{2, 0}, []float64{2, 3})
			args := map[string]any{"sketchIndex": line, "lineIndex": 0, "radius": "2 mm"}
			for k, v := range c.extra {
				args[k] = v
			}
			if healthy, reason := applyFeature(t, cs, c.kind, args); !healthy {
				t.Fatalf("%s unhealthy: %s", c.kind, reason)
			}
		})
	}

	// NOTE: the profile-driven walls (sheetMetalContourFlange, sheetMetalLoftedFlange,
	// sheetMetalContourRoll) need a multi-segment OPEN profile, but add_sketch_entity creates a
	// fresh point per call, so the chain's vertices are not shared and the profile resolver
	// (which currently walks by point identity) cannot connect them over the wire. They are
	// deep-tested in the source model/opregistry suites; a follow-up makes the resolver match
	// by point position so they can be driven over MCP too.
}

// TestE2ESheetMetalUnfold drives the M13-F04 flat pattern over the bridge: flange a base
// sheet, then sheet_metal_unfold reports the developed flat — a positive gauge/area, extents
// that exceed the folded footprint (the developed tab), and one fold line.
func TestE2ESheetMetalUnfold(t *testing.T) {
	cs := sheetMetalPart(t) // 40×30 mm base sheet
	edges, _ := topology(t, cs)
	if healthy, reason := applyFeature(t, cs, "sheetMetalFlange", map[string]any{"edge": edges[0], "height": "10 mm"}); !healthy {
		t.Fatalf("flange unhealthy: %s", reason)
	}

	// types.Point2d marshals as a [x, y] array.
	var flat struct {
		Flat struct {
			Extents struct {
				Min [2]float64 `json:"min"`
				Max [2]float64 `json:"max"`
			} `json:"extents"`
			Thickness float64 `json:"thickness"`
			Area      float64 `json:"area"`
			Bends     []struct {
				Angle float64 `json:"angle"`
			} `json:"bends"`
		} `json:"flat"`
	}
	callJSON(t, cs, "sheet_metal_unfold", nil, &flat)

	if flat.Flat.Thickness <= 0 || flat.Flat.Area <= 0 {
		t.Errorf("flat gauge/area must be positive: %+v", flat.Flat)
	}
	w := flat.Flat.Extents.Max[0] - flat.Flat.Extents.Min[0]
	h := flat.Flat.Extents.Max[1] - flat.Flat.Extents.Min[1]
	if maxf(w, h) < 4.1 { // base is 4×3 cm; the developed tab pushes one extent past 4 cm
		t.Errorf("flat extents = %.3f × %.3f cm, want a developed tab beyond the 4×3 base", w, h)
	}
	if len(flat.Flat.Bends) != 1 {
		t.Errorf("flat fold lines = %d, want 1", len(flat.Flat.Bends))
	}
}

// maxf returns the larger of two floats.
func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
