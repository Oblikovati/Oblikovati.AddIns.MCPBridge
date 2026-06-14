// SPDX-License-Identifier: GPL-2.0-only

package main

func runMainsSocketHoles(c *caller) error {
	if err := addBoxFeature(c, [][]float64{{-1.8, -1.2}, {1.8, 1.2}}, "36 mm", "24 mm", "1.2 mm", "new"); err != nil {
		return err
	}
	for idx, x := range []float64{-1.25, 1.25} {
		c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
		addConstrainedCircle(c, idx+1, []float64{x, 0}, "1.6 mm", "1.6 mm")
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": idx + 1, "profileIndex": 0, "operation": "cut", "extent": "through-all"}); err != nil {
			return err
		}
	}
	if err := addLiveSlotCut(c, []float64{-0.45, 0}, []float64{0.45, 0}, 0.45); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 4, []float64{-1.25, -0.75}, "2.2 mm", "2.2 mm")
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 4, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
}
