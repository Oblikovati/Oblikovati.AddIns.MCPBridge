// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopRodChamferedEnds models a NopSCADlib rod: a cylinder with 45° chamfered
// ends. It extrudes a parametric circle into a cylinder, then chamfers the two end
// rings via the chamfer feature (edge reference keys), exercising the dress-up edge
// path end to end. Volume is checked against the OpenSCAD golden (rod(6,20)).
//
// Reference: NopSCADlib/vitamins/rod.scad (rod = hull of Ø d and Ø d-2·chamfer
// cylinders, chamfer = d/10).
func TestNopRodChamferedEnds(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "d", "expression": "6 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "l", "expression": "20 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	circle := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.3 cm"})
	circleE, center := circle[0], circle[1]
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{center}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{circleE}, "expression": "d / 2"}, nil)

	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "l", "operation": "new"}); !healthy {
		t.Fatalf("cylinder extrude unhealthy: %s", reason)
	}
	cylVol := partVolume(t, cs)

	// Pick the end-ring edges: their midpoints sit on the caps at z≈0 and z≈l (2 cm);
	// side edges sit at z≈1. Chamfer the rings by d/10 = 0.6 mm.
	const lcm = 2.0
	ends := edgesNearZ(t, cs, []float64{0, lcm}, 1e-3)
	if len(ends) == 0 {
		t.Fatal("no end-ring edges found to chamfer")
	}
	if healthy, reason := applyFeature(t, cs, "chamfer",
		map[string]any{"edgeRefs": ends, "distance": "0.6 mm"}); !healthy {
		t.Fatalf("chamfer unhealthy: %s", reason)
	}

	got := partVolume(t, cs)
	if got >= cylVol {
		t.Errorf("chamfer did not remove material: %.5f >= cylinder %.5f", got, cylVol)
	}
	// Golden rod(6,20) = 0.55813 cm^3; allow a faceting band.
	const golden = 0.55813
	if rel := math.Abs(got-golden) / golden; rel > 0.03 {
		t.Errorf("rod volume = %.5f cm^3, want ~%.5f golden (rel %.4f)", got, golden, rel)
	}
}

// edgesNearZ returns the reference keys of edges whose representative point's Z lies
// within tol of any of the given z values — used to select cap-ring edges.
func edgesNearZ(t *testing.T, cs *mcp.ClientSession, zs []float64, tol float64) []string {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Edges []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"edges"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	var keys []string
	for _, b := range rk.Bodies {
		for _, e := range b.Edges {
			if len(e.Point) != 3 {
				continue
			}
			for _, z := range zs {
				if math.Abs(e.Point[2]-z) <= tol {
					keys = append(keys, e.Key)
					break
				}
			}
		}
	}
	return keys
}
