// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runDIP is the live driver for the NopSCADlib DIP IC package body (vitamins/dip.scad). The
// chamfered hull octagon (the parting-line "wings") is extruded the package length, a central
// slot is CUT through-all (splitting the bar into two wings), then two overlapping blocks are
// JOIN-extruded back in along walls coplanar with the wing inner faces, re-welding the two
// wings into one solid. Shows the colliding cut-split + coplanar-reweld stack in the viewport.
// (The pin-1 index notch is omitted — its blind boundary-straddle cut trips a planar-arrangement
// watertightness bug; see kernel/brep TestBlindStraddleCurvedCutWatertight.)
func runDIP(c *caller) error {
	octagon := [][]float64{
		{-0.2675, 0.15}, {-0.3175, 0.0125}, {-0.3175, -0.0125}, {-0.2675, -0.15},
		{0.2675, -0.15}, {0.3175, -0.0125}, {0.3175, 0.0125}, {0.2675, 0.15},
	}
	const length = "8.89 mm"

	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": octagon}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": length, "operation": "new"}); err != nil {
		return err
	}

	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {0.2, 0.2}}}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": length, "operation": "cut"}); err != nil {
		return err
	}

	// "center" 2nd point is an absolute corner ⇒ these land on y[−0.15,0.09] and y[0,0.15].
	for i, rect := range [][2][2]float64{{{0, -0.03}, {0.2, 0.09}}, {{0, 0.075}, {0.2, 0.15}}} {
		si := 2 + i
		c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
		c.json("add_sketch_entity", map[string]any{
			"sketchIndex": si, "kind": "rectangle", "variant": "center",
			"points": [][]float64{{rect[0][0], rect[0][1]}, {rect[1][0], rect[1][1]}},
		}, nil)
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": si, "profileIndex": 0, "distance": length, "operation": "join"}); err != nil {
			return fmt.Errorf("refill %d: %w", i, err)
		}
	}

	if err := c.checkVolume("dip body", 0.17675*0.889); err != nil {
		return err
	}

	c.json("execute_command", map[string]any{"id": "View.Home"}, nil)
	c.json("set_normal_debug", map[string]any{"on": true}, nil)
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-capture.png"}, nil)
	return c.err
}
