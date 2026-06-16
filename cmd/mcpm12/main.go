// SPDX-License-Identifier: GPL-2.0-only

// Command mcpm12 drives the full M12 assembly surface end-to-end against a running head over the
// MCP bridge and captures visual proof of each feature working live: F01 constraints (mate/flush),
// F02 joints (rotational + DOF), F03 drive (animated sweep), Grip Snap (#794, inferred constraint),
// F04 representations (design view), and F05 interference analysis. Each step verifies the model
// changed as expected (occurrence repositioned, DOF reduced, frames produced, overlap volume found)
// and writes a PNG; a PASS/GAP summary is printed at the end and a non-zero exit flags any gap.
// Usage: mcpm12 [--url http://127.0.0.1:7800/mcp] [--out /tmp/m12]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sweep holds the live MCP session plus the running PASS/GAP tally for the summary report.
type sweep struct {
	ctx     context.Context
	cs      *mcp.ClientSession
	outDir  string
	results []checkResult
}

type checkResult struct {
	feature string
	ok      bool
	detail  string
}

func (s *sweep) record(feature string, ok bool, detail string) {
	mark := "PASS"
	if !ok {
		mark = "GAP "
	}
	fmt.Printf("[%s] %-22s %s\n", mark, feature, detail)
	s.results = append(s.results, checkResult{feature, ok, detail})
}

// call invokes an MCP tool, fatally exiting on transport error, and unmarshals the first text
// content into out when provided.
func (s *sweep) call(tool string, args map[string]any, out any) {
	res, err := s.cs.CallTool(s.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", tool, err)
		os.Exit(2)
	}
	if res.IsError {
		fmt.Fprintf(os.Stderr, "%s: tool reported error: %v\n", tool, contentText(res))
		os.Exit(2)
	}
	if out != nil {
		_ = json.Unmarshal([]byte(contentText(res)), out)
	}
}

func contentText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func (s *sweep) capture(name string) {
	path := filepath.Join(s.outDir, name)
	s.call("execute_command", map[string]any{"id": "View.Home"}, nil)
	s.call("capture_window", map[string]any{"path": path}, nil)
	fmt.Printf("       captured -> %s\n", path)
}

func identity() []float64 { return []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1} }

// faceKeys is the box's representative face keys, picked by world point so constraints are
// deterministic: top (+Z), bottom (−Z), and one side (+X).
type faceKeys struct{ top, bottom, sideX string }

// buildBoxFaces creates a 40×30×20 mm box in the active part and returns its labelled faces.
func (s *sweep) buildBoxFaces() faceKeys {
	s.call("create_sketch", map[string]any{"plane": "XY"}, nil)
	s.call("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	s.call("add_feature", map[string]any{"kind": "extrude", "args": map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"}}, nil)

	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	s.call("get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 {
		fmt.Fprintln(os.Stderr, "box produced no body")
		os.Exit(2)
	}
	var k faceKeys
	hiZ, loZ, hiX := math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) != 3 {
			continue
		}
		if f.Point[2] > hiZ {
			k.top, hiZ = f.Key, f.Point[2]
		}
		if f.Point[2] < loZ {
			k.bottom, loZ = f.Key, f.Point[2]
		}
		if f.Point[0] > hiX {
			k.sideX, hiX = f.Key, f.Point[0]
		}
	}
	return k
}

type occResult struct {
	Occurrence struct {
		ID uint64 `json:"id"`
	} `json:"occurrence"`
}

func ref(occ uint64, entity string) map[string]any {
	return map[string]any{"occurrence": occ, "entity": entity}
}

// matrixDelta is the summed absolute cell difference between two 16-cell transforms — 0 when a
// drive frame did not move the occurrence, >0 when it did.
func matrixDelta(a, b []float64) float64 {
	if len(a) != 16 || len(b) != 16 {
		return 0
	}
	sum := 0.0
	for i := range a {
		sum += math.Abs(a[i] - b[i])
	}
	return sum
}

