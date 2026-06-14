// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// This file adds the live counterparts of the newer bridge/nopscad_*_test.go parts: cap_screw,
// box_tray (shell), leadnut (circular pattern), knob (arc revolve), hull, standoff, offset.

// --- shared caller helpers -------------------------------------------------

// conAt adds a constraint to a specific sketch index.
func (c *caller) conAt(idx int, kind string, ents ...uint64) {
	c.json("add_sketch_constraint", map[string]any{"sketchIndex": idx, "kind": kind, "entities": ents}, nil)
}

// closedProfile returns the index of the first closed profile on sketch 0 (the revolve target).
func (c *caller) closedProfile() int {
	var p struct {
		Profiles []struct {
			Index  int  `json:"index"`
			Closed bool `json:"closed"`
		} `json:"profiles"`
	}
	c.json("list_sketch_profiles", map[string]any{"sketchIndex": 0}, &p)
	for _, pr := range p.Profiles {
		if pr.Closed {
			return pr.Index
		}
	}
	return 0
}

// topFaceKey returns the reference key of the active body's top (+Z) face.
func (c *caller) topFaceKey() string {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	best, bestZ := "", math.Inf(-1)
	if len(rk.Bodies) > 0 {
		for _, f := range rk.Bodies[0].Faces {
			if len(f.Point) == 3 && f.Point[2] > bestZ {
				best, bestZ = f.Key, f.Point[2]
			}
		}
	}
	return best
}

// bodyCount returns how many bodies the active part has.
func (c *caller) bodyCount() int {
	var rk struct {
		Bodies []struct{} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	return len(rk.Bodies)
}

// checkVolumeTol is checkVolume with a caller-chosen tolerance band (curved hulls need looser).
func (c *caller) checkVolumeTol(tag string, want, tol float64) error {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	c.json("get_physical_properties", nil, &pp)
	if c.err != nil {
		return c.err
	}
	rel := math.Abs(pp.Volume-want) / want
	fmt.Printf("  %-18s volume = %.6f cm^3  want ~%.6f  (rel %.4f)\n", tag, pp.Volume, want, rel)
	if rel > tol {
		return fmt.Errorf("%s volume off by %.2f%% (> %.0f%%)", tag, rel*100, tol*100)
	}
	return nil
}

// --- cap_screw -------------------------------------------------------------

func runCapScrew(c *caller) error {
	for _, p := range [][2]string{{"headD", "5.5 mm"}, {"headH", "3 mm"}, {"shaftD", "3 mm"}, {"len", "10 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	mk := func(x0, y0, x1, y1 float64) []uint64 {
		return c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{x0, y0}, {x1, y1}}})
	}
	l := [6][]uint64{
		mk(0, 0.3, 0.275, 0.3), mk(0.275, 0.3, 0.275, 0), mk(0.275, 0, 0.15, 0),
		mk(0.15, 0, 0.15, -1), mk(0.15, -1, 0, -1), mk(0, -1, 0, 0.3),
	}
	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	if c.err != nil {
		return c.err
	}
	for _, ln := range l {
		if len(ln) < 3 {
			return fmt.Errorf("screw line reply too short")
		}
	}
	a := func(i int) uint64 { return l[i][1] }
	b := func(i int) uint64 { return l[i][2] }
	for i := 0; i < 6; i++ {
		c.con("coincident", b(i), a((i+1)%6))
	}
	for i := 0; i < 6; i++ {
		if i%2 == 0 {
			c.con("horizontal", a(i), b(i))
		} else {
			c.con("vertical", a(i), b(i))
		}
	}
	c.con("ground", o[0])
	c.con("vertical", o[0], a(0))
	c.con("horizontal", o[0], a(3))
	c.dim("distance", "headD/2", a(0), b(0))
	c.dim("distance", "headH", a(1), b(1))
	c.dim("distance", "len", a(3), b(3))
	c.dim("distance", "shaftD/2", a(4), b(4))
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("revolve", map[string]any{"sketchIndex": 0, "profileIndex": c.closedProfile(), "axisRef": "origin/axis/y", "angle": "360 deg"}); err != nil {
		return err
	}
	vol := func(lenMM float64) float64 {
		hr, hh, sr, l := 5.5/20, 3.0/10, 3.0/20, lenMM/10
		return math.Pi*hr*hr*hh + math.Pi*sr*sr*l
	}
	if err := c.checkVolume("len=10", vol(10)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "len", "expression": "16 mm"}, nil)
	return c.checkVolume("len=16 (resized)", vol(16))
}

