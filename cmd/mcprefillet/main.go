// SPDX-License-Identifier: GPL-2.0-only

// Command mcprefillet drives the LIVE oblikovati app over the MCP bridge to reproduce a fillet-of-
// fillet bug: starting from the fillet-1 scenario (a box with ONE vertical edge rounded), it adds a
// second fillet to topology the first fillet created — a tangent LINE edge (cylinder∩plane), an ARC
// edge (cylinder∩cap), or a mix of those with an original box edge — and reports whether anything
// is actually computed. The symptom under test: the second fillet appears as a feature node in the
// browser but no geometry changes (no edge/volume delta), i.e. it is recorded but not built.
//
// For each scenario it rebuilds fillet-1, applies the second fillet, and prints the add_feature
// health, the model-tree feature list (so a silently-added-but-dead node is visible), and the
// edge/face/volume deltas, then captures a window PNG framed on the rounded (x=4,y=3) corner.
//
//	1 refillet-line       second fillet on a new tangent line edge (cylinder∩side plane)
//	2 refillet-arc        second fillet on a new arc edge (cylinder∩top cap)
//	3 refillet-line+arc+box   line + arc + an original box edge in one feature
//
// Usage: mcprefillet [--url http://127.0.0.1:7800/mcp] [--dir /tmp/refillet]
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

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	dir := flag.String("dir", "/tmp/refillet", "directory for the captured PNGs")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mcprefillet:", err)
		os.Exit(1)
	}
	if err := run(*url, *dir); err != nil {
		fmt.Fprintln(os.Stderr, "mcprefillet:", err)
		os.Exit(1)
	}
}

type ref struct {
	key string
	p   [3]float64
}

type driver struct {
	ctx context.Context
	cs  *mcp.ClientSession
	dir string
	seq int
}

func run(url, dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cl := mcp.NewClient(&mcp.Implementation{Name: "mcprefillet", Version: "0.1.0"}, nil)
	cs, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer cs.Close()
	d := &driver{ctx: ctx, cs: cs, dir: dir}
	d.call("close_all_documents", map[string]any{"force": true})

	if os.Getenv("DUMP") != "" {
		d.fillet1()
		for i, e := range d.edges() {
			fmt.Printf("edge %2d mid=(% .3f % .3f % .3f)\n", i, e.p[0], e.p[1], e.p[2])
		}
		return nil
	}

	d.scenario("1-refillet-line", func(box []ref) []ref {
		return []ref{d.newTangentLine()} // a tangent line the first fillet created
	})
	d.scenario("2-refillet-arc", func(box []ref) []ref {
		return []ref{d.newTopArc()} // an arc the first fillet created
	})
	d.scenario("3-refillet-line-arc-box", func(box []ref) []ref {
		// the original top box edge survives the first fillet but is re-keyed, so resolve it from
		// the CURRENT topology, not the stale pre-fillet box.
		return []ref{d.newTangentLine(), d.newTopArc(), *nearestEdge(d.edges(), [3]float64{2, 0, zTop})}
	})
	return nil
}

// scenario rebuilds fillet-1, selects the second-fillet edges via pick (evaluated AFTER the first
// fillet so it can reference new topology), applies a 1 mm fillet, and reports + captures.
func (d *driver) scenario(label string, pick func(box []ref) []ref) {
	box := d.fillet1() // the fillet-1 start point: a box with one vertical edge rounded
	before := d.snapshot()
	sel := pick(box)
	if anyMissing(sel) {
		fmt.Printf("%-26s could not locate the target edge(s)\n", label)
		d.capture(label, "missing edges")
		return
	}
	status := d.fillet(sel, "1 mm")
	after := d.snapshot()
	d.report(label, status, sel, before, after)
	d.capture(label, status)
}

// snapshot captures the metrics that reveal whether a feature actually built geometry.
type snapshot struct {
	edges, faces int
	volume       float64
	features     []feature
}

func (d *driver) snapshot() snapshot {
	return snapshot{edges: len(d.edges()), faces: len(d.faces()), volume: d.volume(), features: d.features()}
}

// report prints the add_feature health, the model-tree feature list (a node added here but dead is
// the bug), and the edge/face/volume deltas (all zero ⇒ recorded but not built).
func (d *driver) report(label, status string, sel []ref, before, after snapshot) {
	fmt.Printf("%-26s add_feature=%s\n", label, status)
	fmt.Printf("    edges %d→%d  faces %d→%d  volume %.5f→%.5f (Δ%.5f)\n",
		before.edges, after.edges, before.faces, after.faces, before.volume, after.volume, after.volume-before.volume)
	for _, e := range sel {
		fmt.Printf("    picked edge mid=(% .2f % .2f % .2f)\n", e.p[0], e.p[1], e.p[2])
	}
	fmt.Printf("    model tree (%d features):\n", len(after.features))
	for _, f := range after.features {
		h := "healthy"
		if f.Health != "" {
			h = "SICK: " + f.Health
		}
		if f.Suppressed {
			h += " [suppressed]"
		}
		fmt.Printf("      #%d %-10s %s — %s\n", f.ID, f.Kind, f.Name, h)
	}
	if status == "healthy" && after.edges == before.edges && after.volume == before.volume {
		fmt.Printf("    >>> SILENT: feature reported healthy but geometry is UNCHANGED (no fillet built)\n")
	}
}

