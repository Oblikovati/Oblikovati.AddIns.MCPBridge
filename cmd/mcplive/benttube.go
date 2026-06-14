// SPDX-License-Identifier: GPL-2.0-only

package main

import "math"

// runBentTube is the live driver for a bent tube / pipe elbow: a circle swept along a 90°
// arc rail. It exercises sweep-frame transport around a CURVE (the straight-rail tubing did
// not), and was the part that surfaced the curved-rail bug (an arc path collapsed to its
// chord). Shows a quarter-torus elbow in the viewport. Needs the host with the path
// arc-sampling fix (restart the app after building it).
func runBentTube(c *caller) error {
	const r, bend = 0.2, 2.0 // tube radius, bend radius, cm

	// Profile: a circle on XY at the origin (the rail's start).
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.2 cm"})

	// Rail: a 90° arc on XZ from the origin (tangent +Z) curving to +X.
	c.json("create_sketch", map[string]any{"plane": "XZ"}, nil)
	c.json("add_sketch_entity", map[string]any{
		"sketchIndex": 1, "kind": "arc",
		"points": [][]float64{{bend, 0}, {0, 0}, {bend, bend}}, "ccw": false,
	}, nil)

	if err := c.applyFeature("sweep", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "pathSketchIndex": 1, "pathIndex": 0,
	}); err != nil {
		return err
	}
	// Pappus: V = πr² · (centroid arc length) = πr² · (π/2 · bend).
	return c.checkVolume("bent tube (90° elbow)", math.Pi*r*r*(math.Pi/2*bend))
}
