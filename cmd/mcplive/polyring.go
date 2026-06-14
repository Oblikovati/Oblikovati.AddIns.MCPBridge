// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runPolyRing(c *caller) error {
	for _, p := range [][2]string{{"outerR", "7 mm"}, {"innerR", "3.5 mm"}, {"ringT", "1.2 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "7 mm", "outerR")
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": regularPolygon(12, 0.35, math.Pi/12)}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": c.profileWithHole(), "distance": "ringT", "operation": "new"}); err != nil {
		return err
	}
	want := (math.Pi*0.7*0.7 - polygonArea(regularPolygon(12, 0.35, math.Pi/12))) * 0.12
	return c.checkVolumeTol("poly-ring", want, 0.02)
}
