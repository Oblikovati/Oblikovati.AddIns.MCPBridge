// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runBallStud is the live gate for Oblikovati#2036: a rod COAXIAL with a ball meets it in a circle,
// the one sphere∩cylinder configuration with a closed-form answer (OCCT solves the same pair in
// IntAna_QuadQuadGeo), so every boolean in the family must come back as THREE analytic faces — a
// spherical cap, a cylindrical band and a planar disc — instead of the ~500-facet inscribed polyhedron
// the CSG fallback used to ship.
//
// The check that actually bites is the FACE CENSUS, not the volume. A faceted union is still a valid
// solid of roughly the right size (its error was 1.3%), so a volume band alone can be talked into
// passing; "one sphere + one cylinder + one plane" cannot. The volume is then held to 0.1%, which only
// an exact B-rep reaches.
//
// All four results in the family are driven. Which body is the TARGET is the modelling order — a
// feature's own body is always the tool — so the socket is built ball-first and the rest shank-first.
func runBallStud(c *caller) error {
	for _, run := range []func(*caller) error{
		ballStudUnion, ballStudSocket, ballStudStub, ballStudPlug,
	} {
		if err := run(c); err != nil {
			return err
		}
	}
	return nil
}

const (
	ballStudBallD = 1.0  // cm — the Ø10 head
	ballStudRodD  = 0.6  // cm — the Ø6 shank
	ballStudLen   = 1.5  // cm, from the ball centre out
	ballStudTol   = 1e-3 // volume band: tessellation only, once the B-rep is exact
)

// ballStudPlugVolume is the material the two solids share: the rod up to the seam plane at
// √(R²−r²) — OCCT's circle offset — plus the spherical cap the ball raises above it.
func ballStudPlugVolume(ballD, rodD float64) float64 {
	rb, rr := ballD/2, rodD/2
	d := math.Sqrt(rb*rb - rr*rr)
	dome := math.Pi * (rb - d) * (rb - d) * (rb - (rb-d)/3)
	return math.Pi*rr*rr*d + dome
}

func ballStudBallVolume(ballD float64) float64 {
	rb := ballD / 2
	return 4.0 / 3.0 * math.Pi * rb * rb * rb
}

func ballStudRodVolume() float64 {
	rr := ballStudRodD / 2
	return math.Pi * rr * rr * ballStudLen
}

// ballStudUnion is the stud itself. It also resizes the head, because a parametric rebuild must not
// quietly drop back to the fallback.
func ballStudUnion(c *caller) error {
	ballStudDoc(c, "union")
	if err := ballStudShank(c, 0, "new"); err != nil {
		return err
	}
	if err := ballStudBall(c, 1, "join"); err != nil {
		return err
	}
	union := func(ballD float64) float64 {
		return ballStudBallVolume(ballD) + ballStudRodVolume() - ballStudPlugVolume(ballD, ballStudRodD)
	}
	if err := c.checkBallStud("ball ∪ rod (stud)", union(ballStudBallD)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "ball_d", "expression": "14 mm"}, nil)
	return c.checkBallStud("ball ∪ rod (Ø14 head)", union(1.4))
}

// ballStudSocket is ball − rod: a blind spherical bore with a flat bottom. The ball must be modelled
// FIRST for the rod to be the tool.
func ballStudSocket(c *caller) error {
	ballStudDoc(c, "socket")
	if err := ballStudBall(c, 0, "new"); err != nil {
		return err
	}
	if err := ballStudShank(c, 1, "cut"); err != nil {
		return err
	}
	want := ballStudBallVolume(ballStudBallD) - ballStudPlugVolume(ballStudBallD, ballStudRodD)
	return c.checkBallStud("ball − rod (socket)", want)
}

// ballStudStub is rod − ball: the free shank with the ball's own surface hollowed into its base.
func ballStudStub(c *caller) error {
	ballStudDoc(c, "stub")
	if err := ballStudShank(c, 0, "new"); err != nil {
		return err
	}
	if err := ballStudBall(c, 1, "cut"); err != nil {
		return err
	}
	return c.checkBallStud("rod − ball (dimpled stub)",
		ballStudRodVolume()-ballStudPlugVolume(ballStudBallD, ballStudRodD))
}

// ballStudPlug is ball ∩ rod: the buried length of the shank, domed by the ball.
func ballStudPlug(c *caller) error {
	ballStudDoc(c, "plug")
	if err := ballStudShank(c, 0, "new"); err != nil {
		return err
	}
	if err := ballStudBall(c, 1, "intersect"); err != nil {
		return err
	}
	return c.checkBallStud("ball ∩ rod (plug)", ballStudPlugVolume(ballStudBallD, ballStudRodD))
}

