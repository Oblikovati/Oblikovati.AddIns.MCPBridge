// SPDX-License-Identifier: GPL-2.0-only

package main

import "math"

// runCornerBracket is the live driver for a flat L corner bracket: a concave L outline drawn
// with the polyline entity, extruded, with a mounting hole through each arm. Shows an L gusset
// in the viewport; the two holes on a concave face exercise the earcut concave+multi-hole fix.
func runCornerBracket(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{
		"sketchIndex": 0, "kind": "polyline", "closed": true,
		"points": [][]float64{{0, 0}, {4, 0}, {4, 1.5}, {1.5, 1.5}, {1.5, 4}, {0, 4}},
	}, nil)
	if c.err != nil {
		return c.err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "5 mm", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	for _, p := range [][]float64{{3, 0.75}, {0.75, 3}} {
		c.ids(map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{p}, "radius": "3 mm"})
	}
	for pi := 0; pi < 2; pi++ {
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": pi, "distance": "10 mm", "operation": "cut"}); err != nil {
			return err
		}
	}
	// L-area·t − two Ø6 holes·t (cm³).
	return c.checkVolume("L corner bracket", (4.0*4.0-2.5*2.5)*0.5-2*math.Pi*0.3*0.3*0.5)
}
