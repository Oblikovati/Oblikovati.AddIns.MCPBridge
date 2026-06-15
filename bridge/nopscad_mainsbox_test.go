// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// buildSocketBox models the printed enclosure of the NopSCADlib MainsBreakOutBox (socket_box,
// simplified): a 90×70×40 mm shelled tray (open top).
func buildSocketBox(b *partBuilder) {
	for _, p := range [][2]string{{"bw", "90 mm"}, {"bd", "70 mm"}, {"bh", "40 mm"}, {"wall", "2.5 mm"}} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{-4.5, -3.5}, {4.5, 3.5}}, "bw", "bd")
	b.feat("1-box", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "bh", "operation": "new"})
	b.feat("2-shell", "shell", map[string]any{"faceRefs": []string{topFaceKey(t, cs)}, "thickness": "wall"})
}

// buildFootPart models a rubber foot (NopSCADlib foot, simplified): a Ø16 × 6 mm puck.
func buildFootPart(b *partBuilder) {
	b.param("foot_d", "16 mm")
	b.param("foot_h", "6 mm")
	cs, t := b.cs, b.t
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.dim(0, "diameter", "foot_d", b.circle(0, 0, 0, "0.8 cm")[0])
	b.solved(0)
	b.feat("foot", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "foot_h", "operation": "new"})
}

// buildSocketPart models the mains socket mounted on top (simplified): an 80×50×18 mm block.
func buildSocketPart(b *partBuilder) {
	for _, p := range [][2]string{{"sw", "80 mm"}, {"sd", "50 mm"}, {"sh", "18 mm"}} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{-4, -2.5}, {4, 2.5}}, "sw", "sd")
	b.feat("socket", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "sh", "operation": "new"})
}

// transformAt is a row-major 4×4 placement with translation (cm) in the last column.
func transformAt(x, y, z float64) []float64 {
	m := make([]float64, len(identityCells))
	copy(m, identityCells)
	m[3], m[7], m[11] = x, y, z
	return m
}

// occurrenceCount returns how many top-level occurrences the active assembly holds.
func occurrenceCount(t *testing.T, cs *mcp.ClientSession) int {
	t.Helper()
	var list wire.OccurrencesResult
	callJSON(t, cs, "list_occurrences", nil, &list)
	return len(list.Occurrences)
}

// TestMainsBreakOutBoxAssembly composes the NopSCADlib MainsBreakOutBox as a HIERARCHICAL
// assembly (its real base → feet → main nesting, distilled): a `base` sub-assembly — the
// shelled socket box plus four corner feet — is nested inside the top `bob_main` assembly along
// with the mains socket. It exercises nested sub-assemblies and many occurrences across two
// levels (1 box + 4 feet in base; base + socket in bob_main), the scale/hierarchy the example
// projects bring, rather than a new joint type. The full BOM (IEC inlet, jacks, wiring, fuses)
// is out of scope for v1.
func TestMainsBreakOutBoxAssembly(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	boxDoc := newPartDoc(t, cs, "socket_box.obk")
	buildSocketBox(b)
	footDoc := newPartDoc(t, cs, "foot.obk")
	buildFootPart(b)
	socketDoc := newPartDoc(t, cs, "mains_socket.obk")
	buildSocketPart(b)

	// --- base sub-assembly: the box (grounded) + four corner feet. ---
	var base wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "base.obk"}, &base)
	callJSON(t, cs, "activate_document", map[string]any{"id": base.ID}, nil)
	box := placeComponent(t, cs, "place_component", map[string]any{"document": boxDoc, "name": "box", "transform": identityCells})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": box, "grounded": true}, nil)
	for i, corner := range [][2]float64{{-3.8, -2.8}, {3.8, -2.8}, {3.8, 2.8}, {-3.8, 2.8}} {
		foot := placeComponent(t, cs, "place_component", map[string]any{
			"document": footDoc, "name": footName(i), "transform": transformAt(corner[0], corner[1], -0.6),
		})
		callJSON(t, cs, "ground_occurrence", map[string]any{"id": foot, "grounded": true}, nil)
	}
	if n := occurrenceCount(t, cs); n != 5 {
		t.Fatalf("base sub-assembly = %d occurrences, want 5 (box + 4 feet)", n)
	}

	// --- bob_main: the base sub-assembly NESTED in, plus the mains socket on top. ---
	var main wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "bob_main.obk"}, &main)
	callJSON(t, cs, "activate_document", map[string]any{"id": main.ID}, nil)
	baseOcc := placeComponent(t, cs, "place_component", map[string]any{"document": base.ID, "name": "base", "transform": identityCells})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": baseOcc, "grounded": true}, nil)
	socketOcc := placeComponent(t, cs, "place_component", map[string]any{"document": socketDoc, "name": "socket", "transform": transformAt(0, 0, 4.0)})
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": socketOcc, "grounded": true}, nil)
	if n := occurrenceCount(t, cs); n != 2 {
		t.Fatalf("bob_main = %d top-level occurrences, want 2 (base sub-assembly + socket)", n)
	}

	// The whole hierarchy solves (the nested base + the socket) into a consistent assembly.
	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	if health.Status == "" {
		t.Errorf("bob_main did not produce a solve health report")
	}
	t.Logf("MainsBreakOutBox hierarchy solved: base(5 occ) nested in bob_main(2 top-level occ), health=%s", health.Status)
}

// footName labels the i-th corner foot.
func footName(i int) string { return "foot:" + string(rune('1'+i)) }