// ballStudDoc opens a fresh part carrying the three driving parameters.
func ballStudDoc(c *caller, name string) {
	c.json("close_all_documents", map[string]any{"force": true}, nil)
	c.json("create_document", map[string]any{"type": "part", "name": "ballstud-" + name}, nil)
	for _, p := range [][2]string{{"ball_d", "10 mm"}, {"shank_d", "6 mm"}, {"shank_len", "15 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
}

// ballStudShank extrudes the Ø6 shank along +Y from the origin — a circle on XZ, whose normal is +Y.
func ballStudShank(c *caller, sketchIndex int, op string) error {
	c.json("create_sketch", map[string]any{"plane": "XZ"}, nil)
	addConstrainedCircle(c, sketchIndex, []float64{0, 0}, "0.3 cm", "shank_d / 2")
	if c.err != nil {
		return c.err
	}
	return c.applyFeature("extrude", map[string]any{
		"sketchIndex": sketchIndex, "profileIndex": 0, "distance": "shank_len", "operation": op,
	})
}

// ballStudBall revolves a half-disc 360° about the Y axis. The model layer recognises that profile as a
// true geom.Sphere (sphereProfileSolid) rather than facetting the revolve, which is what lets the
// coaxial recognizer see a sphere at all.
func ballStudBall(c *caller, sketchIndex int, op string) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	o := c.ids(map[string]any{"sketchIndex": sketchIndex, "kind": "point", "points": [][]float64{{0, 0}}})
	line := c.ids(map[string]any{"sketchIndex": sketchIndex, "kind": "line",
		"points": [][]float64{{0, 0.5}, {0, -0.5}}})
	arc := c.ids(map[string]any{"sketchIndex": sketchIndex, "kind": "arc",
		"points": [][]float64{{0, 0}, {0, 0.5}, {0, -0.5}}, "ccw": false})
	if c.err != nil {
		return c.err
	}
	if len(o) < 1 || len(line) < 3 || len(arc) < 4 {
		return fmt.Errorf("ball sketch reply too short: o=%v line=%v arc=%v", o, line, arc)
	}
	ballStudHalfDiscConstraints(c, sketchIndex, o[0], line, arc)
	if err := c.requireConstrainedAt(sketchIndex); err != nil {
		return err
	}
	return c.applyFeature("revolve", map[string]any{
		"sketchIndex": sketchIndex, "profileIndex": 0, "axisRef": "origin/axis/y",
		"angle": "360 deg", "operation": op,
	})
}

// ballStudHalfDiscConstraints pins the half-disc to 0 DOF: the arc centred on the grounded origin, its
// ends on the diameter line, that line vertical and bisected by the origin, and one diameter dimension.
func ballStudHalfDiscConstraints(c *caller, sketchIndex int, origin uint64, line, arc []uint64) {
	c.conAt(sketchIndex, "ground", origin)
	c.conAt(sketchIndex, "coincident", arc[1], origin)
	c.conAt(sketchIndex, "coincident", arc[2], line[1])
	c.conAt(sketchIndex, "coincident", arc[3], line[2])
	c.conAt(sketchIndex, "vertical", line[1], line[2])
	c.conAt(sketchIndex, "midpoint", origin, line[0])
	c.dimAt(sketchIndex, "distance", "ball_d", line[1], line[2])
}

// checkBallStud is the pair of assertions every result in the family must pass: the closed-form volume
// to a tessellation-only band, and the exact three-face analytic census.
func (c *caller) checkBallStud(tag string, want float64) error {
	if err := c.checkVolumeTol(tag, want, ballStudTol); err != nil {
		return err
	}
	return c.checkAnalyticTriple(tag)
}

// checkAnalyticTriple fails unless the active part is ONE body of exactly three analytic faces — one
// sphere, one cylinder, one plane. This is what separates the exact path from the CSG fallback: the
// fallback's result is hundreds of "plane" facets and carries no sphere at all.
func (c *caller) checkAnalyticTriple(tag string) error {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Kind string `json:"kind"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	if c.err != nil {
		return c.err
	}
	if len(rk.Bodies) != 1 {
		return fmt.Errorf("%s: %d bodies, want 1", tag, len(rk.Bodies))
	}
	kinds := map[string]int{}
	for _, f := range rk.Bodies[0].Faces {
		kinds[f.Kind]++
	}
	n := len(rk.Bodies[0].Faces)
	if n != 3 || kinds["sphere"] != 1 || kinds["cylinder"] != 1 || kinds["plane"] != 1 {
		return fmt.Errorf("%s: %d faces %v, want one sphere + one cylinder + one plane "+
			"(a faceted result means the boolean fell back to triangle-soup CSG)", tag, n, kinds)
	}
	fmt.Printf("  %-24s exact: 3 analytic faces %v\n", tag, kinds)
	return nil
}
