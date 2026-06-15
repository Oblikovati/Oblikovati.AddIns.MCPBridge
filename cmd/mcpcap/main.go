// SPDX-License-Identifier: GPL-2.0-only

// Command mcpcap is a minimal capture tool: it points the live viewport camera at --eye/--target
// and writes a PNG, without touching the model. Used to inspect a region (e.g. a loft seam) of
// whatever is already built. Usage: mcpcap --eye x,y,z --target x,y,z --out /tmp/shot.png
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

func vec(s string) []float64 {
	p := strings.Split(s, ",")
	out := make([]float64, len(p))
	for i := range p {
		out[i], _ = strconv.ParseFloat(strings.TrimSpace(p[i]), 64)
	}
	return out
}

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint")
	eye := flag.String("eye", "8,8,8", "camera eye x,y,z (cm)")
	target := flag.String("target", "0,0,0", "camera target x,y,z (cm)")
	up := flag.String("up", "0,0,1", "camera up vector")
	fov := flag.Float64("fov", 0.6, "field of view (rad)")
	out := flag.String("out", "/tmp/oblikovati-cap.png", "PNG output path")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cl := mcp.NewClient(&mcp.Implementation{Name: "mcpcap", Version: "0.1.0"}, nil)
	cs, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: *url}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer cs.Close()
	call := func(name string, args map[string]any) {
		if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args}); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			os.Exit(1)
		}
	}
	call("set_camera", map[string]any{"eye": vec(*eye), "target": vec(*target), "up": vec(*up), "fov": *fov})
	call("capture_viewport", map[string]any{"path": *out})
	fmt.Printf("captured -> %s\n", *out)
}
