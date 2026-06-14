// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runFaceplate is the live driver for NopSCADlib's motor_faceplate — a square plate with a
// central raised boss (extrude-JOIN), a bore drilled through the BOSS TOP face (a
// feature-created face, exercising topological naming), and a 2×2 corner bolt pattern. Proves
// the combination over the live C-ABI stack.
func runFaceplate(c *caller) error {
	for _, p := range [][2]string{{"w", "40 mm"}, {"t", "3 mm"}, {"bossDia", "20 mm"}, {"boreDia", "8 mm"}, {"boltDia", "4 mm"}, {"bossLen", "8 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	// Plate: w×w square (corner at origin) extruded t.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	r := c.ids(map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0, 0}, {4, 4}}})
	if len(r) < 5 {
		return fmt.Errorf("rectangle reply too short: %v", r)
	}
	bl, br, tr, tl := r[1], r[2], r[3], r[4]
	c.con("ground", bl)
	c.con("horizontal", bl, br)
	c.con("vertical", bl, tl)
	c.con("horizontal", tl, tr)
	c.con("vertical", br, tr)
	c.dim("distance", "w", bl, br)
	c.dim("distance", "w", bl, tl)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": c.closedProfile(), "distance": "t", "operation": "new"}); err != nil {
		return err
	}

	// Central boss: a disc at the centre, JOINED, bossLen tall.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	boss := c.ids(map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{2, 2}}, "radius": "1 cm"})
	c.conAt(1, "ground", boss[1])
	c.dimAt(1, "diameter", "bossDia", boss[0])
	if err := c.requireConstrainedAt(1); err != nil {
		return err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "operation": "join", "distance": "bossLen"}); err != nil {
		return err
	}

	// Bore through the boss top face (now the topmost face).
	if err := c.applyFeature("hole", map[string]any{"faceRef": c.topFaceKey(), "diameter": "boreDia"}); err != nil {
		return err
	}

	// Corner bolt holes: seed cut + 2×2 pattern.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	bolt := c.ids(map[string]any{"sketchIndex": 2, "kind": "circle", "points": [][]float64{{0.5, 0.5}}, "radius": "0.2 cm"})
	c.conAt(2, "ground", bolt[1])
	c.dimAt(2, "diameter", "boltDia", bolt[0])
	if err := c.requireConstrainedAt(2); err != nil {
		return err
	}
	boltName, err := c.applyNamed("extrude", map[string]any{"sketchIndex": 2, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
	if err != nil {
		return err
	}
	if err := c.applyFeature("patternRectangular", map[string]any{
		"sourceFeatures": []string{boltName}, "countX": 2, "countY": 2,
		"stepX": []float64{3, 0, 0}, "stepY": []float64{0, 3, 0},
	}); err != nil {
		return err
	}

	if err := c.checkVolume("bossLen=8", faceplateVol(8)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "bossLen", "expression": "12 mm"}, nil)
	return c.checkVolume("bossLen=12 (taller)", faceplateVol(12))
}

// faceplateVol = plate + boss above plate − central bore − four bolt holes, cm^3 (bossLen mm).
func faceplateVol(bossLenMM float64) float64 {
	const w, t, bo, bi, rh = 4.0, 0.3, 1.0, 0.4, 0.2
	bl := bossLenMM / 10
	return w*w*t + math.Pi*bo*bo*(bl-t) - math.Pi*bi*bi*bl - 4*math.Pi*rh*rh*t
}
