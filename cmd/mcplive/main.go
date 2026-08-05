// SPDX-License-Identifier: GPL-2.0-only

// Command mcplive drives NopSCADlib parts into a *running* oblikovati-mcp-bridge
// host over HTTP/SSE — the live counterpart of the bridge/nopscad_*_test.go suite.
// Each part is modeled the Inventor way: real parameters, a fully-constrained
// (0-DOF) sketch, a feature, and a volume check against the analytic value, plus a
// parametric resize. It surfaces kernel/API gaps against the live C-ABI stack.
//
// Usage: mcplive [--url http://127.0.0.1:7800/mcp] [--part all|washer|spacer|ball|rod]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// part is one live modeling scenario; it returns a short status line or fails.
type part struct {
	name string
	run  func(c *caller) error
}

var parts = []part{
	{"spacer", runSpacer},
	{"ball", runBall},
	{"rod", runRod},
	{"oring", runORing},
	{"hexnut", runHexNut},
	{"spring", runSpring},
	{"tubing", runTubing},
	{"loft", runLoft},
	{"pcb", runPcb},
	{"pulley", runPulley},
	{"revtaper", runRevTaper},
	{"capscrew", runCapScrew},
	{"boxtray", runBoxTray},
	{"leadnut", runLeadnut},
	{"knob", runKnob},
	{"hull", runHull},
	{"standoff", runStandoff},
	{"offset", runOffset},
	{"countersink", runCountersink},
	{"tappedhole", runTappedHole},
	{"starwasher", runStarWasher},
	{"rib", runRib},
	{"thread", runThread},
	{"fixingblock", runFixingBlock},
	{"pcbmount", runPcbMount},
	{"faceplate", runFaceplate},
	{"counterbore", runCounterbore},
	{"boltflange", runBoltFlange},
	{"shaftcoupling", runShaftCoupling},
	{"fanframe", runFanFrame},
	{"veroboard", runVeroboard},
	{"hullunequal", runHullUnequal},
	{"offsetconcave", runOffsetConcave},
	{"benttube", runBentTube},
	{"loftpyramid", runLoftPyramid},
	{"loftpipe", runLoftPipe},
	{"loftcurved", runLoftCurved},
	{"loftpoint", runLoftPoint},
	{"loftface", runLoftFace},
	{"loftrail", runLoftRail},
	{"loftcenterline", runLoftCenterline},
	{"loftareagraph", runLoftAreaGraph},
	{"loftmapcurve", runLoftMapCurve},
	{"draft", runDraft},
	{"emboss", runEmboss},
	{"cornerbracket", runCornerBracket},
	{"semiteardrop", runSemiTeardrop},
	{"lightstripclip", runLightStripClip},
	{"opengrab", runOpengrabTarget},
	{"nuttrap", runNutTrap},
	{"polyhole", runPolyhole},
	{"hanginghole", runHangingHole},
	{"sbrrail", runSbrRail},
	{"smdresistor", runSmdResistor},
	{"variacdial", runVariacDial},
	{"ellipticalcablestrip", runEllipticalCableStrip},
	{"jack", runJack},
	{"picutout", runPiCutout},
	{"ziptie", runZiptie},
	{"ribbongrommet", runRibbonGrommet},
	{"ribbongrommethole", runRibbonGrommetHole},
	{"polyring", runPolyRing},
	{"quadrant", runQuadrant},
	{"roundedcorner", runRoundedCorner},
	{"squarebutton", runSquareButton},
	{"flatflex", runFlatFlex},
	{"hdmi", runHDMI},
	{"dip", runDIP},
	{"idctransition", runIDCTransition},
	{"carriageend", runCarriageEnd},
	{"grubscrewpositions", runGrubScrewPositions},
	{"chequerboard", runChequerboard},
	{"wovensheet", runWovenSheet},
	{"transformer", runTransformer},
	{"doorlatchstl", runDoorLatchStl},
	{"ledbezelretainer", runLedBezelRetainer},
	{"tearslot", runTearslot},
	{"tearslot2d", runTearslot2D},
	{"verticaltearslot", runVerticalTearslot},
	{"verticaltearslot2d", runVerticalTearslot2D},
	{"dimension", runDimension},
	{"wirelink", runWireLink},
	{"e3dfanduct", runE3dFanDuct},
	{"e3dfan", runE3dFan},
	{"beltb", runBeltb},
	{"extrusioncentersection", runExtrusionCenterSection},
	{"mainssocketholes", runMainsSocketHoles},
	{"adjust", runAdjust},
	{"trimpot3362", runTrimpot3362},
	{"radialprofile", runRadialProfile},
	{"rdelectrolytic", runRdElectrolytic},
	{"smddiode", runSmdDiode},
	{"smdtant", runSmdTant},
	{"singlecableclip", runSingleCableClip},
	{"squatrim", runSquatRim},
	{"straddlinghole", runStraddlingHole},
	{"facetdiag", runFacetDiag},
	{"smoothwire", runSmoothWire},
	{"microplate", runMicroPlate},
}

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	which := flag.String("part", "all", "part name or 'all'")
	flag.Parse()
	if err := run(*url, *which); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(url, which string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	// Retry the connect: right after `make install` the host hot-reloads the add-in,
	// leaving a brief window where the MCP server is restarting and refuses connections.
	client := mcp.NewClient(&mcp.Implementation{Name: "mcplive", Version: "0.1.0"}, nil)
	var cs *mcp.ClientSession
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		cs, err = client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("connect %s (after retries): %w", url, err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "mcplive: close session:", closeErr)
		}
	}()

	// Always start fresh: force-close every document the host has open from prior runs.
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "close_all_documents", Arguments: map[string]any{"force": true}}); err != nil {
		return fmt.Errorf("close_all_documents: %w", err)
	}

	for _, p := range parts {
		if which != "all" && which != p.name {
			continue
		}
		c := &caller{ctx: ctx, cs: cs}
		// Unique name per run: the running host keeps documents across client
		// connections, so a fixed name collides on a second run.
		name := fmt.Sprintf("%s-%d", p.name, time.Now().UnixNano()%100000)
		c.json("create_document", map[string]any{"type": "part", "name": name}, &c.doc)
		c.json("activate_document", map[string]any{"id": c.doc.ID}, nil)
		if err := p.run(c); err != nil {
			return fmt.Errorf("[%s] %w", p.name, err)
		}
		fmt.Printf("PASS %s\n", p.name)
	}
	return nil
}