// --- box_tray (shell) ------------------------------------------------------

func runBoxTray(c *caller) error {
	for _, p := range [][2]string{{"W", "40 mm"}, {"D", "30 mm"}, {"H", "20 mm"}, {"wall", "2 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	var rect struct {
		EntityIDs []uint64 `json:"entityIds"`
		PointIDs  []uint64 `json:"pointIds"`
	}
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0, 0}, {4, 3}}}, &rect)
	if c.err != nil || len(rect.PointIDs) < 4 {
		return fmt.Errorf("rectangle reply: %v (%v)", rect.PointIDs, c.err)
	}
	bl, br, tr, tl := rect.PointIDs[0], rect.PointIDs[1], rect.PointIDs[2], rect.PointIDs[3]
	c.con("ground", bl)
	c.con("horizontal", bl, br)
	c.con("horizontal", tl, tr)
	c.con("vertical", bl, tl)
	c.con("vertical", br, tr)
	c.dim("distance", "W", bl, br)
	c.dim("distance", "D", bl, tl)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "H", "operation": "new"}); err != nil {
		return err
	}
	if err := c.applyFeature("shell", map[string]any{"faceRefs": []string{c.topFaceKey()}, "thickness": "wall"}); err != nil {
		return err
	}
	vol := func(hMM float64) float64 {
		w, d, h, t := 4.0, 3.0, hMM/10, 0.2
		return w*d*h - (w-2*t)*(d-2*t)*(h-t)
	}
	if err := c.checkVolume("H=20", vol(20)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "H", "expression": "30 mm"}, nil)
	return c.checkVolume("H=30 (taller)", vol(30))
}

// --- leadnut (circular pattern) --------------------------------------------

func runLeadnut(c *caller) error {
	for _, p := range [][2]string{{"flangeD", "22 mm"}, {"flangeT", "3.5 mm"}, {"bore", "8 mm"}, {"holeD", "3.5 mm"}, {"pitch", "8 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	flange := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "1.1 cm"})
	bore := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.4 cm"})
	seed := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0.8, 0}}, "radius": "0.175 cm"})
	if c.err != nil || len(o) < 1 || len(flange) < 2 || len(bore) < 2 || len(seed) < 2 {
		return fmt.Errorf("leadnut entity replies too short (%v)", c.err)
	}
	c.con("ground", o[0])
	c.con("coincident", flange[1], o[0])
	c.dim("radius", "flangeD/2", flange[0])
	c.con("coincident", bore[1], o[0])
	c.dim("radius", "bore/2", bore[0])
	c.con("horizontal", o[0], seed[1])
	c.dim("distance", "pitch", o[0], seed[1])
	c.dim("radius", "holeD/2", seed[0])
	c.json("add_sketch_pattern", map[string]any{"sketchIndex": 0, "kind": "circular", "entities": []uint64{seed[0]}, "count": 3, "angle": "360 deg", "center": []float64{0, 0}}, nil)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	prof := c.profileWithHole()
	if prof < 0 {
		return fmt.Errorf("leadnut: no flange-with-holes profile")
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": prof, "distance": "flangeT", "operation": "new"}); err != nil {
		return err
	}
	vol := func(flangeDmm float64) float64 {
		R, tt, rb, rh := flangeDmm/20, 3.5/10, 8.0/20, 3.5/20
		return (math.Pi*R*R - math.Pi*rb*rb - 3*math.Pi*rh*rh) * tt
	}
	if err := c.checkVolume("flangeD=22", vol(22)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "flangeD", "expression": "26 mm"}, nil)
	return c.checkVolume("flangeD=26 (wider)", vol(26))
}

// --- knob (arc revolve) ----------------------------------------------------

