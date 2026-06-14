// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runBoltFlange is the live driver for a bolt-circle flange (the flanged
// coupling/leadnut family: NopSCADlib vitamins/leadnut.scad, vitamins/shaft_coupling.scad):
// an annular disc with a central bore and a ring of fastener holes. Unlike the leadnut
// driver, which builds its bolt circle at the SKETCH level, this builds it with a
// FEATURE-LEVEL circular pattern — patternCircular replicates a single hole feature about
// the axis, re-applying its cut boolean at each occurrence. Proves the feature-pattern +
// recompute path over the live C-ABI stack and shows a clean 6-hole flange in the viewport.
func runBoltFlange(c *caller) error {
	for _, p := range [][2]string{
		{"flangeD", "40 mm"}, {"flangeT", "4 mm"}, {"boreD", "10 mm"},
		{"holeD", "4 mm"}, {"boltR", "15 mm"},
	} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	const count = 6

	// Flange disc with a central bore (one annular profile on XY).
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	outer := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	bore := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.5 cm"})
	if c.err != nil || len(o) < 1 || len(outer) < 2 || len(bore) < 2 {
		return fmt.Errorf("flange entity replies too short (%v)", c.err)
	}
	c.con("ground", o[0])
	c.con("coincident", outer[1], o[0])
	c.con("coincident", bore[1], o[0])
	c.dim("radius", "flangeD/2", outer[0])
	c.dim("radius", "boreD/2", bore[0])
	if err := c.requireConstrained(); err != nil {
		return err
	}
	annulus := c.profileWithHole()
	if annulus < 0 {
		return fmt.Errorf("boltflange: no annular flange profile")
	}
	if err := c.applyFeature("extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": annulus, "distance": "flangeT", "operation": "new",
	}); err != nil {
		return err
	}

	// One seed bolt hole on the bolt circle (a cut feature), then patternCircular it.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	o2 := c.ids(map[string]any{"sketchIndex": 1, "kind": "point", "points": [][]float64{{0, 0}}})
	hole := c.ids(map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{1.5, 0}}, "radius": "0.2 cm"})
	if c.err != nil || len(o2) < 1 || len(hole) < 2 {
		return fmt.Errorf("flange hole replies too short (%v)", c.err)
	}
	c.conAt(1, "ground", o2[0])
	c.conAt(1, "horizontal", o2[0], hole[1])
	c.dimAt(1, "distance", "boltR", o2[0], hole[1])
	c.dimAt(1, "radius", "holeD/2", hole[0])
	if err := c.requireConstrainedAt(1); err != nil {
		return err
	}
	seed, err := c.applyNamed("extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "operation": "cut", "extent": "through-all",
	})
	if err != nil {
		return err
	}
	if err := c.applyFeature("patternCircular", map[string]any{
		"sourceFeatures": []string{seed}, "count": count, "angle": "360 deg", "axisDir": []float64{0, 0, 1},
	}); err != nil {
		return err
	}

	if err := c.checkVolume("flangeD=40", flangeVol(40, count)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "flangeD", "expression": "50 mm"}, nil)
	return c.checkVolume("flangeD=50 (wider)", flangeVol(50, count))
}

// flangeVol = annular disc minus `count` cylindrical bolt holes, cm^3 (flangeD in mm).
func flangeVol(flangeDmm float64, count float64) float64 {
	R, rb, rh, th := flangeDmm/20, 10.0/20, 4.0/20, 4.0/10
	return math.Pi * ((R*R - rb*rb) - count*rh*rh) * th
}