// runSpacer: a tube — two concentric circles extruded. OD 6, ID 3.2, H 10 (mm).
func runSpacer(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "od", "expression": "6 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "id", "expression": "3.2 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "h", "expression": "10 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	outer := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.3 cm"})
	inner := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.16 cm"})
	if c.err != nil {
		return c.err
	}
	if len(outer) < 2 || len(inner) < 2 {
		return fmt.Errorf("circle reply missing center id: outer=%v inner=%v", outer, inner)
	}
	outerE, outerC := outer[0], outer[1]
	innerE, innerC := inner[0], inner[1]

	c.con("ground", outerC)
	c.con("coincident", outerC, innerC)
	c.dim("radius", "od / 2", outerE)
	c.dim("radius", "id / 2", innerE)
	if err := c.requireConstrained(); err != nil {
		return err
	}

	annulus := c.profileWithHole()
	if annulus < 0 {
		return fmt.Errorf("no annular profile found (expected outer ring with inner hole)")
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": annulus, "distance": "h"}); err != nil {
		return err
	}

	want := func(odMM, idMM, hMM float64) float64 {
		R, r, h := odMM/20, idMM/20, hMM/10
		return math.Pi * (R*R - r*r) * h
	}
	if err := c.checkVolume("od=6", want(6, 3.2, 10)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "od", "expression": "8 mm"}, nil)
	return c.checkVolume("od=8 (resized)", want(8, 3.2, 10))
}