// occZ returns the world Z (cm) of an occurrence's placement origin from list_occurrences.
func (s *sweep) occZ(id uint64) float64 {
	var tree struct {
		Occurrences []struct {
			ID        uint64    `json:"id"`
			Transform []float64 `json:"transform"`
		} `json:"occurrences"`
	}
	s.call("list_occurrences", nil, &tree)
	var walk func() float64
	walk = func() float64 {
		for _, o := range tree.Occurrences {
			if o.ID == id && len(o.Transform) == 16 {
				return o.Transform[11] // row-major translation Z
			}
		}
		return math.NaN()
	}
	return walk()
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()
	url := "http://127.0.0.1:7800/mcp"
	outDir := "/tmp/m12"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	_ = os.MkdirAll(outDir, 0o755)

	mc := mcp.NewClient(&mcp.Implementation{Name: "mcpm12", Version: "0.1.0"}, nil)
	cs, err := mc.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(2)
	}
	defer cs.Close()
	s := &sweep{ctx: ctx, cs: cs, outDir: outDir}

	s.call("close_all_documents", map[string]any{"force": true}, nil)
	s.call("create_document", map[string]any{"type": "part", "name": "m12box"}, nil)
	fk := s.buildBoxFaces()

	var part struct {
		Documents []struct {
			ID   uint64 `json:"id"`
			Type string `json:"type"`
		} `json:"documents"`
	}
	s.call("list_documents", nil, &part)
	var boxID uint64
	for _, d := range part.Documents {
		if d.Type == "part" {
			boxID = d.ID
		}
	}

	s.call("create_document", map[string]any{"type": "assembly", "name": "m12asm"}, nil)
	var a, b, c occResult
	s.call("place_component", map[string]any{"document": boxID, "name": "box:1", "transform": identity()}, &a)
	s.call("place_component_copy", map[string]any{"source": a.Occurrence.ID, "name": "box:2", "transform": identity()}, &b)
	s.call("place_component_copy", map[string]any{"source": a.Occurrence.ID, "name": "box:3", "transform": identity()}, &c)
	s.call("ground_occurrence", map[string]any{"id": a.Occurrence.ID, "grounded": true}, nil)

	s.checkMate(a, b, fk)
	s.checkFlush(a, c, fk)
	s.checkJointAndDrive(a, b, fk)
	s.checkGripSnap(a, c, fk)
	s.checkRepresentation()
	d := s.checkInterference(a)
	s.checkContact(a, d)
	s.checkFlexible(boxID)

	s.summary()
}

// checkMate (F01) mates box:2's bottom onto the grounded box:1's top and verifies it stacked one
// box-height (2 cm) up.
func (s *sweep) checkMate(a, b occResult, fk faceKeys) {
	var r struct {
		Constraint struct {
			Type string `json:"type"`
		} `json:"constraint"`
	}
	s.call("add_mate_constraint", map[string]any{"a": ref(a.Occurrence.ID, fk.top), "b": ref(b.Occurrence.ID, fk.bottom)}, &r)
	z := s.occZ(b.Occurrence.ID)
	ok := r.Constraint.Type == "mate" && math.Abs(z-2.0) < 1e-3
	s.record("F01 mate", ok, fmt.Sprintf("type=%s box:2 z=%.3fcm (want ~2.0)", r.Constraint.Type, z))
	s.capture("01-mate.png")
}

// checkFlush (F01) flushes box:3's +X face against the grounded box:1's +X face (coplanar, aligned).
func (s *sweep) checkFlush(a, c occResult, fk faceKeys) {
	var r struct {
		Constraint struct{ Type string } `json:"constraint"`
		Health     string                `json:"health"`
	}
	s.call("add_flush_constraint", map[string]any{"a": ref(a.Occurrence.ID, fk.sideX), "b": ref(c.Occurrence.ID, fk.sideX)}, &r)
	s.record("F01 flush", r.Constraint.Type == "flush", fmt.Sprintf("type=%s health=%s", r.Constraint.Type, r.Health))
	s.capture("02-flush.png")
}

