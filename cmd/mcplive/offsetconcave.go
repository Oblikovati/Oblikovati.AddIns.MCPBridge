// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runOffsetConcave is the live driver for a 2D offset of a CONCAVE outline: an L-shape
// (authored with the new polyline entity) grown outward by d into an L-frame band, extruded.
// Exercises offset over a reflex corner and shows an L-shaped frame in the viewport.
func runOffsetConcave(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	// L outline (cm): 3×3 with a 1.5×1.5 notch (reflex corner at 1.5,1.5).
	c.json("add_sketch_entity", map[string]any{
		"sketchIndex": 0, "kind": "polyline", "closed": true,
		"points": [][]float64{{0, 0}, {3, 0}, {3, 1.5}, {1.5, 1.5}, {1.5, 3}, {0, 3}},
	}, nil)
	var off struct {
		Created []uint64 `json:"created"`
	}
	c.json("offset_sketch", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "3 mm", "arcSegments": 16}, &off)
	if c.err != nil {
		return c.err
	}
	band := c.profileWithHole()
	if band < 0 {
		return fmt.Errorf("offsetconcave: no annular band profile")
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": band, "distance": "2 mm", "operation": "new"}); err != nil {
		return err
	}
	// Steiner band area P·d + π·d² (P=12 cm, d=0.3 cm), extruded 0.2 cm.
	return c.checkVolume("concave-L offset frame", (12.0*0.3+math.Pi*0.3*0.3)*0.2)
}
