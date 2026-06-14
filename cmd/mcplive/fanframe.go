// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runFanFrame is the live driver for the FRAME of a 50 mm fan (NopSCADlib vitamins/fan.scad):
// a square plate with its four vertical corners rounded (3D fillet), a large central bore, and
// four corner mounting holes — a real constituent of the fan minus the hub+blades. It exercises
// the fillet feature on real geometry plus a big bore and a hole pattern, climbing toward the
// full fan; shows a clean rounded fan frame in the viewport.
func runFanFrame(c *caller) error {
	// 50×50×15 mm square plate centered on the origin.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2.5, 2.5}}})
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "15 mm", "operation": "new"}); err != nil {
		return err
	}
	// Round the four vertical corner edges (r=5 mm).
	corners := c.cornerEdgesXY(2.0)
	if len(corners) < 4 {
		return fmt.Errorf("fanframe: want 4 corner edges, found %d", len(corners))
	}
	if err := c.applyFeature("fillet", map[string]any{"edgeRefs": corners[:4], "radius": "5 mm"}); err != nil {
		return err
	}
	// Central bore Ø47 mm through the frame.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "23.5 mm"})
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": "30 mm", "operation": "cut", "direction": "symmetric"}); err != nil {
		return err
	}
	// Four Ø3.4 mm corner mounting holes.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	for _, p := range [][]float64{{2, 2}, {-2, 2}, {-2, -2}, {2, -2}} {
		c.ids(map[string]any{"sketchIndex": 2, "kind": "circle", "points": [][]float64{p}, "radius": "1.7 mm"})
	}
	for pi := 0; pi < 4; pi++ {
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 2, "profileIndex": pi, "distance": "30 mm", "operation": "cut", "direction": "symmetric"}); err != nil {
			return fmt.Errorf("mount hole %d: %w", pi, err)
		}
	}
	// Empirical faceted volume (the Ø47 bore facets ~2.4% off the analytic ideal).
	return c.checkVolume("fan frame", 10.85990)
}

// cornerEdgesXY returns reference keys of edges whose representative point is past minXY in
// BOTH x and y — the four vertical corner edges of a centered rectangular block.
func (c *caller) cornerEdgesXY(minXY float64) []string {
	var rk struct {
		Bodies []struct {
			Edges []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"edges"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	var keys []string
	for _, b := range rk.Bodies {
		for _, e := range b.Edges {
			if len(e.Point) == 3 && math.Abs(e.Point[0]) > minXY && math.Abs(e.Point[1]) > minXY {
				keys = append(keys, e.Key)
			}
		}
	}
	return keys
}