// runBall: a sphere by revolving a half-disk (diameter line + semicircular arc) about
// the Y axis. Fully constrained via ground + midpoint + a diameter dimension. d 5 mm.
func runBall(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "d", "expression": "5 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	line := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0.25}, {0, -0.25}}})
	arc := c.ids(map[string]any{"sketchIndex": 0, "kind": "arc", "points": [][]float64{{0, 0}, {0, 0.25}, {0, -0.25}}, "ccw": false})
	if c.err != nil {
		return c.err
	}
	if len(o) < 1 || len(line) < 3 || len(arc) < 4 {
		return fmt.Errorf("entity reply too short: o=%v line=%v arc=%v", o, line, arc)
	}
	oID, lineE, top, bot := o[0], line[0], line[1], line[2]
	arcCenter, arcStart, arcEnd := arc[1], arc[2], arc[3]

	c.con("ground", oID)
	c.con("coincident", arcCenter, oID)
	c.con("coincident", arcStart, top)
	c.con("coincident", arcEnd, bot)
	c.con("vertical", top, bot)
	c.con("midpoint", oID, lineE)
	c.dim("distance", "d", top, bot)
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); err != nil {
		return err
	}

	want := func(dMM float64) float64 { dc := dMM / 10; return math.Pi * dc * dc * dc / 6 }
	if err := c.checkVolume("d=5", want(5)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "d", "expression": "8 mm"}, nil)
	return c.checkVolume("d=8 (resized)", want(8))
}

// runRod: a cylinder with 45° chamfered ends — extrude a parametric circle, then
// chamfer the two cap rings (edges picked by their Z). d 6 mm, l 20 mm.
func runRod(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "d", "expression": "6 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "l", "expression": "20 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	circle := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.3 cm"})
	if c.err != nil {
		return c.err
	}
	if len(circle) < 2 {
		return fmt.Errorf("circle reply missing center: %v", circle)
	}
	c.con("ground", circle[1])
	c.dim("radius", "d / 2", circle[0])
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "l", "operation": "new"}); err != nil {
		return err
	}

	ends := c.edgesNearZ([]float64{0, 2.0}, 1e-3) // caps at z=0 and z=l(=2cm)
	if len(ends) == 0 {
		return fmt.Errorf("no end-ring edges found to chamfer")
	}
	if err := c.applyFeature("chamfer", map[string]any{"edgeRefs": ends, "distance": "0.6 mm"}); err != nil {
		return err
	}
	return c.checkVolume("rod 6x20", 0.55813) // OpenSCAD golden rod(6,20)
}

// edgesNearZ returns reference keys of edges whose representative point's Z is within
// tol of any given z — used to select cap-ring edges for the chamfer.
func (c *caller) edgesNearZ(zs []float64, tol float64) []string {
	var rk struct {
		Bodies []struct {
			Edges []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"edges"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	var keys []string
	for _, b := range rk.Bodies {
		for _, e := range b.Edges {
			if len(e.Point) != 3 {
				continue
			}
			for _, z := range zs {
				if math.Abs(e.Point[2]-z) <= tol {
					keys = append(keys, e.Key)
					break
				}
			}
		}
	}
	return keys
}

// runORing: a torus (O-ring) by revolving a circular section offset from the axis.
// id 20 mm, minor 3 mm; centreline R = id/2 + minor/4, section r = minor/2.
func runORing(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "id", "expression": "20 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "minor", "expression": "3 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	circle := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{1.075, 0}}, "radius": "0.15 cm"})
	if c.err != nil {
		return c.err
	}
	if len(o) < 1 || len(circle) < 2 {
		return fmt.Errorf("entity reply too short: o=%v circle=%v", o, circle)
	}
	c.con("ground", o[0])
	c.con("horizontal", o[0], circle[1])
	c.dim("distance", "id/2 + minor/4", o[0], circle[1])
	c.dim("radius", "minor/2", circle[0])
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); err != nil {
		return err
	}
	want := func(idMM, minorMM float64) float64 {
		r := (minorMM / 2) / 10
		R := (idMM/2 + minorMM/4) / 10
		return 2 * math.Pi * math.Pi * R * r * r
	}
	if err := c.checkVolume("id20 minor3", want(20, 3)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "minor", "expression": "4 mm"}, nil)
	return c.checkVolume("minor4 (resized)", want(20, 4))
}

