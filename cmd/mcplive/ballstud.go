// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// runBallStud is the live gate for Oblikovati#2036 and #2061: a rod COAXIAL with a ball meets it in a
// circle, the one sphere∩cylinder configuration with a closed-form answer (OCCT solves the same pair in
// IntAna_QuadQuadGeo), so every boolean in the family must come back as a handful of ANALYTIC faces
// instead of the ~500-facet inscribed polyhedron the CSG fallback used to ship.
//
// The check that actually bites is the FACE CENSUS, not the volume. A faceted union is still a valid
// solid of roughly the right size (its error was 1.3%), so a volume band alone can be talked into
// passing; "one sphere + one cylinder + one plane" cannot. The volume is then held to 0.1%, which only
// an exact B-rep reaches.
//
// Every extent of the shank is driven: the four results where it ENDS inside the ball, the four where it
// passes right THROUGH (whose ball face is a spherical BELT rather than a cap), and the two where it
// stops part way through the ball's SHOULDER — the extent where a plane∩sphere circle joins the seam
// circle, so the ball survives in two pieces and the shank's end cap survives as an ANNULUS. Which body
// is the TARGET is the modelling order — a feature's own body is always the tool — so the socket and the
// bead are built ball-first and the rest shank-first.
func runBallStud(c *caller) error {
	for _, run := range []func(*caller) error{
		ballStudUnion, ballStudSocket, ballStudStub, ballStudPlug,
		ballAxleUnion, ballAxleBead, ballAxleStubs, ballAxleCore,
		ballShoulderUnion, ballShoulderCut,
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
	ballAxleLen   = 2.5  // cm, the THROUGH rod's full length, centred on the ball
	ballStudTol   = 1e-3 // volume band: tessellation only, once the B-rep is exact
)

// ballStudFaces is the census every ONE-SEAM result carries: the ball's surviving cap, the shank's wall
// and one planar disc.
var ballStudFaces = map[string]int{"sphere": 1, "cylinder": 1, "plane": 1}

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

func ballAxleVolume() float64 {
	rr := ballStudRodD / 2
	return math.Pi * rr * rr * ballAxleLen
}

// ballAxleCoreVolume is what a THROUGH rod shares with the ball: the rod between the two seam planes,
// domed by a spherical cap at each end.
func ballAxleCoreVolume() float64 {
	rb, rr := ballStudBallD/2, ballStudRodD/2
	d := math.Sqrt(rb*rb - rr*rr)
	dome := math.Pi * (rb - d) * (rb - d) * (rb - (rb-d)/3)
	return math.Pi*rr*rr*(2*d) + 2*dome
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
	if err := c.checkBallStud("ball ∪ rod (stud)", union(ballStudBallD), ballStudFaces); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "ball_d", "expression": "14 mm"}, nil)
	return c.checkBallStud("ball ∪ rod (Ø14 head)", union(1.4), ballStudFaces)
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
	return c.checkBallStud("ball − rod (socket)", want, ballStudFaces)
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
		ballStudRodVolume()-ballStudPlugVolume(ballStudBallD, ballStudRodD), ballStudFaces)
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
	return c.checkBallStud("ball ∩ rod (plug)", ballStudPlugVolume(ballStudBallD, ballStudRodD), ballStudFaces)
}

// ballAxleUnion is a ball on a through axle: the ball's BELT between the two seams, plus a stub and a
// tip at each end.
func ballAxleUnion(c *caller) error {
	ballStudDoc(c, "axle-union")
	if err := ballAxleShank(c, 0, "new"); err != nil {
		return err
	}
	if err := ballStudBall(c, 1, "join"); err != nil {
		return err
	}
	return c.checkBallStud("ball ∪ axle", ballStudBallVolume(ballStudBallD)+ballAxleVolume()-ballAxleCoreVolume(),
		map[string]int{"sphere": 1, "cylinder": 2, "plane": 2})
}

// ballAxleBead is ball − axle: a bead, the ball's belt plus one open bore. It is the sharpest shape in
// the family — a genus-1 solid of exactly TWO faces, and no planar disc at all, because the axle's own
// caps are both outside the ball.
func ballAxleBead(c *caller) error {
	ballStudDoc(c, "axle-bead")
	if err := ballStudBall(c, 0, "new"); err != nil {
		return err
	}
	if err := ballAxleShank(c, 1, "cut"); err != nil {
		return err
	}
	return c.checkBallStud("ball − axle (bead)", ballStudBallVolume(ballStudBallD)-ballAxleCoreVolume(),
		map[string]int{"sphere": 1, "cylinder": 1})
}

// ballAxleStubs is axle − ball: the ball severs the axle, so the result is TWO separate stubs, each
// dimpled by the ball's own surface.
func ballAxleStubs(c *caller) error {
	ballStudDoc(c, "axle-stubs")
	if err := ballAxleShank(c, 0, "new"); err != nil {
		return err
	}
	if err := ballStudBall(c, 1, "cut"); err != nil {
		return err
	}
	return c.checkBallStud("axle − ball (two stubs)", ballAxleVolume()-ballAxleCoreVolume(),
		map[string]int{"sphere": 2, "cylinder": 2, "plane": 2})
}

// ballAxleCore is ball ∩ axle: the axle's length inside the ball, domed at both ends.
func ballAxleCore(c *caller) error {
	ballStudDoc(c, "axle-core")
	if err := ballAxleShank(c, 0, "new"); err != nil {
		return err
	}
	if err := ballStudBall(c, 1, "intersect"); err != nil {
		return err
	}
	return c.checkBallStud("ball ∩ axle (core)", ballAxleCoreVolume(),
		map[string]int{"sphere": 2, "cylinder": 1})
}

