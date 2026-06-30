// SPDX-License-Identifier: GPL-2.0-only

// Command mcpcamsim is a throwaway live check for the CAM simulator: it builds a box in the running
// head, generates a pocket toolpath, opens the simulator, plays it, and captures the viewport so the
// carved voxel stock + tool marker can be inspected. Usage: go run ./cmd/mcpcamsim
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type driver struct {
	ctx context.Context
	cs  *mcp.ClientSession
}

func (d *driver) call(tool string, a map[string]any) {
	if _, err := d.cs.CallTool(d.ctx, &mcp.CallToolParams{Name: tool, Arguments: a}); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", tool, err)
	}
}

func (d *driver) cmd(id string) { d.call("execute_command", map[string]any{"id": id}) }

func (d *driver) shot(path string) {
	d.call("capture_window", map[string]any{"path": path})
	fmt.Printf("captured -> %s\n", path)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cl := mcp.NewClient(&mcp.Implementation{Name: "mcpcamsim", Version: "0.1.0"}, nil)
	cs, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://127.0.0.1:7800/mcp"}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer cs.Close()
	d := &driver{ctx: ctx, cs: cs}

	// A 50×40×15 mm block to machine.
	d.call("close_all_documents", map[string]any{"force": true})
	d.call("create_document", map[string]any{"type": "part", "name": "camsim"})
	d.call("create_sketch", map[string]any{"plane": "XY"})
	d.call("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "50 mm", "height": "40 mm"})
	d.call("add_feature", map[string]any{"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "15 mm", "operation": "new"}})
	d.cmd("View.Home")
	time.Sleep(1 * time.Second)

	// Generate a pocket toolpath, then open the simulator on it.
	d.cmd("CAM.GeneratePocket")
	time.Sleep(4 * time.Second) // async CAM job + post
	d.cmd("CAM.Simulate")
	time.Sleep(2 * time.Second)
	d.cmd("View.Home")
	d.shot("/tmp/cam-sim-start.png") // material view, nothing carved yet

	// Play the simulation, let it carve partway, pause and capture.
	d.cmd("CAM.SimPlayPause")
	time.Sleep(3 * time.Second)
	d.cmd("CAM.SimPlayPause") // pause
	time.Sleep(1 * time.Second)
	d.shot("/tmp/cam-sim-mid.png")

	// Resume and let it finish, then capture the fully carved stock.
	d.cmd("CAM.SimPlayPause")
	time.Sleep(6 * time.Second)
	d.shot("/tmp/cam-sim-done.png")
	fmt.Println("done")
}
