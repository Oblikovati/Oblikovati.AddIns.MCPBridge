// SPDX-License-Identifier: GPL-2.0-only

package main

func runQuadrant(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "quadT", "expression": "2 mm"}, nil)
	return extrudeProfilePart(c, "quadrant", roundedCornerRectPoints2D(2.0, 1.4, 0.5, 20), "quadT", 0.2)
}
