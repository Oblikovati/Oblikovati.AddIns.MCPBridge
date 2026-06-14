// SPDX-License-Identifier: GPL-2.0-only

package main

func runRoundedCorner(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "cornerT", "expression": "2 mm"}, nil)
	return extrudeProfilePart(c, "rounded-corner", roundedCornerRectPoints2D(1.6, 2.4, 0.35, 20), "cornerT", 0.2)
}