// runHexNut: a regular hexagonal prism (across-flats af) with a central through hole.
// Exercises the regular-polygon auto-constraints + a hole-aware extrude. af 10mm,
// hole 5mm, th 5mm.
func runHexNut(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "af", "expression": "10 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "hole", "expression": "5 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "th", "expression": "5 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	poly := c.ids(map[string]any{"sketchIndex": 0, "kind": "polygon", "points": [][]float64{{0, 0}, {0.57735, 0}}, "sides": 6})
	circle := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.25 cm"})
	if c.err != nil {
		return c.err
	}
	if len(poly) < 8 || len(circle) < 2 {
		return fmt.Errorf("entity reply too short: poly=%v circle=%v", poly, circle)
	}
	edge0, v0, center := poly[0], poly[1], poly[len(poly)-1]

	c.con("ground", center)
	c.con("horizontal", center, v0)
	c.dim("offsetDim", "af/2", center, edge0)
	c.con("coincident", circle[1], center)
	c.dim("radius", "hole/2", circle[0])
	if err := c.requireConstrained(); err != nil {
		return err
	}

	annulus := c.profileWithHole()
	if annulus < 0 {
		return fmt.Errorf("no hex-with-hole profile found")
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": annulus, "distance": "th"}); err != nil {
		return err
	}
	want := func(afMM, holeMM, thMM float64) float64 {
		af, hole, th := afMM/10, holeMM/10, thMM/10
		return (math.Sqrt(3)/2*af*af - math.Pi*(hole/2)*(hole/2)) * th
	}
	if err := c.checkVolume("af10 hole5", want(10, 5, 5)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "af", "expression": "13 mm"}, nil)
	return c.checkVolume("af13 (resized)", want(13, 5, 5))
}

// runSpring: a round wire coiled helically about Z (compression spring). R 5mm mean,
// wire 1mm, pitch 3mm, 5 turns. Exercises the coil feature with parametric pitch/turns.
func runSpring(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "R", "expression": "5 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "wire", "expression": "1 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "pitch", "expression": "3 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "revs", "expression": "5"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XZ"}, nil)

	o := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})
	circle := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0.5, 0}}, "radius": "0.1 cm"})
	if c.err != nil {
		return c.err
	}
	if len(o) < 1 || len(circle) < 2 {
		return fmt.Errorf("entity reply too short: o=%v circle=%v", o, circle)
	}
	c.con("ground", o[0])
	c.con("horizontal", o[0], circle[1])
	c.dim("distance", "R", o[0], circle[1])
	c.dim("radius", "wire", circle[0])
	if err := c.requireConstrained(); err != nil {
		return err
	}
	if err := c.applyFeature("coil", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/z", "pitch": "pitch", "revolutions": "revs",
	}); err != nil {
		return err
	}
	want := func(Rmm, wireMM, pitchMM, revs float64) float64 {
		R, r, p := Rmm/10, wireMM/10, pitchMM/10
		circumference := 2 * math.Pi * R
		l := revs * math.Sqrt(circumference*circumference+p*p)
		return math.Pi * r * r * l
	}
	if err := c.checkVolume("5turns", want(5, 1, 3, 5)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "revs", "expression": "8"}, nil)
	return c.checkVolume("8turns (resized)", want(5, 1, 3, 8))
}

