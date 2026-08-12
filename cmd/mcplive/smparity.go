// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// Live drivers for the M44 sheet-metal parity closeout (#1964 corner seam, #1965 rip, #1967 corner
// chamfer/round variants, #1968 punch tool). The pure-Go suites measure the geometry exactly; these
// prove the same paths survive the real C-ABI stack and render a valid solid, and leave a screenshot
// for a human to eyeball. Every check measures against the BARE SHEET this run built.

// smThickness derives the sheet gauge from the bare 40x30 mm plate's volume (area 12 cm²), so a
// rule change moves the thresholds with it instead of silently invalidating a hard-coded gauge.
func smThickness(base float64) float64 { return base / (4.0 * 3.0) }

// smBottomFaceKey returns the reference key of the plate's lower large face (smallest-Z locating
// point) — the RipFace a face-extents rip acts on.
func smBottomFaceKey(c *caller) (string, error) {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	if c.err != nil || len(rk.Bodies) == 0 {
		return "", fmt.Errorf("no bodies to read faces from (%v)", c.err)
	}
	best, bestZ := "", math.Inf(1)
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) == 3 && f.Point[2] < bestZ {
			best, bestZ = f.Key, f.Point[2]
		}
	}
	if best == "" {
		return "", fmt.Errorf("plate has no locatable face")
	}
	return best, nil
}

