// SPDX-License-Identifier: GPL-2.0-only

package main

func runWireLink(c *caller) error {
	for idx, x := range []float64{-0.6, 0.6} {
		c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
		addConstrainedCircle(c, idx, []float64{x, 0}, "0.6 mm", "0.6 mm")
		op := "join"
		if idx == 0 {
			op = "new"
		}
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": idx, "profileIndex": 0, "distance": "12 mm", "operation": op}); err != nil {
			return err
		}
	}
	c.json("create_sketch", map[string]any{"plane": "YZ"}, nil)
	addConstrainedCircle(c, 2, []float64{0, 0.9}, "0.6 mm", "0.6 mm")
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 2, "profileIndex": 0, "distance": "12 mm", "operation": "join", "direction": "symmetric"})
}
