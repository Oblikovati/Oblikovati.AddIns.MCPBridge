// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runCounterbore is the live driver for a counterbored cap-screw mount — the stepped screw
// recess pervasive in NopSCADlib printed parts. Exercises the hole feature's COUNTERBORE path
// (brep.CutCounterboreHole) end to end: a wide recess over a narrow bore, in one drill.
func runCounterbore(c *caller) error {
	for _, p := range [][2]string{{"L", "40 mm"}, {"W", "30 mm"}, {"t", "12 mm"},
		{"boreDia", "6 mm"}, {"boreDepth", "9 mm"}, {"cDia", "11 mm"}, {"cDepth", "4 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	// Plate: L×W rectangle (corner at origin) extruded t.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	r := c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0, 0}, {4, 3}}})
	if len(r) < 5 {
		return fmt.Errorf("rectangle reply too short: %v", r)
	}
	bl, br, tr, tl := r[1], r[2], r[3], r[4]
	c.con("ground", bl)
	c.con("horizontal", bl, br)
	c.con("vertical", bl, tl)
	c.con("horizontal", tl, tr)
	c.con("vertical", br, tr)
	c.dim("distance", "L", bl, br)
	c.dim("distance", "W", bl, tl)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": c.closedProfile(), "distance": "t", "operation": "new"}); err != nil {
		return err
	}

	// Counterbore at the top-face centroid: 11×4 recess over a Ø6 bore, 9 deep (blind).
	if err := c.applyFeature("hole", map[string]any{
		"faceRef": c.topFaceKey(), "type": "counterbore",
		"diameter": "boreDia", "depth": "boreDepth", "counterDiameter": "cDia", "counterDepth": "cDepth",
	}); err != nil {
		return err
	}

	if err := c.checkVolume("cDepth=4", counterboreVol(4)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "cDepth", "expression": "6 mm"}, nil)
	return c.checkVolume("cDepth=6 (deeper)", counterboreVol(6))
}

// counterboreVol = plate − (recess + bore-below-recess), cm^3 (cDepth in mm).
func counterboreVol(cDepthMM float64) float64 {
	const L, W, t, r, cr, depth = 4.0, 3.0, 1.2, 0.3, 0.55, 0.9
	cd := cDepthMM / 10
	return L*W*t - (math.Pi*cr*cr*cd + math.Pi*r*r*(depth-cd))
}
