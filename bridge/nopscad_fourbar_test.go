// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// buildLinkPart models one bar of a planar linkage: a flat bar of pin-spacing `lenCm` with a
// round pivot boss at each end (at x=0 and x=lenCm). Each boss's top face (normal +Z, centred
// on the pin) is the rotational-joint axis the assembly pivots about — the faceted pin holes
// themselves can't yield a clean axis, so a boss face stands in for the pin.
func buildLinkPart(b *partBuilder, lenMM string, lenCm float64) {
	for _, p := range [][2]string{
		{"link_len", lenMM}, {"link_w", "12 mm"}, {"link_t", "4 mm"}, {"boss_d", "10 mm"}, {"boss_h", "4 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t
	// Bar spanning both pins with a small overhang, centred on Y.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{-0.7, -0.6}, {lenCm + 0.7, 0.6}}, "link_len + 14 mm", "link_w")
	b.feat("1-bar", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "link_t", "operation": "new"})
	// A pivot boss at each end.
	for i, x := range []float64{0, lenCm} {
		sk := addSketchOn(t, cs)
		b.dim(sk, "diameter", "boss_d", b.circle(sk, x, 0, "0.5 cm")[0])
		b.solved(sk)
		b.feat(stepName("2-boss", i), "extrude", map[string]any{"sketchIndex": sk, "profileIndex": 0, "distance": "boss_h", "operation": "join"})
	}
}

// bossFaceKey returns the key of the pivot-boss top face nearest x=nearX — i.e. the topmost
// (max-Z) face whose representative point is closest to nearX along X.
func bossFaceKey(t *testing.T, cs *mcp.ClientSession, nearX float64) string {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	maxZ := math.Inf(-1)
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) == 3 && f.Point[2] > maxZ {
			maxZ = f.Point[2]
		}
	}
	best, bestDX := "", math.Inf(1)
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) == 3 && f.Point[2] > maxZ-0.01 {
			if dx := math.Abs(f.Point[0] - nearX); dx < bestDX {
				best, bestDX = f.Key, dx
			}
		}
	}
	if best == "" {
		t.Fatalf("no boss top face near x=%.2f", nearX)
	}
	return best
}

type link struct {
	doc, occ   uint64
	pinA, pinB string // boss-top face keys at x=0 and x=lenCm
}

// Test4BarLinkage assembles a four-bar linkage — a CLOSED kinematic chain: ground, crank,
// coupler and rocker bars connected pin-to-pin by four rotational joints, the fourth closing
// the loop. With all pin axes parallel it is a planar mechanism of mobility 1 (the crank turns,
// the rest follow), though in 3D the parallel-axis joints constrain the out-of-plane DOF
// redundantly — so the test is whether the shared solver closes the loop and solves it (and
// the crank still drives) rather than choking on the redundant loop. Lengths are a Grashof
// crank-rocker (ground 80, crank 30, coupler 70, rocker 60 mm).
func Test4BarLinkage(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	mk := func(name, lenMM string, lenCm float64) link {
		doc := newPartDoc(t, cs, name)
		buildLinkPart(b, lenMM, lenCm)
		return link{doc: doc, pinA: bossFaceKey(t, cs, 0), pinB: bossFaceKey(t, cs, lenCm)}
	}
	ground := mk("ground.obk", "80 mm", 8.0)
	crank := mk("crank.obk", "30 mm", 3.0)
	coupler := mk("coupler.obk", "70 mm", 7.0)
	rocker := mk("rocker.obk", "60 mm", 6.0)

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "fourbar.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	place := func(l *link, name string) {
		l.occ = placeComponent(t, cs, "place_component", map[string]any{"document": l.doc, "name": name, "transform": identityCells})
	}
	place(&ground, "ground")
	place(&crank, "crank")
	place(&coupler, "coupler")
	place(&rocker, "rocker")
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": ground.occ, "grounded": true}, nil)

	// Four rotational joints around the loop.
	join := func(a link, ak string, c link, ck string) uint64 {
		var r wire.AssemblyJointResult
		callJSON(t, cs, "add_rotational_joint", map[string]any{"a": geomRef(a.occ, ak), "b": geomRef(c.occ, ck)}, &r)
		return r.Joint.ID
	}
	crankJoint := join(ground, ground.pinB, crank, crank.pinA)
	join(crank, crank.pinB, coupler, coupler.pinA)
	join(coupler, coupler.pinB, rocker, rocker.pinA)
	join(rocker, rocker.pinB, ground, ground.pinA) // closes the loop

	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	dofs := fmt.Sprintf("crank=%d coupler=%d rocker=%d", dofOf(health, crank.occ), dofOf(health, coupler.occ), dofOf(health, rocker.occ))
	t.Logf("4-bar after closing the loop: %s health=%s", dofs, health.Status)

	// The mechanism must drive — turning the crank moves the closed chain through valid poses.
	var res wire.DriveResult
	callJSON(t, cs, "drive_joint", map[string]any{
		"joint": crankJoint, "settings": map[string]any{"start": 0.0, "end": math.Pi / 4, "step": math.Pi / 16},
	}, &res)
	if len(res.Frames) == 0 {
		t.Errorf("4-bar crank drive returned no frames — the closed loop did not solve through its range")
	}
	t.Logf("4-bar crank drive: %d frames", len(res.Frames))
}
