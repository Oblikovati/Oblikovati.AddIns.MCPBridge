// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runRevTaper is the live stress for Oblikovati#1603 (audit A7): the revolve surface-type
// classifier reads the meridian edge's dimensionless SLOPE instead of an absolute 1e-7 radial
// gap. A stepped shaft — an oblique taper section under a straight wall — must classify
// taper→cone and wall→cylinder from real solver-produced profile coordinates, proven by the
// analytic frustum+cylinder volume (a cylinder misread of the taper differs by the
// frustum-vs-cylinder deficit, ~4% here). Then the taper is re-parameterized to r1 = r0, making
// the taper edge vertical BY SOLVED VALUE, not by constraint — the solver-noise case where an
// over-tight slope test would misclassify a true cylinder as a cone with an apex ~1/noise away
// and corrupt the recompute.
//
// NOTE: an earlier revision also chamfered the top rim; that step is parked on
// Oblikovati#1689 — the general wedge-cut chamfer silently zeroes this body on develop too
// (pre-existing, unrelated to the classifier).
func runRevTaper(c *caller) error {
	for _, p := range [][2]string{
		{"r0", "8 mm"}, {"r1", "10 mm"}, {"wall", "10 mm"}, {"height", "20 mm"},
	} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	mkLine := func(x0, y0, x1, y1 float64) []uint64 {
		return c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{x0, y0}, {x1, y1}}})
	}
	l := [5][]uint64{
		mkLine(0, 0, 0.8, 0), // base disk
		mkLine(0.8, 0, 1, 1), // taper (oblique → cone)
		mkLine(1, 1, 1, 2),   // straight wall (→ cylinder)
		mkLine(1, 2, 0, 2),   // top disk
		mkLine(0, 2, 0, 0),   // axis edge
	}
	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	if c.err != nil {
		return c.err
	}
	if len(o) < 1 || len(l[0]) < 3 {
		return fmt.Errorf("revtaper entity replies too short")
	}
	a := func(i int) uint64 { return l[i][1] }
	b := func(i int) uint64 { return l[i][2] }

	for i := 0; i < 5; i++ {
		c.con("coincident", b(i), a((i+1)%5))
	}
	c.con("horizontal", a(0), b(0))
	c.con("vertical", a(2), b(2))
	c.con("horizontal", a(3), b(3))
	c.con("vertical", a(4), b(4))
	c.con("ground", o[0])
	c.con("coincident", o[0], a(0))

	c.dim("distance", "r0", a(0), b(0))
	c.dim("distance", "r1", a(3), b(3))
	c.dim("distance", "wall", a(2), b(2))
	c.dim("distance", "height", a(4), b(4))
	if err := c.requireConstrained(); err != nil {
		return err
	}

	if err := c.applyFeature("revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); err != nil {
		return err
	}

	// Frustum (taper section, Pappus) + wall cylinder, analytic in cm.
	shaft := func(r0, r1, taperH, wallH float64) float64 {
		return math.Pi/3*taperH*(r0*r0+r0*r1+r1*r1) + math.Pi*r1*r1*wallH
	}
	if err := c.checkVolume("taper r1=10", shaft(0.8, 1.0, 1.0, 1.0)); err != nil {
		return err
	}

	// r1 := r0: the taper edge becomes vertical by SOLVED VALUE — the solver-noise cylinder case.
	c.json("set_parameter", map[string]any{"name": "r1", "expression": "8 mm"}, nil)
	return c.checkVolume("straight r1=r0", shaft(0.8, 0.8, 1.0, 1.0))
}
