// SPDX-License-Identifier: GPL-2.0-only

// Command mcpviews live-tests the per-document multi-view feature against a running
// oblikovati-mcp-bridge: it builds a cube, switches to a quad layout, creates four views
// looking at the cube from front/top/right/iso, and confirms the document carries four
// views with distinct cameras. Run it with the app focused to watch the four tiles render.
//
// Usage: mcpviews [--url http://127.0.0.1:7800/mcp]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	flag.Parse()
	if err := run(*url); err != nil {
		fmt.Fprintln(os.Stderr, "mcpviews:", err)
		os.Exit(1)
	}
}

type viewInfo struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Camera struct {
		Eye [3]float64 `json:"eye"`
	} `json:"camera"`
}
type listViews struct {
	Views       []viewInfo `json:"views"`
	ActiveIndex int        `json:"activeIndex"`
	Layout      int        `json:"layout"`
}

func run(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpviews", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer cs.Close()
	d := &drv{ctx: ctx, cs: cs}

	// A 50 mm cube to look at.
	d.call("create_document", map[string]any{"type": "part", "name": "ViewsDemo"})
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	d.callInto("create_sketch", map[string]any{"plane": "XY"}, &sk)
	d.call("sketch_rectangle", map[string]any{"sketchIndex": sk.SketchIndex, "width": "50 mm", "height": "50 mm"})
	d.call("add_feature", map[string]any{"kind": "extrude", "args": map[string]any{
		"sketchIndex": sk.SketchIndex, "profileIndex": 0, "distance": "50 mm", "operation": "new"}})

	// Quad layout, four views (the first exists; add three more), framed from four angles.
	d.call("set_view_layout", map[string]any{"layout": 4})
	d.call("add_view", map[string]any{"copyActiveCamera": true})
	d.call("add_view", map[string]any{"copyActiveCamera": true})
	d.call("add_view", map[string]any{"copyActiveCamera": true})

	c := [3]float64{25, 25, 25} // cube centre
	frames := [][3]float64{{25, -150, 25}, {25, 25, 200}, {200, 25, 25}, {150, -150, 150}}
	ups := [][3]float64{{0, 0, 1}, {0, 1, 0}, {0, 0, 1}, {0, 0, 1}}
	for i, eye := range frames {
		d.call("activate_view", map[string]any{"index": i})
		d.call("set_camera", map[string]any{"eye": eye, "target": c, "up": ups[i], "fov": 0.7})
	}

	var lv listViews
	d.callInto("list_views", map[string]any{}, &lv)
	fmt.Printf("layout=%d activeIndex=%d views=%d\n", lv.Layout, lv.ActiveIndex, len(lv.Views))
	for _, v := range lv.Views {
		fmt.Printf("  view %d %-8q active=%v eye=%v\n", v.Index, v.Name, v.Active, v.Camera.Eye)
	}
	if len(lv.Views) != 4 || lv.Layout != 4 {
		return fmt.Errorf("FAIL: want 4 views in quad layout, got %d views layout=%d", len(lv.Views), lv.Layout)
	}
	for i, v := range lv.Views {
		if v.Camera.Eye != frames[i] {
			return fmt.Errorf("FAIL: view %d eye=%v, want %v (cameras not per-view)", i, v.Camera.Eye, frames[i])
		}
	}
	fmt.Println("PASS: four views with distinct cameras in a quad layout (watch the four tiles)")
	return nil
}

type drv struct {
	ctx context.Context
	cs  *mcp.ClientSession
}

func (d *drv) call(tool string, args map[string]any) {
	res, err := d.cs.CallTool(d.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s: transport error: %v\n", tool, err)
		return
	}
	if res.IsError {
		fmt.Fprintf(os.Stderr, "  %s: host error: %s\n", tool, firstText(res))
	}
}

func (d *drv) callInto(tool string, args map[string]any, v any) {
	res, err := d.cs.CallTool(d.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil || res.IsError {
		fmt.Fprintf(os.Stderr, "  %s failed\n", tool)
		return
	}
	_ = json.Unmarshal([]byte(firstText(res)), v)
}

func firstText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