func runKnob(c *caller) error {
	for _, p := range [][2]string{{"knobD", "15 mm"}, {"knobH", "18 mm"}, {"rimR", "2 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	mk := func(x0, y0, x1, y1 float64) []uint64 {
		return c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{x0, y0}, {x1, y1}}})
	}
	l0 := mk(0, 0, 0.75, 0)
	l1 := mk(0.75, 0, 0.75, 1.6)
	l3 := mk(0.55, 1.8, 0, 1.8)
	l4 := mk(0, 1.8, 0, 0)
	arc := c.ids(map[string]any{"sketchIndex": 0, "kind": "arc", "ccw": true, "points": [][]float64{{0.55, 1.6}, {0.75, 1.6}, {0.55, 1.8}}})
	if c.err != nil || len(l0) < 3 || len(l1) < 3 || len(l3) < 3 || len(l4) < 3 || len(arc) < 4 {
		return fmt.Errorf("knob entity replies too short (%v)", c.err)
	}
	cC, aStart, aEnd := arc[1], arc[2], arc[3]
	c.con("coincident", l0[2], l1[1])
	c.con("coincident", l1[2], aStart)
	c.con("coincident", aEnd, l3[1])
	c.con("coincident", l3[2], l4[1])
	c.con("coincident", l4[2], l0[1])
	c.con("horizontal", l0[1], l0[2])
	c.con("vertical", l1[1], l1[2])
	c.con("horizontal", l3[1], l3[2])
	c.con("vertical", l4[1], l4[2])
	c.con("ground", l0[1])
	c.con("horizontal", cC, aStart)
	c.con("vertical", cC, aEnd)
	c.dim("distance", "knobD/2", l0[1], l0[2])
	c.dim("distance", "knobH - rimR", l1[1], l1[2])
	c.dim("distance", "rimR", cC, aStart)
	c.dim("distance", "rimR", cC, aEnd)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("revolve", map[string]any{"sketchIndex": 0, "profileIndex": c.closedProfile(), "axisRef": "origin/axis/y", "angle": "360 deg"}); err != nil {
		return err
	}
	vol := func(hMM float64) float64 {
		R, H, r := 15.0/20, hMM/10, 2.0/10
		rimCap := math.Pi * ((R-r)*(R-r)*r + (math.Pi/2)*(R-r)*r*r + (2.0/3.0)*r*r*r)
		return math.Pi*R*R*(H-r) + rimCap
	}
	if err := c.checkVolume("knobH=18", vol(18)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "knobH", "expression": "24 mm"}, nil)
	return c.checkVolume("knobH=24 (taller)", vol(24))
}

// --- hull (two cylinders → stadium prism) ----------------------------------

func runHull(c *caller) error {
	for _, p := range [][2]string{{"r", "5 mm"}, {"h", "8 mm"}, {"d", "12 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	if err := cylinderBody(c, 0, 0, ""); err != nil {
		return err
	}
	if err := cylinderBody(c, 1, 1.2, "d"); err != nil {
		return err
	}
	if n := c.bodyCount(); n != 2 {
		return fmt.Errorf("expected 2 cylinder bodies before hull, got %d", n)
	}
	if err := c.applyFeature("hull", map[string]any{}); err != nil {
		return err
	}
	if n := c.bodyCount(); n != 1 {
		return fmt.Errorf("hull should leave 1 body, got %d", n)
	}
	vol := func(dMM float64) float64 {
		r, hh, dd := 5.0/10, 8.0/10, dMM/10
		return (math.Pi*r*r + 2*r*dd) * hh
	}
	if err := c.checkVolumeTol("d=12", vol(12), 0.03); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "d", "expression": "20 mm"}, nil)
	return c.checkVolumeTol("d=20 (spread)", vol(20), 0.03)
}

// cylinderBody extrudes a parametric circle into a separate body.
func cylinderBody(c *caller, sketchIdx int, cx float64, centreExpr string) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	circle := c.ids(map[string]any{"sketchIndex": sketchIdx, "kind": "circle", "points": [][]float64{{cx, 0}}, "radius": "0.5 cm"})
	o := c.ids(map[string]any{"sketchIndex": sketchIdx, "kind": "point", "points": [][]float64{{0, 0}}})
	if c.err != nil || len(circle) < 2 || len(o) < 1 {
		return fmt.Errorf("cylinder %d entity replies too short (%v)", sketchIdx, c.err)
	}
	c.conAt(sketchIdx, "ground", o[0])
	c.dimAt(sketchIdx, "radius", "r", circle[0])
	if centreExpr == "" {
		c.conAt(sketchIdx, "coincident", o[0], circle[1])
	} else {
		c.conAt(sketchIdx, "horizontal", o[0], circle[1])
		c.dimAt(sketchIdx, "distance", centreExpr, o[0], circle[1])
	}
	if err := c.requireConstrainedAt(sketchIdx); err != nil {
		return err
	}
	return c.applyFeature("extrude", map[string]any{"sketchIndex": sketchIdx, "profileIndex": 0, "distance": "h", "operation": "new"})
}

// --- standoff (two spheres → capsule) --------------------------------------

