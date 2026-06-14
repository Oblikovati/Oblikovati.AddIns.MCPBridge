// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

func runSemiTeardrop(c *caller) error {
	for _, p := range [][2]string{{"r", "4 mm"}, {"h", "20 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	origin := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	dia := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{-0.4, 0}, {0.4, 0}}})
	arc := c.ids(map[string]any{"sketchIndex": 0, "kind": "arc", "ccw": true, "points": [][]float64{{0, 0}, {0.4, 0}, {-0.4, 0}}})
	if c.err != nil || len(origin) < 1 || len(dia) < 3 || len(arc) < 4 {
		return fmt.Errorf("semi_teardrop entity replies too short (%v)", c.err)
	}
	c.con("ground", origin[0])
	c.con("coincident", arc[1], origin[0])
	c.con("coincident", arc[2], dia[2])
	c.con("coincident", arc[3], dia[1])
	c.con("horizontal", dia[1], dia[2])
	c.con("midpoint", origin[0], dia[0])
	c.dim("distance", "2 * r", dia[1], dia[2])
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": c.closedProfile(), "distance": "h", "operation": "new"}); err != nil {
		return err
	}
	vol := func(rMM float64) float64 { r, h := rMM/10, 2.0; return math.Pi * r * r * h / 2 }
	if err := c.checkVolumeTol("r=4", vol(4), 0.03); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "r", "expression": "6 mm"}, nil)
	return c.checkVolumeTol("r=6 (resized)", vol(6), 0.03)
}
