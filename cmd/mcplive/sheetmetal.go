// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

// Live drivers for the M44 sheet-metal parity tranche (#1956 hems, #1957 bend position and
// height datum, #1958 width extents, #1960/#2072 relief geometry, #1961 auto-miter and the
// contour flange, #1963 flat punches). The pure-Go suites prove the geometry; these prove the
// same paths survive the real C-ABI stack, where the args cross a JSON schema first.
//
// Every expectation is measured against the BARE SHEET this run built, never against a gauge
// written in here — a driver that hard-codes the thickness keeps passing when the rule changes
// underneath it.

// smSheet creates a sheet-metal document with a 40x30 mm base face and returns the bare
// sheet's volume, which is the baseline every wall measurement subtracts.
func smSheet(c *caller, name string) (float64, error) {
	var doc struct {
		ID uint64 `json:"id"`
	}
	c.json("create_document", map[string]any{
		"type": "part", "name": name, "subType": "org.oblikovati.part.sheetMetal",
	}, &doc)
	c.json("activate_document", map[string]any{"id": doc.ID}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	if err := c.applyFeature("sheetMetalFace", map[string]any{"sketchIndex": 0}); err != nil {
		return 0, err
	}
	base := smVolume(c)
	if c.err != nil {
		return 0, c.err
	}
	if base <= 0 {
		return 0, fmt.Errorf("the base sheet has no volume (%.6f cm^3)", base)
	}
	return base, nil
}

// smEdge is one edge of the sheet with the point that locates it.
type smEdge struct {
	Key   string    `json:"key"`
	Point []float64 `json:"point"`
}

// smTopEdges returns the two adjacent top-face edges of the base sheet: the one running along
// X (at max Y) and the one running along Y (at max X). They share a corner, which is what the
// miter and the corner relief need.
func smTopEdges(c *caller) (alongX, alongY string, err error) {
	var rk struct {
		Bodies []struct{ Edges []smEdge } `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	if c.err != nil || len(rk.Bodies) == 0 {
		return "", "", fmt.Errorf("no bodies to read edges from (%v)", c.err)
	}
	top := smHighestZ(rk.Bodies[0].Edges)
	alongX = smFarthest(top, 1)
	alongY = smFarthest(top, 0)
	if alongX == "" || alongY == "" {
		return "", "", fmt.Errorf("sheet has no pair of adjacent top edges (%d candidates)", len(top))
	}
	return alongX, alongY, nil
}

// smHighestZ keeps the edges sitting on the sheet's top face.
func smHighestZ(edges []smEdge) []smEdge {
	best := math.Inf(-1)
	for _, e := range edges {
		if len(e.Point) == 3 && e.Point[2] > best {
			best = e.Point[2]
		}
	}
	var top []smEdge
	for _, e := range edges {
		if len(e.Point) == 3 && math.Abs(e.Point[2]-best) < 1e-6 {
			top = append(top, e)
		}
	}
	return top
}

// smFarthest picks the edge whose locating point is farthest along the given axis.
func smFarthest(edges []smEdge, axis int) string {
	key, best := "", math.Inf(-1)
	for _, e := range edges {
		if e.Point[axis] > best {
			key, best = e.Key, e.Point[axis]
		}
	}
	return key
}

// smVolume reports the active part's volume.
func smVolume(c *caller) float64 {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	c.json("get_physical_properties", nil, &pp)
	return pp.Volume
}

// runSheetMetalFlange drives #1957 and #1958 live: the height datum decides what the height is
// measured FROM and the width extent decides how much of the edge the wall covers, so a wall
// asked for over half the edge has to come out near half the material of the full one.
func runSheetMetalFlange(c *caller) error {
	full, err := smWallVolume(c, "smflange-full", nil, "tangent")
	if err != nil {
		return err
	}
	half, err := smWallVolume(c, "smflange-half",
		map[string]any{"type": "centered", "width": "20 mm"}, "tangent")
	if err != nil {
		return err
	}
	fmt.Printf("  wall full = %.6f cm^3, half-width = %.6f cm^3 (ratio %.4f)\n", full, half, half/full)
	if r := half / full; math.Abs(r-0.5) > 0.02 {
		return fmt.Errorf("a 20 mm wall on a 40 mm edge is %.4f of the full wall, want ~0.5", r)
	}
	return smHeightDatumShortens(c)
}

// smWallVolume folds one 10 mm flange under the given width extent and height datum, and
// returns the material the WALL added — the part's volume less the sheet it started from.
func smWallVolume(c *caller, name string, width map[string]any, datum string) (float64, error) {
	base, err := smSheet(c, name)
	if err != nil {
		return 0, err
	}
	edge, _, err := smTopEdges(c)
	if err != nil {
		return 0, err
	}
	args := map[string]any{"edge": edge, "height": "10 mm", "radius": "2 mm", "heightDatum": datum}
	if width != nil {
		args["width"] = width
	}
	if err := c.applyFeature("sheetMetalFlange", args); err != nil {
		return 0, err
	}
	wall := smVolume(c) - base
	if c.err != nil {
		return 0, c.err
	}
	if wall <= 0 {
		return 0, fmt.Errorf("the %s flange added no material (%.6f cm^3)", name, wall)
	}
	return wall, nil
}

// smHeightDatumShortens checks the datum moves the wall (#1957): "outer" measures the height
// from the sharp corner the outer faces would make, which is FARTHER out than where the bend
// ends, so the same 10 mm buys a shorter wall than "tangent" does.
func smHeightDatumShortens(c *caller) error {
	tangent, err := smWallVolume(c, "smflange-tangent", nil, "tangent")
	if err != nil {
		return err
	}
	outer, err := smWallVolume(c, "smflange-outer", nil, "outer")
	if err != nil {
		return err
	}
	fmt.Printf("  wall by datum: tangent = %.6f cm^3, outer = %.6f cm^3\n", tangent, outer)
	if !(outer < tangent) {
		return fmt.Errorf("the outer height datum did not shorten the wall: %.6f vs %.6f", outer, tangent)
	}
	return nil
}

// runSheetMetalMiter drives #1961 live. Two walls folding away from one corner each stop at
// their own bend line, so the corner between them is OPEN. Auto-mitering fills it, and the
// miter gap cuts the two extensions apart again so the part can fold.
func runSheetMetalMiter(c *caller) error {
	open, err := smCornerVolume(c, "smmiter-open", false, "")
	if err != nil {
		return err
	}
	// 0.5 mm is the gap that found Oblikovati#2075: the miter-gap cut face abuts the wall it cuts,
	// and the self-intersection check called that contact an interpenetration. Keep this exact
	// value — a wider gap does not reproduce it, which is how the pure-Go fixtures missed it.
	narrow, err := smCornerVolume(c, "smmiter-narrow", true, "0.5 mm")
	if err != nil {
		return err
	}
	wide, err := smCornerVolume(c, "smmiter-wide", true, "3 mm")
	if err != nil {
		return err
	}
	fmt.Printf("  corner open = %.6f, 0.5 mm gap = %.6f, 3 mm gap = %.6f cm^3\n", open, narrow, wide)
	if !(narrow > open) {
		return fmt.Errorf("auto-miter filled nothing: %.6f vs the open corner's %.6f", narrow, open)
	}
	if !(wide < narrow) {
		return fmt.Errorf("a wider miter gap cut no more: %.6f at 3 mm vs %.6f at 0.5 mm", wide, narrow)
	}
	return smMiterGapDefaultsToTheStyle(c)
}

// smMiterGapDefaultsToTheStyle checks an omitted miterGap falls back to the rule's GapSize
// (#1961), which the standard roster states as Thickness (#1962) — 1 mm on this sheet. Asking
// for "1 mm" explicitly must therefore build the same part as not asking at all.
func smMiterGapDefaultsToTheStyle(c *caller) error {
	implicit, err := smCornerVolume(c, "smmiter-styled", true, "")
	if err != nil {
		return err
	}
	explicit, err := smCornerVolume(c, "smmiter-explicit", true, "1 mm")
	if err != nil {
		return err
	}
	fmt.Printf("  gap from the style = %.6f, asked for 1 mm = %.6f cm^3\n", implicit, explicit)
	if math.Abs(implicit-explicit) > 1e-9 {
		return fmt.Errorf("an omitted miterGap gave %.6f, the style's 1 mm gives %.6f", implicit, explicit)
	}
	return nil
}

// smCornerVolume folds two walls off adjacent edges, mitering the second onto the first.
func smCornerVolume(c *caller, name string, miter bool, gap string) (float64, error) {
	if _, err := smSheet(c, name); err != nil {
		return 0, err
	}
	edgeX, edgeY, err := smTopEdges(c)
	if err != nil {
		return 0, err
	}
	first := map[string]any{"edge": edgeX, "height": "10 mm", "radius": "2 mm"}
	if err := c.applyFeature("sheetMetalFlange", first); err != nil {
		return 0, err
	}
	second := map[string]any{"edge": edgeY, "height": "10 mm", "radius": "2 mm", "applyAutoMiter": miter}
	if gap != "" {
		second["miterGap"] = gap
	}
	if err := c.applyFeature("sheetMetalFlange", second); err != nil {
		return 0, err
	}
	v := smVolume(c)
	fmt.Printf("  %-18s = %.6f cm^3\n", name, v)
	return v, smValidSolid(c, name)
}

// smValidSolid fails unless every body passes the topology and self-intersection checks. A
// mitered corner that unions badly still reports the feature HEALTHY while leaving an open
// shell behind, so feature health is not evidence the geometry closed.
func smValidSolid(c *caller, tag string) error {
	for i := 0; i < c.bodyCount(); i++ {
		var res struct {
			Valid    bool `json:"valid"`
			Problems []struct {
				Kind  string `json:"kind"`
				Issue string `json:"issue"`
			} `json:"problems"`
		}
		c.json("body_validate", map[string]any{"bodyIndex": i, "checkLevel": 2}, &res)
		if c.err != nil {
			return c.err
		}
		if !res.Valid {
			return fmt.Errorf("[%s] body %d is invalid: %d problem(s): %v", tag, i, len(res.Problems), res.Problems)
		}
	}
	return nil
}

// runSheetMetalHems drives #1956 live: all four hem shapes fold and leave a valid solid, and
// each adds material. A hem that silently folds through the parent sheet still reports healthy.
func runSheetMetalHems(c *caller) error {
	for _, h := range []struct {
		name string
		args map[string]any
	}{
		{"single", map[string]any{"type": "single", "length": "6 mm", "gap": "1 mm"}},
		{"double", map[string]any{"type": "double", "length": "6 mm", "gap": "1 mm"}},
		{"rolled", map[string]any{"type": "rolled", "radius": "3 mm", "angle": "270 deg"}},
		{"teardrop", map[string]any{"type": "teardrop", "radius": "3 mm", "angle": "300 deg"}},
	} {
		if err := smHemAdds(c, h.name, h.args); err != nil {
			return err
		}
	}
	return nil
}

// smHemAdds folds one hem on a fresh sheet and checks it added material and stayed solid.
func smHemAdds(c *caller, name string, args map[string]any) error {
	if _, err := smSheet(c, "smhem-"+name); err != nil {
		return err
	}
	edge, _, err := smTopEdges(c)
	if err != nil {
		return err
	}
	before := smVolume(c)
	args["edge"] = edge
	if err := c.applyFeature("sheetMetalHem", args); err != nil {
		return err
	}
	after := smVolume(c)
	fmt.Printf("  hem %-9s %.6f -> %.6f cm^3 (+%.6f)\n", name, before, after, after-before)
	if after <= before {
		return fmt.Errorf("the %s hem added no material: %.6f -> %.6f", name, before, after)
	}
	return smValidSolid(c, name)
}

// runSheetMetalFlat drives #1963 and the flat pattern live: a folded sheet develops to a flat
// whose area exceeds the folded footprint, and the punch list answers (empty on a part with no
// punches, which is the answer a nest needs — not an error).
func runSheetMetalFlat(c *caller) error {
	if _, err := smSheet(c, "smflat"); err != nil {
		return err
	}
	edge, _, err := smTopEdges(c)
	if err != nil {
		return err
	}
	if err := c.applyFeature("sheetMetalFlange", map[string]any{
		"edge": edge, "height": "10 mm", "radius": "2 mm",
	}); err != nil {
		return err
	}
	if err := smFlatDevelops(c); err != nil {
		return err
	}
	return smPunchesAnswer(c)
}

// smFlatDevelops checks the developed flat is bigger than the 40x30 footprint it folded from,
// and that the one flange it folded shows up as exactly one bend line.
func smFlatDevelops(c *caller) error {
	var res struct {
		Flat struct {
			Area      float64 `json:"area"`
			Thickness float64 `json:"thickness"`
			Bends     []struct {
				Angle float64 `json:"angle"`
			} `json:"bends"`
		} `json:"flat"`
	}
	c.json("sheet_metal_unfold", nil, &res)
	if c.err != nil {
		return c.err
	}
	flat := res.Flat
	fmt.Printf("  flat area = %.4f cm^2, gauge %.4f cm, %d bend line(s)\n",
		flat.Area, flat.Thickness, len(flat.Bends))
	if flat.Area <= 4.0*3.0 {
		return fmt.Errorf("the flat is %.4f cm^2, no bigger than the 12 cm^2 it folded from", flat.Area)
	}
	if len(flat.Bends) != 1 {
		return fmt.Errorf("one flange developed %d bend lines, want 1", len(flat.Bends))
	}
	return nil
}

// smPunchesAnswer checks the punch list is reachable and reports none on a part with no punch.
func smPunchesAnswer(c *caller) error {
	var res struct {
		Punches []struct {
			Name string `json:"name"`
		} `json:"punches"`
	}
	c.json("flat_pattern_list_punches", nil, &res)
	if c.err != nil {
		return c.err
	}
	fmt.Printf("  flat punches = %d\n", len(res.Punches))
	if len(res.Punches) != 0 {
		return fmt.Errorf("a part with no punch feature reported %d punches", len(res.Punches))
	}
	return nil
}
