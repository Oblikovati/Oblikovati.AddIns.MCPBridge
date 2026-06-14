// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runPulley models a flanged belt pulley (NopSCADlib vitamins/pulley.scad) live: an 8-segment
// half-section — bore, two flanges, a recessed belt channel — revolved 360° about Y. Volume is
// the flanged cylinder minus the channel ring: π(R²−r_bore²)·w − π(R²−channel_r²)·cw.
func runPulley(c *caller) error {
	for _, p := range [][2]string{
		{"flangeD", "18 mm"}, {"boreD", "5 mm"}, {"channelD", "15 mm"},
		{"ft", "1 mm"}, {"cw", "6 mm"}, {"width", "8 mm"},
	} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	mkLine := func(x0, y0, x1, y1 float64) []uint64 {
		return c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{x0, y0}, {x1, y1}}})
	}
	l := [8][]uint64{
		mkLine(0.25, 0, 0.9, 0),
		mkLine(0.9, 0, 0.9, 0.1),
		mkLine(0.9, 0.1, 0.75, 0.1),
		mkLine(0.75, 0.1, 0.75, 0.7),
		mkLine(0.75, 0.7, 0.9, 0.7),
		mkLine(0.9, 0.7, 0.9, 0.8),
		mkLine(0.9, 0.8, 0.25, 0.8),
		mkLine(0.25, 0.8, 0.25, 0),
	}
	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	if c.err != nil {
		return c.err
	}
	if len(o) < 1 || len(l[0]) < 3 {
		return fmt.Errorf("pulley entity replies too short")
	}
	a := func(i int) uint64 { return l[i][1] }
	b := func(i int) uint64 { return l[i][2] }

	for i := 0; i < 8; i++ {
		c.con("coincident", b(i), a((i+1)%8))
	}
	for _, i := range []int{0, 2, 4, 6} {
		c.con("horizontal", a(i), b(i))
	}
	for _, i := range []int{1, 3, 5, 7} {
		c.con("vertical", a(i), b(i))
	}
	c.con("ground", o[0])
	c.con("horizontal", o[0], a(0))

	c.dim("distance", "boreD/2", o[0], a(0))
	c.dim("distance", "flangeD/2", o[0], b(0))
	c.dim("distance", "ft", a(1), b(1))
	c.dim("distance", "(flangeD - channelD)/2", a(2), b(2))
	c.dim("distance", "cw", a(3), b(3))
	c.dim("distance", "(flangeD - channelD)/2", a(4), b(4))
	c.dim("distance", "width", a(7), b(7))
	if err := c.requireConstrained(); err != nil {
		return err
	}

	if err := c.applyFeature("revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); err != nil {
		return err
	}

	vol := func(flangeDmm float64) float64 {
		R, rb, rc := flangeDmm/20, 5.0/20, 15.0/20
		w, cw := 8.0/10, 6.0/10
		return math.Pi*(R*R-rb*rb)*w - math.Pi*(R*R-rc*rc)*cw
	}
	if err := c.checkVolume("flangeD=18", vol(18)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "flangeD", "expression": "22 mm"}, nil)
	return c.checkVolume("flangeD=22 (resized)", vol(22))
}