// runTubing: a circular section swept along a straight +Z rail (a tube). Two
// fully-constrained sketches (profile on XY, rail on XZ). r 2mm, len 20mm.
func runTubing(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "r", "expression": "2 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "len", "expression": "20 mm"}, nil)

	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	prof := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.2 cm"})
	if c.err != nil || len(prof) < 2 {
		return fmt.Errorf("profile reply: %v (%v)", prof, c.err)
	}
	c.con("ground", prof[1])
	c.dim("radius", "r", prof[0])
	if err := c.requireConstrainedAt(0); err != nil {
		return err
	}

	c.json("create_sketch", map[string]any{"plane": "XZ"}, nil)
	path := c.ids(map[string]any{"sketchIndex": 1, "kind": "line", "points": [][]float64{{0, 0}, {0, 2}}})
	if c.err != nil || len(path) < 3 {
		return fmt.Errorf("path reply: %v (%v)", path, c.err)
	}
	callCon := func(kind string, ents ...uint64) {
		c.json("add_sketch_constraint", map[string]any{"sketchIndex": 1, "kind": kind, "entities": ents}, nil)
	}
	callCon("ground", path[1])
	callCon("vertical", path[1], path[2])
	c.json("add_sketch_dimension", map[string]any{"sketchIndex": 1, "kind": "distance", "entities": []uint64{path[1], path[2]}, "expression": "len"}, nil)
	if err := c.requireConstrainedAt(1); err != nil {
		return err
	}

	if err := c.applyFeature("sweep", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "pathSketchIndex": 1, "pathIndex": 0,
	}); err != nil {
		return err
	}
	want := func(rMM, lenMM float64) float64 { rc, lc := rMM/10, lenMM/10; return math.Pi * rc * rc * lc }
	if err := c.checkVolume("len20", want(2, 20)); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "len", "expression": "30 mm"}, nil)
	return c.checkVolume("len30 (resized)", want(2, 30))
}

// requireConstrainedAt asserts the sketch at idx solves to 0 DOF.
func (c *caller) requireConstrainedAt(idx int) error {
	var s struct {
		DOF int `json:"dof"`
	}
	c.json("solve_sketch", map[string]any{"sketchIndex": idx}, &s)
	if c.err != nil {
		return c.err
	}
	fmt.Printf("  sketch %d DOF = %d\n", idx, s.DOF)
	if s.DOF != 0 {
		return fmt.Errorf("sketch %d not fully constrained: dof=%d", idx, s.DOF)
	}
	return nil
}