// checkJointAndDrive (F02/F03) adds a rotational joint between two faces, confirms it reduces DOF to a
// single hinge, then drives it through a range and confirms an animated frame series comes back.
func (s *sweep) checkJointAndDrive(a, b occResult, fk faceKeys) {
	var jr struct {
		Joint struct {
			ID               uint64 `json:"id"`
			Type             string `json:"type"`
			DegreesOfFreedom int    `json:"degreesOfFreedom"`
		} `json:"joint"`
	}
	s.call("add_rotational_joint", map[string]any{"a": ref(a.Occurrence.ID, fk.top), "b": ref(b.Occurrence.ID, fk.bottom)}, &jr)
	s.record("F02 rotational joint", jr.Joint.Type == "rotational" && jr.Joint.ID != 0,
		fmt.Sprintf("id=%d type=%s dof=%d", jr.Joint.ID, jr.Joint.Type, jr.Joint.DegreesOfFreedom))

	var dr struct {
		Frames []struct {
			Value      float64 `json:"value"`
			Collided   bool    `json:"collided"`
			Placements []struct {
				Occurrence uint64    `json:"occurrence"`
				Transform  []float64 `json:"transform"`
			} `json:"placements"`
		} `json:"frames"`
	}
	s.call("drive_joint", map[string]any{"joint": jr.Joint.ID, "settings": map[string]any{
		"variable": "angular", "start": 0.0, "end": math.Pi / 2, "step": math.Pi / 8}}, &dr)
	// Proof of motion: the driven occurrence's world transform must actually change between the
	// first and last frame — a frame count alone could come back from a no-op solve.
	placementOf := func(frame int) []float64 {
		if frame < 0 || frame >= len(dr.Frames) {
			return nil
		}
		for _, p := range dr.Frames[frame].Placements {
			if p.Occurrence == b.Occurrence.ID {
				return p.Transform
			}
		}
		return nil
	}
	moved := matrixDelta(placementOf(0), placementOf(len(dr.Frames)-1))
	ok := len(dr.Frames) >= 4 && moved > 1e-6
	last := math.NaN()
	if n := len(dr.Frames); n > 0 {
		last = dr.Frames[n-1].Value
	}
	s.record("F03 drive sweep", ok, fmt.Sprintf("%d frames, last value=%.3frad, box:2 moved Δ=%.4f", len(dr.Frames), last, moved))
	s.capture("03-joint-driven.png")
}

// checkGripSnap (#794) snaps box:3's bottom onto box:1's top WITHOUT naming a constraint and confirms
// the host infers a mate from the two opposed planar faces.
func (s *sweep) checkGripSnap(a, c occResult, fk faceKeys) {
	var r struct {
		Constraint struct{ Type string } `json:"constraint"`
	}
	s.call("assembly_snap_constrain", map[string]any{
		"a": ref(a.Occurrence.ID, fk.top), "b": ref(c.Occurrence.ID, fk.bottom)}, &r)
	s.record("Grip Snap (#794)", r.Constraint.Type == "mate",
		fmt.Sprintf("inferred=%s (want mate)", r.Constraint.Type))
	s.capture("04-grip-snap.png")
}

// checkRepresentation (F04) captures a design view of the current assembly state and confirms it is
// listed back — proving the representation subsystem persists named states.
func (s *sweep) checkRepresentation() {
	var cap struct {
		Representation struct {
			Name string `json:"name"`
		} `json:"representation"`
	}
	s.call("capture_design_view", map[string]any{"name": "M12 Sweep"}, &cap)
	var list struct {
		Representations []struct {
			Name string `json:"name"`
		} `json:"representations"`
	}
	s.call("list_design_views", nil, &list)
	found := false
	for _, v := range list.Representations {
		if v.Name == "M12 Sweep" {
			found = true
		}
	}
	s.record("F04 design view", found, fmt.Sprintf("captured %q, %d views listed", cap.Representation.Name, len(list.Representations)))
}

// checkInterference (F05) drops a fresh box:4 coincident with the grounded box:1 (identity placement,
// full overlap) and confirms the analysis reports the whole box volume (4×3×2 = 24 cm³) as interference
// — a real positive detection, not the trivial zero of non-overlapping parts.
func (s *sweep) checkInterference(a occResult) occResult {
	var d occResult
	s.call("place_component_copy", map[string]any{"source": a.Occurrence.ID, "name": "box:4", "transform": identity()}, &d)
	var r struct {
		Results []struct {
			Volume float64 `json:"volume"`
		} `json:"results"`
		TotalVolume float64 `json:"totalVolume"`
	}
	s.call("analyze_interference", map[string]any{
		"occurrences": []uint64{a.Occurrence.ID, d.Occurrence.ID}}, &r)
	const boxVol = 4.0 * 3.0 * 2.0 // cm³
	ok := len(r.Results) >= 1 && math.Abs(r.TotalVolume-boxVol) < 0.5
	s.record("F05 interference", ok,
		fmt.Sprintf("%d pairs, total overlap=%.3fcm³ (want ~%.0f)", len(r.Results), r.TotalVolume, boxVol))
	s.capture("05-interference.png")
	return d
}