func runStandoff(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "d2", "expression": "4 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "L", "expression": "10 mm"}, nil)
	if err := ballBody(c, 0, ""); err != nil {
		return err
	}
	if err := ballBody(c, 1, "L"); err != nil {
		return err
	}
	if err := c.applyFeature("hull", map[string]any{}); err != nil {
		return err
	}
	if n := c.bodyCount(); n != 1 {
		return fmt.Errorf("hull should leave 1 body, got %d", n)
	}
	vol := func(lMM float64) float64 {
		r, l := 4.0/20, lMM/10
		return math.Pi*r*r*l + (4.0/3.0)*math.Pi*r*r*r
	}
	if err := c.checkVolumeTol("L=10", vol(10), 0.05); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "L", "expression": "16 mm"}, nil)
	return c.checkVolumeTol("L=16 (spread)", vol(16), 0.05)
}

// ballBody revolves a half-disk into a sphere body whose centre sits at y=centreExpr.
func ballBody(c *caller, sketchIdx int, centreExpr string) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	o := c.ids(map[string]any{"sketchIndex": sketchIdx, "kind": "point", "points": [][]float64{{0, 0}}})
	cy := 0.0
	if centreExpr != "" {
		cy = 1.0
	}
	line := c.ids(map[string]any{"sketchIndex": sketchIdx, "kind": "line", "points": [][]float64{{0, cy + 0.2}, {0, cy - 0.2}}})
	arc := c.ids(map[string]any{"sketchIndex": sketchIdx, "kind": "arc", "ccw": false, "points": [][]float64{{0, cy}, {0, cy + 0.2}, {0, cy - 0.2}}})
	if c.err != nil || len(o) < 1 || len(line) < 3 || len(arc) < 4 {
		return fmt.Errorf("ball %d entity replies too short (%v)", sketchIdx, c.err)
	}
	lineE, top, bot := line[0], line[1], line[2]
	arcCenter, arcStart, arcEnd := arc[1], arc[2], arc[3]
	c.conAt(sketchIdx, "ground", o[0])
	c.conAt(sketchIdx, "coincident", arcStart, top)
	c.conAt(sketchIdx, "coincident", arcEnd, bot)
	c.conAt(sketchIdx, "vertical", top, bot)
	c.conAt(sketchIdx, "midpoint", arcCenter, lineE)
	if centreExpr == "" {
		c.conAt(sketchIdx, "coincident", arcCenter, o[0])
	} else {
		c.conAt(sketchIdx, "vertical", o[0], arcCenter)
		c.dimAt(sketchIdx, "distance", centreExpr, o[0], arcCenter)
	}
	c.dimAt(sketchIdx, "distance", "d2", top, bot)
	if err := c.requireConstrainedAt(sketchIdx); err != nil {
		return err
	}
	return c.applyFeature("revolve", map[string]any{"sketchIndex": sketchIdx, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg", "operation": "new"})
}

// --- offset (region offset → extruded band) --------------------------------

func runOffset(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "th", "expression": "2 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	var rect struct {
		EntityIDs []uint64 `json:"entityIds"`
		PointIDs  []uint64 `json:"pointIds"`
	}
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0, 0}, {2, 1.4}}}, &rect)
	if c.err != nil || len(rect.PointIDs) < 4 {
		return fmt.Errorf("rectangle reply: %v (%v)", rect.PointIDs, c.err)
	}
	bl, br, tr, tl := rect.PointIDs[0], rect.PointIDs[1], rect.PointIDs[2], rect.PointIDs[3]
	c.con("ground", bl)
	c.con("horizontal", bl, br)
	c.con("horizontal", tl, tr)
	c.con("vertical", bl, tl)
	c.con("vertical", br, tr)
	c.dim("distance", "20 mm", bl, br)
	c.dim("distance", "14 mm", bl, tl)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	prof0 := 0
	c.json("offset_sketch", map[string]any{"sketchIndex": 0, "profileIndex": prof0, "distance": "3 mm", "arcSegments": 16}, nil)
	if c.err != nil {
		return c.err
	}
	band := c.profileWithHole()
	if band < 0 {
		return fmt.Errorf("offset: no annular band profile")
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": band, "distance": "th", "operation": "new"}); err != nil {
		return err
	}
	vol := func(thMM float64) float64 {
		w, h, d := 2.0, 1.4, 0.3
		return (2*(w+h)*d + math.Pi*d*d) * (thMM / 10)
	}
	if err := c.checkVolume("th=2", vol(2)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "th", "expression": "5 mm"}, nil)
	return c.checkVolume("th=5 (thicker)", vol(5))
}