// runLoft: a conical frustum lofted between two circles on parallel planes (offset
// work plane). Parametric radii; literal height. r1 10mm, r2 5mm, h 15mm.
func runLoft(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "r1", "expression": "10 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "r2", "expression": "5 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "h", "expression": "15 mm"}, nil)

	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "1 cm"})
	if c.err != nil || len(c0) < 2 {
		return fmt.Errorf("bottom circle reply: %v (%v)", c0, c.err)
	}
	c.con("ground", c0[1])
	c.dim("radius", "r1", c0[0])
	if err := c.requireConstrainedAt(0); err != nil {
		return err
	}

	var wp struct {
		Index int `json:"index"`
	}
	c.json("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "h"}, &wp)
	var sk1 struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk1)
	c1 := c.ids(map[string]any{"sketchIndex": sk1.SketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.5 cm"})
	if c.err != nil || len(c1) < 2 {
		return fmt.Errorf("top circle reply: %v (%v)", c1, c.err)
	}
	c.json("add_sketch_constraint", map[string]any{"sketchIndex": sk1.SketchIndex, "kind": "ground", "entities": []uint64{c1[1]}}, nil)
	c.dimAt(sk1.SketchIndex, "radius", "r2", c1[0])
	if err := c.requireConstrainedAt(sk1.SketchIndex); err != nil {
		return err
	}

	if err := c.applyFeature("loft", map[string]any{"sections": []map[string]any{
		{"sketchIndex": 0, "profileIndex": 0},
		{"sketchIndex": sk1.SketchIndex, "profileIndex": 0},
	}}); err != nil {
		return err
	}
	want := func(r1MM, r2MM, hMM float64) float64 {
		r1, r2, h := r1MM/10, r2MM/10, hMM/10
		return math.Pi * h / 3 * (r1*r1 + r1*r2 + r2*r2)
	}
	if err := c.checkVolume("h=15", want(10, 5, 15)); err != nil {
		return err
	}
	// Resize the height: the work plane moves and the top sketch tracks it.
	c.json("set_parameter", map[string]any{"name": "h", "expression": "25 mm"}, nil)
	return c.checkVolume("h=25 (taller)", want(10, 5, 25))
}

// dimAt adds a dimension to a specific sketch index.
func (c *caller) dimAt(idx int, kind, expr string, ents ...uint64) {
	c.json("add_sketch_dimension", map[string]any{"sketchIndex": idx, "kind": kind, "entities": ents, "expression": expr}, nil)
}

// runPcb: a board with a parameter-spaced row of mounting holes. Resizing the board
// length re-spaces the holes (the clone tracks L − m). Proves parametric pattern spacing.
func runPcb(c *caller) error {
	for _, p := range [][2]string{{"L", "40 mm"}, {"W", "30 mm"}, {"m", "4 mm"}, {"hr", "1.5 mm"}, {"th", "1.6 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)

	var rect struct {
		EntityIDs []uint64 `json:"entityIds"`
		PointIDs  []uint64 `json:"pointIds"`
	}
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0, 0}, {4, 3}}}, &rect)
	if c.err != nil || len(rect.EntityIDs) < 4 || len(rect.PointIDs) < 4 {
		return fmt.Errorf("rectangle reply: lines=%v pts=%v (%v)", rect.EntityIDs, rect.PointIDs, c.err)
	}
	bottom, left := rect.EntityIDs[0], rect.EntityIDs[3]
	bl, br, tr, tl := rect.PointIDs[0], rect.PointIDs[1], rect.PointIDs[2], rect.PointIDs[3]

	c.con("ground", bl)
	c.con("horizontal", bl, br)
	c.con("horizontal", tl, tr)
	c.con("vertical", bl, tl)
	c.con("vertical", br, tr)
	c.dim("distance", "L", bl, br)
	c.dim("distance", "W", bl, tl)

	seed := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0.4, 0.4}}, "radius": "0.15 cm"})
	if c.err != nil || len(seed) < 2 {
		return fmt.Errorf("seed circle: %v (%v)", seed, c.err)
	}
	c.dim("offsetDim", "m", seed[1], bottom)
	c.dim("offsetDim", "m", seed[1], left)
	c.dim("radius", "hr", seed[0])
	c.json("add_sketch_pattern", map[string]any{
		"sketchIndex": 0, "kind": "rectangular", "entities": []uint64{seed[0]},
		"count1": 2, "spacing1": "L - 2*m", "dir1": []float64{1, 0},
		"count2": 1, "spacing2": "1 mm", "dir2": []float64{0, 1},
	}, nil)
	if err := c.requireConstrained(); err != nil {
		return err
	}

	fmt.Printf("  clone hole X @ L=40mm: %.3f cm (want 3.6)\n", c.cloneHoleX())
	c.json("set_parameter", map[string]any{"name": "L", "expression": "60 mm"}, nil)
	x := c.cloneHoleX()
	fmt.Printf("  clone hole X @ L=60mm: %.3f cm (want 5.6)\n", x)
	if math.Abs(x-5.6) > 0.02 {
		return fmt.Errorf("clone hole X = %.3f, want 5.6 (spacing did not track L)", x)
	}

	annulus := c.profileWithHole()
	if annulus < 0 {
		return fmt.Errorf("no board-with-holes profile")
	}
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": annulus, "distance": "th"}); err != nil {
		return err
	}
	return c.checkVolume("pcb 60x30 2 holes", (6.0*3.0-2*math.Pi*0.15*0.15)*0.16)
}

// cloneHoleX returns the larger X of the two circle centres (the patterned clone).
func (c *caller) cloneHoleX() float64 {
	c.json("solve_sketch", map[string]any{"sketchIndex": 0}, nil)
	var ents struct {
		Entities []struct {
			Kind   string      `json:"kind"`
			Points [][]float64 `json:"points"`
		} `json:"entities"`
	}
	c.json("list_sketch_entities", map[string]any{"sketchIndex": 0}, &ents)
	max := -1e9
	for _, e := range ents.Entities {
		if e.Kind == "circle" && len(e.Points) == 1 && len(e.Points[0]) == 2 && e.Points[0][0] > max {
			max = e.Points[0][0]
		}
	}
	return max
}

