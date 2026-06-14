// SPDX-License-Identifier: GPL-2.0-only

package main

func runRdElectrolytic(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "9.6 mm", "9.6 mm")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "11.5 mm", "operation": "new"}); err != nil {
		return err
	}
	for idx, x := range []float64{-0.125, 0.125} {
		c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
		addConstrainedCircle(c, idx+1, []float64{x, 0}, "0.25 mm", "0.25 mm")
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": idx + 1, "profileIndex": 0, "distance": "3.2 mm", "operation": "join"}); err != nil {
			return err
		}
	}
	return addBoxFeature(c, [][]float64{{0.18, -0.02}, {0.4, 0.02}}, "2.2 mm", "0.4 mm", "0.4 mm", "join")
}
