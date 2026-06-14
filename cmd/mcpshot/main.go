// SPDX-License-Identifier: GPL-2.0-only

// Command mcpshot drives the LIVE oblikovati app over the MCP bridge to import a CAD file and capture
// the viewport — the repeatable headless "see the import" loop. It closes all docs, creates a part,
// imports --file, optionally moves the camera (--eye/--target), captures to --out, and prints the
// reply. Usage: mcpshot --file /path/EDF.STEP [--out /tmp/shot.png] [--eye x,y,z --target x,y,z].
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func vec(s string) ([3]float64, bool) {
	p := strings.Split(s, ",")
	if len(p) != 3 {
		return [3]float64{}, false
	}
	var v [3]float64
	for i := range p {
		f, err := strconv.ParseFloat(strings.TrimSpace(p[i]), 64)
		if err != nil {
			return [3]float64{}, false
		}
		v[i] = f
	}
	return v, true
}

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint")
	file := flag.String("file", "", "CAD file to import (.step/.stl/.obj/.3mf)")
	format := flag.String("format", "step", "import format")
	out := flag.String("out", "/tmp/oblikovati-capture.png", "PNG output path")
	eye := flag.String("eye", "", "camera eye x,y,z (overrides --home framing)")
	target := flag.String("target", "", "camera target x,y,z (with --eye)")
	home := flag.Bool("home", true, "frame with the Home isometric view (ignored when --eye is set)")
	normals := flag.Bool("normals", true, "enable normal-debug render (front green / back red)")
	faces := flag.Bool("faces", false, "color each B-rep face a distinct color")
	tris := flag.Bool("triangles", false, "color each TRIANGLE a distinct color")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "need --file")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cl := mcp.NewClient(&mcp.Implementation{Name: "mcpshot", Version: "0.1.0"}, nil)
	cs, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: *url}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer cs.Close()
	call := func(name string, args map[string]any) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			os.Exit(1)
		}
		if res.IsError {
			fmt.Fprintf(os.Stderr, "%s: tool error: %v\n", name, res.Content)
			os.Exit(1)
		}
		fmt.Printf("%-18s ok\n", name)
	}
	call("close_all_documents", map[string]any{"force": true})
	call("create_document", map[string]any{"type": "part", "name": "import"})
	call("import_file", map[string]any{"path": *file, "format": *format})
	if e, ok := vec(*eye); ok { // an explicit angle (keep a fixed eye/target across probes for consistency)
		t, _ := vec(*target)
		call("set_camera", map[string]any{"eye": e[:], "target": t[:], "up": []float64{0, 0, 1}, "fov": 0.6})
	} else if *home {
		call("execute_command", map[string]any{"id": "View.Home"}) // default isometric, framed to fit
	}
	call("set_normal_debug", map[string]any{"on": *normals})
	if *faces || *tris {
		call("set_mesh_colors", map[string]any{"on": true, "perTriangle": *tris})
	}
	call("capture_viewport", map[string]any{"path": *out})
	fmt.Printf("captured -> %s\n", *out)
}