// ---- live MCP caller helpers ----

type caller struct {
	ctx context.Context
	cs  *mcp.ClientSession
	err error
	doc struct {
		ID uint64 `json:"id"`
	}
}

func (c *caller) json(name string, args map[string]any, v any) {
	if c.err != nil {
		return
	}
	res, err := c.cs.CallTool(c.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		c.err = fmt.Errorf("%s: %w", name, err)
		return
	}
	if res.IsError {
		c.err = fmt.Errorf("%s: %s", name, firstText(res))
		return
	}
	if v == nil {
		return
	}
	// Action tools (add_sketch_entity, get_physical_properties, solve_sketch) return
	// their JSON as text; summarized list_* tools put a human digest in text and the
	// real JSON in StructuredContent. Try text-as-JSON first; if it isn't valid JSON
	// (a digest), fall back to the structured payload.
	if text := firstText(res); text != "" {
		if err := json.Unmarshal([]byte(text), v); err == nil {
			return
		}
	}
	if res.StructuredContent != nil {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			c.err = fmt.Errorf("%s: marshal structured: %w", name, err)
			return
		}
		if err := json.Unmarshal(raw, v); err != nil {
			c.err = fmt.Errorf("%s: decode structured %q: %w", name, raw, err)
		}
	}
}

func (c *caller) con(kind string, ents ...uint64) {
	c.json("add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
}

func (c *caller) dim(kind, expr string, ents ...uint64) {
	c.json("add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
}

func (c *caller) ids(args map[string]any) []uint64 {
	var r struct {
		EntityID  uint64   `json:"entityId"`
		PointIDs  []uint64 `json:"pointIds"`
		EntityIDs []uint64 `json:"entityIds"`
	}
	c.json("add_sketch_entity", args, &r)
	if len(r.PointIDs) > 0 {
		return append([]uint64{r.EntityID}, r.PointIDs...)
	}
	return append([]uint64{r.EntityID}, r.EntityIDs...)
}

func (c *caller) requireConstrained() error {
	var s struct {
		DOF int `json:"dof"`
	}
	c.json("solve_sketch", map[string]any{"sketchIndex": 0}, &s)
	if c.err != nil {
		return c.err
	}
	fmt.Printf("  sketch DOF = %d\n", s.DOF)
	if s.DOF != 0 {
		return fmt.Errorf("sketch not fully constrained: dof=%d", s.DOF)
	}
	return nil
}

func (c *caller) profileWithHole() int {
	var p struct {
		Profiles []struct {
			Index int `json:"index"`
			Holes int `json:"holes"`
		} `json:"profiles"`
	}
	c.json("list_sketch_profiles", map[string]any{"sketchIndex": 0}, &p)
	for _, pr := range p.Profiles {
		if pr.Holes > 0 {
			return pr.Index
		}
	}
	return -1
}

func (c *caller) applyFeature(kind string, args map[string]any) error {
	var r struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	c.json("add_feature", map[string]any{"kind": kind, "args": args}, &r)
	if c.err != nil {
		return c.err
	}
	if !r.Healthy {
		return fmt.Errorf("%s unhealthy: %s", kind, r.Reason)
	}
	return nil
}

func (c *caller) checkVolume(tag string, want float64) error {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	c.json("get_physical_properties", nil, &pp)
	if c.err != nil {
		return c.err
	}
	rel := math.Abs(pp.Volume-want) / want
	fmt.Printf("  %-18s volume = %.6f cm^3  want ~%.6f  (rel %.4f)\n", tag, pp.Volume, want, rel)
	if rel > 0.02 {
		return fmt.Errorf("%s volume off by %.2f%%", tag, rel*100)
	}
	return nil
}

func firstText(res *mcp.CallToolResult) string {
	for _, ct := range res.Content {
		if tc, ok := ct.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