// fillet1 creates a fresh box and rounds its single vertical edge at (4,3) — the fillet-1 scenario.
func (d *driver) fillet1() []ref {
	d.seq++
	d.call("create_document", map[string]any{"type": "part", "name": fmt.Sprintf("refillet-%d", d.seq)})
	d.call("create_sketch", map[string]any{"plane": "XY"})
	d.call("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"})
	d.callJSON("add_feature", map[string]any{"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"}}, &struct{}{})
	box := d.edges()
	vert := filter(box, func(r ref) bool { return near(r.p[0], xMax) && near(r.p[1], yMax) }) // the (4,3) vertical edge
	d.fillet(vert, "3 mm")
	return box
}

// fillet applies one constant-radius fillet to the selected edges and returns a status.
func (d *driver) fillet(sel []ref, radius string) string {
	keys := make([]string, len(sel))
	for i, e := range sel {
		keys[i] = e.key
	}
	var r struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	d.callJSON("add_feature", map[string]any{"kind": "fillet", "args": map[string]any{"edgeRefs": keys, "radius": radius}}, &r)
	if !r.Healthy {
		if r.Reason == "" {
			return "unhealthy(no reason)"
		}
		return "SICK: " + r.Reason
	}
	return "healthy"
}

// --- locating the edges the first fillet created (box x∈[0,4], y∈[0,3], z∈[0,2]; fillet r=0.3 on
// the (4,3) vertical edge → cylinder centre (3.7,2.7), tangent lines at (4,2.7) and (3.7,3), and
// quarter-arc caps on z=0 and z=2) -----------------------------------------------------------

const (
	xMax = 4.0
	yMax = 3.0
	zTop = 2.0
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// newTangentLine returns the vertical tangent line where the fillet cylinder meets the x=4 face
// (midpoint ≈ (4, 2.7, 1)).
func (d *driver) newTangentLine() ref {
	return *nearestEdge(d.edges(), [3]float64{xMax, 2.7, 1})
}

// newTopArc returns the quarter-arc where the fillet cylinder meets the top cap z=2 (range-box
// centre ≈ (3.85, 2.85, 2)).
func (d *driver) newTopArc() ref {
	return *nearestEdge(d.edges(), [3]float64{3.85, 2.85, zTop})
}

func anyMissing(sel []ref) bool {
	for _, e := range sel {
		if e.key == "" {
			return true
		}
	}
	return false
}

// capture frames on the rounded (x=4,y=3) vertical corner and writes a window PNG (the browser tree
// is in-frame, so a feature node with no geometry is visible).
func (d *driver) capture(label, note string) {
	d.call("set_camera", map[string]any{"eye": []float64{7.5, 6.5, 3.2}, "target": []float64{3.85, 2.85, 1}, "up": []float64{0, 0, 1}, "fov": 0.5})
	d.call("set_mesh_colors", map[string]any{"on": true, "perTriangle": false})
	out := d.dir + "/" + label + ".png"
	d.call("capture_window", map[string]any{"path": out})
	fmt.Printf("    -> %s\n", out)
}

// --- model-tree / topology reads --------------------------------------------------------------

type feature struct {
	ID         uint64 `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Suppressed bool   `json:"suppressed"`
	Health     string `json:"health"`
}

func (d *driver) features() []feature {
	var mt struct {
		Features []feature `json:"features"`
	}
	d.callJSON("get_model_tree", nil, &mt)
	return mt.Features
}

func (d *driver) volume() float64 {
	var pp struct {
		Volume float64 `json:"volume"`
	}
	d.callJSON("get_physical_properties", nil, &pp)
	return pp.Volume
}

func (d *driver) edges() []ref { return d.topo(func(b bodyTopo) []rawRef { return b.Edges }) }
func (d *driver) faces() []ref { return d.topo(func(b bodyTopo) []rawRef { return b.Faces }) }

type rawRef struct {
	Key   string    `json:"key"`
	Point []float64 `json:"point"`
}
type bodyTopo struct {
	Faces []rawRef `json:"faces"`
	Edges []rawRef `json:"edges"`
}

func (d *driver) topo(pick func(bodyTopo) []rawRef) []ref {
	var rk struct {
		Bodies []bodyTopo `json:"bodies"`
	}
	if !d.callJSON("get_reference_keys", nil, &rk) || len(rk.Bodies) == 0 {
		return nil
	}
	var out []ref
	for _, e := range pick(rk.Bodies[0]) {
		if len(e.Point) == 3 {
			out = append(out, ref{key: e.Key, p: [3]float64{e.Point[0], e.Point[1], e.Point[2]}})
		}
	}
	return out
}

func nearestEdge(e []ref, target [3]float64) *ref {
	var best *ref
	bestD := 0.0
	for i := range e {
		dx, dy, dz := e[i].p[0]-target[0], e[i].p[1]-target[1], e[i].p[2]-target[2]
		dd := dx*dx + dy*dy + dz*dz
		if best == nil || dd < bestD {
			best, bestD = &e[i], dd
		}
	}
	if best == nil {
		return &ref{}
	}
	return best
}

func filter(e []ref, keep func(ref) bool) []ref {
	var out []ref
	for _, r := range e {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

// --- mcp plumbing -----------------------------------------------------------------------------

func (d *driver) call(tool string, a map[string]any) {
	_, _ = d.cs.CallTool(d.ctx, &mcp.CallToolParams{Name: tool, Arguments: a})
}

func (d *driver) callJSON(tool string, a map[string]any, v any) bool {
	res, err := d.cs.CallTool(d.ctx, &mcp.CallToolParams{Name: tool, Arguments: a})
	if err != nil || res.IsError {
		return false
	}
	for _, ct := range res.Content {
		if tc, ok := ct.(*mcp.TextContent); ok {
			return json.Unmarshal([]byte(tc.Text), v) == nil
		}
	}
	return false
}