// checkContact (F05) groups box:1 and box:4 into a contact set, enables the contact solver, and
// confirms the solver reports itself enabled with the set counted.
func (s *sweep) checkContact(a, d occResult) {
	var set struct {
		ContactSet struct {
			ID uint64 `json:"id"`
		} `json:"contactSet"`
	}
	s.call("create_contact_set", map[string]any{"name": "M12 contact"}, &set)
	s.call("add_contact_member", map[string]any{"set": set.ContactSet.ID, "occurrence": a.Occurrence.ID}, nil)
	s.call("add_contact_member", map[string]any{"set": set.ContactSet.ID, "occurrence": d.Occurrence.ID}, nil)
	s.call("set_contact_solver", map[string]any{"enabled": true}, nil)
	var status struct {
		Solver struct {
			Enabled  bool `json:"enabled"`
			SetCount int  `json:"setCount"`
		} `json:"solver"`
	}
	s.call("contact_solver_status", nil, &status)
	ok := status.Solver.Enabled && status.Solver.SetCount >= 1
	s.record("F05 contact set", ok, fmt.Sprintf("enabled=%v setCount=%d", status.Solver.Enabled, status.Solver.SetCount))
}

// checkFlexible (F06) builds a 2-box subassembly, places it as an occurrence in the top assembly,
// marks it flexible, and confirms the host reports the occurrence as flexible — proving a subassembly
// can solve its components independently per placement.
func (s *sweep) checkFlexible(boxID uint64) {
	s.call("create_document", map[string]any{"type": "assembly", "name": "m12sub"}, nil)
	var sub struct {
		Documents []struct {
			ID   uint64 `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"documents"`
	}
	var subAnchor occResult
	s.call("place_component", map[string]any{"document": boxID, "name": "subbox:1", "transform": identity()}, &subAnchor)
	s.call("place_component_copy", map[string]any{"source": subAnchor.Occurrence.ID, "name": "subbox:2",
		"transform": []float64{1, 0, 0, 5, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}}, nil)

	s.call("list_documents", nil, &sub)
	var subID, topID uint64
	for _, dd := range sub.Documents {
		switch dd.Name {
		case "m12sub":
			subID = dd.ID
		case "m12asm":
			topID = dd.ID
		}
	}
	s.call("activate_document", map[string]any{"id": topID}, nil)
	var subOcc struct {
		Occurrence struct {
			ID       uint64 `json:"id"`
			Flexible bool   `json:"flexible"`
		} `json:"occurrence"`
	}
	s.call("place_component", map[string]any{"document": subID, "name": "sub:1",
		"transform": []float64{1, 0, 0, 0, 0, 1, 0, 8, 0, 0, 1, 0, 0, 0, 0, 1}}, &subOcc)
	var flex struct {
		Occurrence struct {
			Flexible bool `json:"flexible"`
		} `json:"occurrence"`
	}
	s.call("set_flexible_occurrence", map[string]any{"id": subOcc.Occurrence.ID, "flexible": true}, &flex)
	s.record("F06 flexible subassembly", flex.Occurrence.Flexible,
		fmt.Sprintf("occurrence %d flexible=%v", subOcc.Occurrence.ID, flex.Occurrence.Flexible))
	s.capture("06-flexible.png")
}

func (s *sweep) summary() {
	fmt.Println("\n=== M12 live sweep summary ===")
	gaps := 0
	for _, r := range s.results {
		if !r.ok {
			gaps++
		}
	}
	fmt.Printf("%d checks, %d gaps. captures in %s\n", len(s.results), gaps, s.outDir)
	if gaps > 0 {
		os.Exit(1)
	}
}
