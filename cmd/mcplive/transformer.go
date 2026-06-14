// SPDX-License-Identifier: GPL-2.0-only

package main

func runTransformer(c *caller) error {
	if err := addBoxFeature(c, [][]float64{{-2, -1.5}, {2, 1.5}}, "40 mm", "30 mm", "2 mm", "new"); err != nil {
		return err
	}
	idx := 1
	for _, x := range []float64{-1.5, 1.5} {
		for _, y := range []float64{-1.0, 1.0} {
			c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
			addConstrainedCircle(c, idx, []float64{x, y}, "1.6 mm", "1.6 mm")
			if err := c.applyFeature("extrude", map[string]any{"sketchIndex": idx, "profileIndex": 0, "operation": "cut", "extent": "through-all"}); err != nil {
				return err
			}
			idx++
		}
	}
	if err := addBoxFeature(c, [][]float64{{-1.6, -0.6}, {1.6, 0.6}}, "32 mm", "12 mm", "18 mm", "join"); err != nil {
		return err
	}
	return addBoxFeature(c, [][]float64{{-1, -1.1}, {1, 1.1}}, "20 mm", "22 mm", "11 mm", "join")
}
