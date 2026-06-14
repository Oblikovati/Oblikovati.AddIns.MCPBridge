// SPDX-License-Identifier: GPL-2.0-only

package main

func runE3dFan(c *caller) error {
	if err := addLiveE3dDuct(c); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{1.5, -1.5}, {2.5, 1.5}}, "10 mm", "30 mm", "3 mm", "join"); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 5, []float64{2.0, 0}, "11 mm", "11 mm")
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 5, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
}
