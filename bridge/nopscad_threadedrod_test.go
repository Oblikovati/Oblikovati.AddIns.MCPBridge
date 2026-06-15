// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/app"
)

// TestNopThreadedRod models a threaded rod (NopSCADlib utils/thread.scad male thread, distilled):
// a core cylinder with a helical thread ridge coiled onto it. The thread is a round profile
// swept by the COIL feature and JOINED to the core — a helical swept solid unioned onto a
// cylinder, the most boolean-stressful combination in this porting sweep (the spring port
// exercises the coil alone; this adds the join against the core). The V thread profile is
// simplified to a round (worm/lead-screw) ridge for v1.
//
// Core Ø6 × 12 mm; thread ridge Ø2 round profile at mean radius 3.5, pitch 2.5 mm → ~4.8 turns (watertight via #880).
func TestNopThreadedRod(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	for _, p := range [][2]string{
		{"core_d", "6 mm"}, {"rod_len", "12 mm"}, {"thread_r", "3.5 mm"}, {"wire", "1 mm"}, {"pitch", "2.5 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs2, tt := b.cs, b.t

	// 1. Core cylinder Ø6 × 12 along Z.
	callJSON(tt, cs2, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.dim(0, "diameter", "core_d", b.circle(0, 0, 0, "0.3 cm")[0])
	b.solved(0)
	b.feat("1-core", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "rod_len", "operation": "new"})

	// 2. Thread profile: a round wire on the XZ axis-plane at mean radius thread_r, then COILED
	// up the rod and JOINED to the core — the helical thread ridge.
	callJSON(tt, cs2, "create_sketch", map[string]any{"plane": "XZ"}, nil)
	o := idsOf(tt, cs2, map[string]any{"sketchIndex": 1, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	wire := b.circle(1, 0.35, 0, "0.1 cm")
	b.con(1, "ground", o)
	b.con(1, "horizontal", o, wire[1]) // wire centre on the axis plane (world Z = 0)
	b.dim(1, "distance", "thread_r", o, wire[1])
	b.dim(1, "radius", "wire", wire[0])
	b.solved(1)
	b.feat("2-thread", "coil", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "axisRef": "origin/axis/z",
		"pitch": "pitch", "height": "rod_len", "operation": "join",
	})

	// Envelope: the thread ridge reaches Ø9 (mean 3.5 + wire 1 = 4.5 radius) and overshoots the
	// 12 mm core ends by the wire radius (z = −1 … 13 mm).
	assertEnvelope(t, cs, [3][2]float64{{-0.45, 0.45}, {-0.45, 0.45}, {-0.1, 1.3}})
	if v := partVolume(t, cs); v <= 0 {
		t.Errorf("threaded rod volume = %.4f, want > 0", v)
	}

	// Parametric: a finer pitch re-coils more turns onto the core, still one valid solid.
	callJSON(tt, cs2, "set_parameter", map[string]any{"name": "pitch", "expression": "2 mm"}, nil)
	b.mustValid("reparam")
}