// ballShoulderUnion is a shank that stops PART WAY through the ball's shoulder — past the seam plane at
// 4 mm but short of the pole at 5 mm. The ball's own surface crosses the shank's end disc there, so the
// union keeps TWO spherical pieces (the ball below the seam, and the tip beyond the shank's end) and the
// end cap survives as an annulus.
func ballShoulderUnion(c *caller) error {
	ballStudDoc(c, "shoulder-union")
	if err := ballShoulderShank(c, 0, "new"); err != nil {
		return err
	}
	if err := ballStudBall(c, 1, "join"); err != nil {
		return err
	}
	want := ballStudBallVolume(ballStudBallD) + ballShoulderRodVolume() - ballShoulderSharedVolume()
	return c.checkBallStud("ball ∪ shoulder rod", want,
		map[string]int{"sphere": 2, "cylinder": 1, "plane": 1})
}

// ballShoulderCut is the ball bored by that same shank: the bore is blind, but its flat bottom is now the
// shank's BURIED cap and its mouth is the annular step the shoulder leaves.
func ballShoulderCut(c *caller) error {
	ballStudDoc(c, "shoulder-cut")
	if err := ballStudBall(c, 0, "new"); err != nil {
		return err
	}
	if err := ballShoulderShank(c, 1, "cut"); err != nil {
		return err
	}
	return c.checkBallStud("ball − shoulder rod",
		ballStudBallVolume(ballStudBallD)-ballShoulderSharedVolume(),
		map[string]int{"sphere": 2, "cylinder": 1, "plane": 2})
}

// ballShoulderStop is where the shoulder shank ends: between the seam plane and the pole.
const ballShoulderStop = 0.45 // cm

func ballShoulderRodVolume() float64 {
	rr := ballStudRodD / 2
	return math.Pi * rr * rr * ballShoulderStop
}

// ballShoulderSharedVolume is what that shank shares with the ball. Inside the ball's own circle at the
// stop station the shank's cap limits the solid; outside it the ball's surface does:
//
//	V = π·ρ²·stop + (2π/3)·((R²−ρ²)^{3/2} − (R²−r_c²)^{3/2}),   ρ = √(R²−stop²)
func ballShoulderSharedVolume() float64 {
	rb, rr := ballStudBallD/2, ballStudRodD/2
	rho2 := rb*rb - ballShoulderStop*ballShoulderStop
	return math.Pi*rho2*ballShoulderStop +
		(2*math.Pi/3)*(math.Pow(rb*rb-rho2, 1.5)-math.Pow(rb*rb-rr*rr, 1.5))
}

// ballShoulderShank extrudes the Ø6 shank from the ball centre out to the shoulder stop.
func ballShoulderShank(c *caller, sketchIndex int, op string) error {
	c.json("create_sketch", map[string]any{"plane": "XZ"}, nil)
	addConstrainedCircle(c, sketchIndex, []float64{0, 0}, "0.3 cm", "shank_d / 2")
	if c.err != nil {
		return c.err
	}
	return c.applyFeature("extrude", map[string]any{
		"sketchIndex": sketchIndex, "profileIndex": 0, "distance": "shoulder_len", "operation": op,
	})
}

// ballStudDoc opens a fresh part carrying the three driving parameters.
func ballStudDoc(c *caller, name string) {
	c.json("close_all_documents", map[string]any{"force": true}, nil)
	c.json("create_document", map[string]any{"type": "part", "name": "ballstud-" + name}, nil)
	for _, p := range [][2]string{{"ball_d", "10 mm"}, {"shank_d", "6 mm"}, {"shank_len", "15 mm"},
		{"axle_len", "25 mm"}, {"shoulder_len", "4.5 mm"}} {
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

// ballAxleShank extrudes the Ø6 axle SYMMETRICALLY about the sketch plane, so it clears the ball at
// both ends and the two surfaces meet in two seam circles.
func ballAxleShank(c *caller, sketchIndex int, op string) error {
	c.json("create_sketch", map[string]any{"plane": "XZ"}, nil)
	addConstrainedCircle(c, sketchIndex, []float64{0, 0}, "0.3 cm", "shank_d / 2")
	if c.err != nil {
		return c.err
	}
	return c.applyFeature("extrude", map[string]any{
		"sketchIndex": sketchIndex, "profileIndex": 0, "distance": "axle_len",
		"operation": op, "direction": "symmetric",
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
// to a tessellation-only band, and the exact analytic face census.
func (c *caller) checkBallStud(tag string, want float64, faces map[string]int) error {
	if err := c.checkVolumeTol(tag, want, ballStudTol); err != nil {
		return err
	}
	return c.checkAnalyticFaces(tag, faces)
}

// checkAnalyticFaces fails unless the active part carries exactly the expected census of analytic
// faces. This is what separates the exact path from the CSG fallback: the fallback's result is hundreds
// of "plane" facets and carries no sphere at all.
func (c *caller) checkAnalyticFaces(tag string, want map[string]int) error {
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
	total := 0
	for _, f := range rk.Bodies[0].Faces {
		kinds[f.Kind]++
		total++
	}
	if !sameFaceCensus(kinds, want) {
		return fmt.Errorf("%s: %d faces %v, want %v "+
			"(a faceted result means the boolean fell back to triangle-soup CSG)", tag, total, kinds, want)
	}
	fmt.Printf("  %-24s exact: %d analytic faces %v\n", tag, total, kinds)
	return nil
}

// sameFaceCensus compares two face-kind tallies.
func sameFaceCensus(got, want map[string]int) bool {
	if len(got) != len(want) {
		return false
	}
	for k, n := range want {
		if got[k] != n {
			return false
		}
	}
	return true
}