// smCornerVerticalEdge returns a through-thickness corner edge (its midpoint sits between the two
// faces in Z, unlike a top/bottom edge) at the +X/+Y corner — what a corner seam or chamfer needs.
func smCornerVerticalEdge(c *caller, thickness float64) (string, error) {
	// smEdge already carries Key + Point; reuse the sheetmetal.go census shape.
	var keys struct {
		Bodies []struct {
			Edges []smEdge `json:"edges"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &keys)
	if c.err != nil || len(keys.Bodies) == 0 {
		return "", fmt.Errorf("no bodies to read edges from (%v)", c.err)
	}
	best, bestSum := "", math.Inf(-1)
	for _, e := range keys.Bodies[0].Edges {
		if len(e.Point) != 3 || e.Point[2] <= 1e-6 || e.Point[2] >= thickness-1e-6 {
			continue // a top/bottom edge sits on a face, not through the thickness
		}
		if s := e.Point[0] + e.Point[1]; s > bestSum {
			best, bestSum = e.Key, s
		}
	}
	if best == "" {
		return "", fmt.Errorf("plate has no through-thickness corner edge")
	}
	return best, nil
}

// runSheetMetalRip drives #1965 live: a face-extents rip slits the plate along its full length,
// removing gap·length·thickness of material and leaving a still-valid solid (a C-channel of one
// body, two shells). Captured for a look at the open seam.
func runSheetMetalRip(c *caller) error {
	base, err := smSheet(c, "smrip")
	if err != nil {
		return err
	}
	face, err := smBottomFaceKey(c)
	if err != nil {
		return err
	}
	if err := c.applyFeature("sheetMetalRip", map[string]any{
		"faceKey": face, "type": "faceExtents", "gap": "2 mm", "gapSide": "symmetric",
	}); err != nil {
		return err
	}
	removed := base - smVolume(c)
	t := smThickness(base)
	want := 0.2 * 4.0 * t // gap 0.2 cm × the 4 cm long axis × gauge
	fmt.Printf("  rip removed %.6f cm^3, want ~%.6f (gap·4cm·gauge)\n", removed, want)
	if math.Abs(removed-want) > 0.02 {
		return fmt.Errorf("face-extents rip removed %.6f, want ~%.6f", removed, want)
	}
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-smrip.png"}, nil)
	return smValidSolid(c, "smrip")
}

// runSheetMetalPunchTool drives #1968 live: a rotated punch turns the die about its centroid, so
// the cut hole is turned but removes the SAME area as the un-rotated one. Captured to see the turn.
func runSheetMetalPunchTool(c *caller) error {
	flat, err := smPunchRemoved(c, "smpunch-flat", "0 deg")
	if err != nil {
		return err
	}
	turned, err := smPunchRemoved(c, "smpunch-turned", "40 deg")
	if err != nil {
		return err
	}
	fmt.Printf("  punch removed flat = %.6f cm^3, turned 40° = %.6f cm^3\n", flat, turned)
	if math.Abs(flat-turned) > 1e-4 {
		return fmt.Errorf("a rotated punch removed %.6f, the un-rotated one %.6f; a turn preserves area", turned, flat)
	}
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-smpunch.png"}, nil)
	return nil
}

// smPunchRemoved punches one 20x8 mm slot at the given rotation and returns the material removed.
func smPunchRemoved(c *caller, name, angle string) (float64, error) {
	base, err := smSheet(c, name)
	if err != nil {
		return 0, err
	}
	// A 20×8 mm slot centred on the plate (which spans [0,4]×[0,3] cm), so a rotation about its
	// centroid keeps every corner inside the material (half-diagonal 1.08 cm < 1.5 cm to the edge)
	// and the through-all cut removes the whole die — the area-preserving case #1968 asserts.
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "polyline", "closed": true,
		"points": [][]float64{{1.0, 1.1}, {3.0, 1.1}, {3.0, 1.9}, {1.0, 1.9}}}, nil)
	if err := c.applyFeature("sheetMetalPunch", map[string]any{"sketchIndex": 1, "angle": angle}); err != nil {
		return 0, err
	}
	removed := base - smVolume(c)
	if c.err != nil {
		return 0, c.err
	}
	if removed <= 0 {
		return 0, fmt.Errorf("the %s punch removed no material (%.6f)", name, removed)
	}
	return removed, smValidSolid(c, name)
}

// runSheetMetalCornerVariants drives #1967 (a distance-and-angle corner chamfer) and #1964 (a gap
// corner seam) live on the plate's through-thickness corner edge, each leaving a valid solid.
func runSheetMetalCornerVariants(c *caller) error {
	base, err := smSheet(c, "smcorner")
	if err != nil {
		return err
	}
	edge, err := smCornerVerticalEdge(c, smThickness(base))
	if err != nil {
		return err
	}
	if err := c.applyFeature("sheetMetalCorner", map[string]any{
		"edges": []string{edge}, "treatment": "chamfer", "size": "6 mm", "chamferType": "distanceAndAngle", "angle": "30 deg",
	}); err != nil {
		return err
	}
	removed := base - smVolume(c)
	fmt.Printf("  distance-and-angle chamfer removed %.6f cm^3\n", removed)
	if removed <= 0 {
		return fmt.Errorf("the distance-and-angle chamfer removed no material (%.6f)", removed)
	}
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-smcorner.png"}, nil)
	return smValidSolid(c, "smcorner")
}

// runSheetMetalCornerSeam drives #1964 live: a gap corner seam cuts a square relief notch at the
// plate's through-thickness corner edge, removing gap²·thickness and leaving a valid solid.
func runSheetMetalCornerSeam(c *caller) error {
	base, err := smSheet(c, "smseam")
	if err != nil {
		return err
	}
	edge, err := smCornerVerticalEdge(c, smThickness(base))
	if err != nil {
		return err
	}
	if err := c.applyFeature("sheetMetalCornerSeam", map[string]any{"edges": []string{edge}, "gap": "5 mm"}); err != nil {
		return err
	}
	removed := base - smVolume(c)
	want := 0.5 * 0.5 * smThickness(base) // gap² · gauge
	fmt.Printf("  gap corner seam removed %.6f cm^3, want ~%.6f (gap²·gauge)\n", removed, want)
	if math.Abs(removed-want) > 0.01 {
		return fmt.Errorf("gap corner seam removed %.6f, want ~%.6f", removed, want)
	}
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-smseam.png"}, nil)
	return smValidSolid(c, "smseam")
}

// runSheetMetalLoftedFlange drives #1966 live: a die-formed transition between two offset open
// profiles is a smooth (finely sampled) wall, and a press-brake output of the SAME transition is
// faceted — visibly and countably coarser (fewer faces). Both must be valid solids.
func runSheetMetalLoftedFlange(c *caller) error {
	die, err := smLoftedFaceCount(c, "smloft-die", "dieFormed", "")
	if err != nil {
		return err
	}
	press, err := smLoftedFaceCount(c, "smloft-press", "pressBrakeFacetDistance", "8 mm")
	if err != nil {
		return err
	}
	fmt.Printf("  lofted flange faces: die-formed = %d, press-brake = %d\n", die, press)
	if press >= die {
		return fmt.Errorf("press-brake wall has %d faces, die-formed %d; press-brake must be coarser", press, die)
	}
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-smloft.png"}, nil)
	return nil
}

// smLoftedFaceCount builds a lofted flange between two offset L-profiles at the given output type
// and returns the wall's face count (proxy for how finely the transition is sampled).
func smLoftedFaceCount(c *caller, name, output, tol string) (int, error) {
	var doc struct {
		ID uint64 `json:"id"`
	}
	c.json("create_document", map[string]any{"type": "part", "name": name, "subType": "org.oblikovati.part.sheetMetal"}, &doc)
	c.json("activate_document", map[string]any{"id": doc.ID}, nil)
	// Two open L-profiles of matching vertex count, offset in-plane between two parallel planes, so
	// the die-formed transition genuinely curves (a straight ruling could not be faceted).
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": false,
		"points": [][]float64{{0, 0}, {1, 0}, {1, 1}}}, nil)
	var wp struct {
		Index int `json:"index"`
	}
	c.json("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "30 mm"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "polyline", "closed": false,
		"points": [][]float64{{1, 1}, {3, 1}, {3, 3}}}, nil)
	args := map[string]any{"profileA": 0, "profileB": sk.SketchIndex, "outputType": output}
	if tol != "" {
		args["facetTolerance"] = tol
	}
	if err := c.applyFeature("sheetMetalLoftedFlange", args); err != nil {
		return 0, err
	}
	if err := smValidSolid(c, name); err != nil {
		return 0, err
	}
	return smFaceCount(c)
}

// smFaceCount reports how many faces the active part's first body carries.
func smFaceCount(c *caller) (int, error) {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key string `json:"key"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	if c.err != nil || len(rk.Bodies) == 0 {
		return 0, fmt.Errorf("no body to count faces on (%v)", c.err)
	}
	return len(rk.Bodies[0].Faces), nil
}
