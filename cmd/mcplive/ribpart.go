// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runRib exercises the RIB feature: an open sketch path (a line) thickened by ±thickness/2 and
// extruded depth along the plane normal into a support wall of volume length·thickness·depth.
func runRib(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "ribLen", "expression": "20 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "ribTh", "expression": "2 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "ribDepth", "expression": "10 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	line := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0}, {2, 0}}})
	if c.err != nil || len(line) < 3 {
		return fmt.Errorf("rib line reply too short (%v)", c.err)
	}
	c.con("ground", line[1])
	c.con("horizontal", line[1], line[2])
	c.dim("distance", "ribLen", line[1], line[2])
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("rib", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "thickness": "ribTh", "depth": "ribDepth", "operation": "new",
	}); err != nil {
		return err
	}
	vol := func(depthMM float64) float64 { return 2.0 * 0.2 * (depthMM / 10) }
	if err := c.checkVolume("depth=10", vol(10)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "ribDepth", "expression": "16 mm"}, nil)
	return c.checkVolume("depth=16 (taller)", vol(16))
}
