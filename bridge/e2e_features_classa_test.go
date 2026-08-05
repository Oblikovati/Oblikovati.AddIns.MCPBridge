// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// End-to-end validation of the M36 Class-A surfacing operations — fillSurface, bridgeSurface,
// networkSurface, fairSurface and fitSurface. Every one of them was reachable over add_feature but
// untested here, which is exactly what TestE2EFeatureRegistryCoverage exists to catch: it had been
// failing with "registry has 74 kinds; update this suite to deep-test the new one(s)" because the
// registry advertised 74 while this suite covered 69.
//
// networkSurface and fitSurface carry their own input (a curve grid / a point cloud), so they
// assert HEALTHY and that a surface body actually appeared. fairSurface runs on the surface a
// networkSurface just built, so it asserts healthy too. fillSurface and bridgeSurface consume "the
// last N surface bodies" forming an opening, which the wire cannot cheaply guarantee, so they
// follow this file's established convention and assert the kernel path is reached without a panic.

// surfaceBodyCount returns how many bodies the active part holds. A surfacing feature that
// produced nothing leaves this unchanged, so it is the check that separates "ran" from "built".
func surfaceBodyCount(t *testing.T, cs *mcp.ClientSession) int {
	t.Helper()
	var r struct {
		Bodies []struct {
			Name string `json:"name"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "body_list", nil, &r)
	return len(r.Bodies)
}

// gridCurves returns the U and V curve grids of a gently domed 3×3 patch: three lines of constant
// v and three of constant u over [0,4]², lifted by z = 1 − ((x−2)² + (y−2)²)/16 so the network has
// real curvature to interpolate rather than degenerating to a plane.
func gridCurves() (uCurves, vCurves [][][]float64) {
	at := func(x, y float64) []float64 {
		dx, dy := x-2, y-2
		return []float64{x, y, 1 - (dx*dx+dy*dy)/16}
	}
	steps := []float64{0, 2, 4}
	for _, y := range steps {
		row := make([][]float64, 0, len(steps))
		for _, x := range steps {
			row = append(row, at(x, y))
		}
		uCurves = append(uCurves, row)
	}
	for _, x := range steps {
		col := make([][]float64, 0, len(steps))
		for _, y := range steps {
			col = append(col, at(x, y))
		}
		vCurves = append(vCurves, col)
	}
	return uCurves, vCurves
}

func TestE2ENetworkSurface(t *testing.T) {
	cs := freshPart(t)
	before := surfaceBodyCount(t, cs)
	u, v := gridCurves()
	if healthy, reason := applyFeature(t, cs, "networkSurface", map[string]any{"uCurves": u, "vCurves": v}); !healthy {
		t.Fatalf("networkSurface unhealthy: %s", reason)
	}
	if after := surfaceBodyCount(t, cs); after <= before {
		t.Errorf("networkSurface produced no body: bodies %d -> %d", before, after)
	}
}

// TestE2EFairSurfaceOnANetwork chains the two: fairing needs a running surface, and the network
// surface is one this suite can build outright — so this covers fairSurface for real rather than
// asserting it merely reached the kernel.
func TestE2EFairSurfaceOnANetwork(t *testing.T) {
	cs := freshPart(t)
	u, v := gridCurves()
	if healthy, reason := applyFeature(t, cs, "networkSurface", map[string]any{"uCurves": u, "vCurves": v}); !healthy {
		t.Fatalf("networkSurface setup unhealthy: %s", reason)
	}
	if healthy, reason := applyFeature(t, cs, "fairSurface", map[string]any{
		"continuity": "g2", "strength": 0.5, "iterations": 5,
	}); !healthy {
		t.Fatalf("fairSurface unhealthy: %s", reason)
	}
	if n := surfaceBodyCount(t, cs); n == 0 {
		t.Error("fairSurface left the part with no body")
	}
}

// writeScanFile writes a .xyz point cloud sampling the same dome as [gridCurves] and returns its
// path. A real file is the only way in: point_clouds_attach takes a filename, since a scan is a
// REFERENCED asset rather than embedded document data (#645).
func writeScanFile(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for i := 0; i <= 12; i++ {
		for j := 0; j <= 12; j++ {
			x, y := float64(i)/3, float64(j)/3
			dx, dy := x-2, y-2
			fmt.Fprintf(&b, "%.6f %.6f %.6f\n", x, y, 1-(dx*dx+dy*dy)/16)
		}
	}
	path := filepath.Join(t.TempDir(), "dome.xyz")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write scan: %v", err)
	}
	return path
}

func TestE2EFitSurfaceToAScan(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "point_clouds_attach", map[string]any{"name": "Scan", "fullFileName": writeScanFile(t)}, nil)
	before := surfaceBodyCount(t, cs)
	if healthy, reason := applyFeature(t, cs, "fitSurface", map[string]any{
		"cloud": "Scan", "degree": 3, "nu": 5, "nv": 5,
	}); !healthy {
		t.Fatalf("fitSurface unhealthy: %s", reason)
	}
	if after := surfaceBodyCount(t, cs); after <= before {
		t.Errorf("fitSurface produced no body: bodies %d -> %d", before, after)
	}
}

// TestE2EFillAndBridgeSurfaceReach covers the two operations that consume "the last N surface
// bodies" bounding an opening. Standing up a genuine N-sided opening over the sketch wire is not
// something this harness can guarantee, so — as with ruledSurface/stitch/sculpt above — each
// asserts the resolver and kernel path run end to end without a panic.
func TestE2EFillAndBridgeSurfaceReach(t *testing.T) {
	t.Run("fillSurface", func(t *testing.T) {
		cs := freshPart(t)
		u, v := gridCurves()
		callJSON(t, cs, "add_feature", map[string]any{"kind": "networkSurface",
			"args": map[string]any{"uCurves": u, "vCurves": v}}, nil)
		mustReachFeature(t, cs, "fillSurface", map[string]any{"continuity": "g2", "sides": 4})
	})
	t.Run("bridgeSurface", func(t *testing.T) {
		cs := freshPart(t)
		// Two boundary patches on parallel planes are two surface bodies to bridge between.
		s0 := profileSketch(t, cs, "XY", "40 mm", "30 mm")
		callJSON(t, cs, "add_feature", map[string]any{"kind": "boundaryPatch",
			"args": map[string]any{"sketchIndex": s0, "profileIndex": 0}}, nil)
		var wp struct {
			Index int `json:"index"`
		}
		callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "20 mm"}, &wp)
		var sk struct {
			SketchIndex int `json:"sketchIndex"`
		}
		callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
		callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": sk.SketchIndex, "width": "40 mm", "height": "30 mm"}, nil)
		callJSON(t, cs, "add_feature", map[string]any{"kind": "boundaryPatch",
			"args": map[string]any{"sketchIndex": sk.SketchIndex, "profileIndex": 0}}, nil)
		mustReachFeature(t, cs, "bridgeSurface", map[string]any{"continuityA": "g2", "continuityB": "g2"})
	})
}
