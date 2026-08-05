// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runStraddlingHole drives Oblikovati#2030 through the LIVE C-ABI stack: a plate with two
// through-holes, then a block joined on top whose footprint edge CROSSES both holes, leaving a
// thin crescent of each outside the block.
//
// The plate's top plane then carries openings that TOUCH — the block footprint and each hole — so
// they bound one connected region. Nesting them as separate hole loops wrote their shared arcs
// twice, giving a face with edges whose two uses lay on that same face, and χ = V−E+2F−L drifted by
// one per doubled edge: the Raspberry-Pi camera PCB landed on χ = −31 across 31 doubled edges and
// the solid was rejected outright.
//
// The volume is the check that survives the fix being wrong in either direction: the block sits
// ABOVE the plate and the holes go DOWN through it, so the union is exactly plate + block minus the
// two bores — no overlap to reason about, and any doubled/dropped boundary moves it.
func runStraddlingHole(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "plate_l", "expression": "25 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "plate_w", "expression": "24 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "plate_t", "expression": "1 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "hole_d", "expression": "2.1 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "blk_w", "expression": "8 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "blk_d", "expression": "5 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "blk_h", "expression": "1 mm"}, nil)

	// 1. Plate, top face at z=0 (grown downwards).
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{
		"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{-1.25, -1.2}, {1.25, 1.2}},
	}, nil)
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "plate_t", "direction": "negative", "operation": "new",
	}); err != nil {
		return fmt.Errorf("plate: %w", err)
	}

	// 2. Two holes at y=0.96, each DIMENSIONED by hole_d so the resize below actually drives them
	// (an undimensioned circle keeps its literal radius and makes the resize check vacuous). At
	// Ø2.1 their far side reaches y=1.065, past the block edge at y=1.05.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	for _, x := range []float64{0.2, -0.2} {
		ids := c.ids(map[string]any{
			"sketchIndex": 1, "kind": "circle", "points": [][]float64{{x, 0.96}}, "radius": "0.105 cm",
		})
		if c.err != nil {
			return c.err
		}
		c.dimAt(1, "diameter", "hole_d", ids[0])
	}
	c.json("solve_sketch", map[string]any{"sketchIndex": 1}, nil)
	for i := 0; i < 2; i++ {
		if err := c.applyFeature("extrude", map[string]any{
			"sketchIndex": 1, "profileIndex": i, "distance": "plate_t", "direction": "negative", "operation": "cut",
		}); err != nil {
			return fmt.Errorf("hole %d: %w", i, err)
		}
	}

	// 3. The block, joined on top. Its edge at y=1.05 cuts BOTH holes: 0.015 of each stays outside.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{
		"sketchIndex": 2, "kind": "rectangle", "points": [][]float64{{-0.4, 0.55}, {0.4, 1.05}},
	}, nil)
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 2, "profileIndex": 0, "distance": "blk_h", "operation": "join",
	}); err != nil {
		return fmt.Errorf("block join over the straddled holes: %w", err)
	}

	// plate + block − 2 bores. The bores are inside the plate footprint and the block sits above
	// the plate, so nothing double-counts.
	want := 2.5*2.4*0.1 + 0.8*0.5*0.1 - 2*math.Pi*0.105*0.105*0.1
	if err := c.checkVolume("straddled-hole join", want); err != nil {
		return err
	}
	// Parametric resize: widen the holes so they straddle DEEPER, and re-check.
	c.json("set_parameter", map[string]any{"name": "hole_d", "expression": "2.6 mm"}, nil)
	wantWide := 2.5*2.4*0.1 + 0.8*0.5*0.1 - 2*math.Pi*0.13*0.13*0.1
	if err := c.checkVolume("straddled-hole join (Ø2.6)", wantWide); err != nil {
		return err
	}

	// Look straight down the straddled corner and capture what the renderer actually drew, then
	// again in normal-debug (front-facing GREEN, back-facing RED). A doubled boundary shows up
	// there as red slivers along the block edge where the zero-width flap faces away.
	c.json("set_camera", map[string]any{
		"eye": []float64{2.4, -1.6, 2.2}, "target": []float64{0, 0.9, 0}, "up": []float64{0, 0, 1}, "fov": 0.61,
	}, nil)
	c.json("capture_viewport", map[string]any{"path": "/tmp/straddle-shaded.png"}, nil)
	c.json("set_normal_debug", map[string]any{"on": true}, nil)
	c.json("capture_viewport", map[string]any{"path": "/tmp/straddle-normals.png"}, nil)
	c.json("set_normal_debug", map[string]any{"on": false}, nil)

	// Close up on ONE straddled hole from the +y side, where the crescent that survives outside
	// the block is: the opening must read as a single continuous outline running from the block
	// edge, around the arc, and back — not as an arc drawn over the block's own edge.
	c.json("set_camera", map[string]any{
		"eye": []float64{0.32, 1.55, 0.62}, "target": []float64{0.2, 1.02, 0}, "up": []float64{0, 0, 1}, "fov": 0.61,
	}, nil)
	c.json("capture_viewport", map[string]any{"path": "/tmp/straddle-closeup.png"}, nil)
	return c.err
}
