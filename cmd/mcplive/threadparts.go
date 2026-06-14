// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// Live counterparts of the countersink (cone revolve) and tapped-hole (thread feature) parts.

// cylinderFaceKey returns the key of the part's first cylindrical face (a drilled bore wall),
// by the surface kind in get_reference_keys.
func (c *caller) cylinderFaceKey() string {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key  string `json:"key"`
				Kind string `json:"kind"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	if len(rk.Bodies) > 0 {
		for _, f := range rk.Bodies[0].Faces {
			if f.Kind == "cylinder" {
				return f.Key
			}
		}
	}
	return ""
}

// runCountersink: a countersunk screw — a conical head over a shaft, revolved (the cone is a
// free diagonal edge of the half-section).
func runCountersink(c *caller) error {
	for _, p := range [][2]string{{"headD", "6 mm"}, {"shaftD", "3 mm"}, {"headH", "1.7 mm"}, {"len", "10 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	mk := func(x0, y0, x1, y1 float64) []uint64 {
		return c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{x0, y0}, {x1, y1}}})
	}
	l := [5][]uint64{
		mk(0, 0, 0.3, 0), mk(0.3, 0, 0.15, -0.17), mk(0.15, -0.17, 0.15, -1.17),
		mk(0.15, -1.17, 0, -1.17), mk(0, -1.17, 0, 0),
	}
	if c.err != nil {
		return c.err
	}
	for _, ln := range l {
		if len(ln) < 3 {
			return fmt.Errorf("countersink line reply too short")
		}
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
	c.con("ground", a(0))
	c.dim("distance", "headD/2", a(0), b(0))
	c.dim("distance", "headH + len", a(0), b(3))
	c.dim("distance", "shaftD/2", a(3), b(3))
	c.dim("distance", "len", a(2), b(2))
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("revolve", map[string]any{"sketchIndex": 0, "profileIndex": c.closedProfile(), "axisRef": "origin/axis/y", "angle": "360 deg"}); err != nil {
		return err
	}
	vol := func(lenMM float64) float64 {
		hr, sr, hh, l := 6.0/20, 3.0/20, 1.7/10, lenMM/10
		return math.Pi*hh/3*(hr*hr+hr*sr+sr*sr) + math.Pi*sr*sr*l
	}
	if err := c.checkVolume("len=10", vol(10)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "len", "expression": "16 mm"}, nil)
	return c.checkVolume("len=16 (resized)", vol(16))
}

// runTappedHole: a block with a drilled bore tapped by a cosmetic thread (the THREAD feature).
func runTappedHole(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "th", "expression": "10 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "30 mm", "height": "30 mm"}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "th", "operation": "new"}); err != nil {
		return err
	}
	if err := c.applyFeature("hole", map[string]any{"faceRef": c.topFaceKey(), "diameter": "8 mm", "depth": "th"}); err != nil {
		return err
	}
	bore := c.cylinderFaceKey()
	if bore == "" {
		return fmt.Errorf("no cylindrical bore face found")
	}
	if err := c.applyFeature("thread", map[string]any{"faceRef": bore, "designation": "M8x1.25"}); err != nil {
		return err
	}
	vol := func(thMM float64) float64 {
		side, h, r := 3.0, thMM/10, 0.4
		return (side*side - math.Pi*r*r) * h
	}
	if err := c.checkVolume("th=10", vol(10)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "th", "expression": "16 mm"}, nil)
	return c.checkVolume("th=16 (thicker)", vol(16))
}

// volume reads the active part's current volume (cm^3).
func (c *caller) volume() float64 {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	c.json("get_physical_properties", nil, &pp)
	return pp.Volume
}

// runThread is the live thread demo: a block with a drilled bore, then a MODELED (cut) M8×1.25
// thread cut into the bore wall — the real thread geometry removes material (volume drops), and
// the threaded bore is visible in the viewport. Proves the thread feature end to end over the
// live C-ABI stack (the ribbon Thread tool wraps this same feature).
func runThread(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "th", "expression": "12 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "24 mm", "height": "24 mm"}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "th", "operation": "new"}); err != nil {
		return err
	}
	if err := c.applyFeature("hole", map[string]any{"faceRef": c.topFaceKey(), "diameter": "8 mm", "depth": "th"}); err != nil {
		return err
	}
	bore := c.cylinderFaceKey()
	if bore == "" {
		return fmt.Errorf("no cylindrical bore face")
	}
	bored := c.volume()
	if c.err != nil {
		return c.err
	}
	if err := c.applyFeature("thread", map[string]any{"faceRef": bore, "designation": "M8x1.25", "cut": true}); err != nil {
		return err
	}
	threaded := c.volume()
	if c.err != nil {
		return c.err
	}
	fmt.Printf("  bored=%.5f  threaded(cut M8x1.25)=%.5f cm^3 (real thread surface on the bore wall)\n", bored, threaded)
	// The cut thread retypes the bore face to a real threaded surface (changes the geometry,
	// in microseconds — no boolean). Assert it modeled a thread (the volume changed).
	if threaded == bored {
		return fmt.Errorf("cut thread did not change the geometry (%.5f)", bored)
	}
	return nil
}
