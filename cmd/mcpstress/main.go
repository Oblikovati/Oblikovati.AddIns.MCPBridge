// SPDX-License-Identifier: GPL-2.0-only

// Command mcpstress drives a running oblikovati-mcp-bridge endpoint hard to surface kernel
// bugs and performance problems: it builds large models (N parameters, an N-entity sketch, N
// extruded features), measures per-operation latency (p50/p95/max) per phase, and tails the
// host operation trace (tail_logs) for errors and recovered panics. It exits non-zero if any
// panic is caught (a kernel bug) — so it doubles as a CI smoke against a live app.
//
// Usage: mcpstress [--url http://127.0.0.1:7800/mcp] [-n 200] [-op new|join]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	n := flag.Int("n", 200, "scale: parameters, sketch entities, and features to create")
	op := flag.String("op", "new", "extrude boolean operation: new (independent bodies) or join (stresses the boolean kernel)")
	flag.Parse()
	if err := run(*url, *n, *op); err != nil {
		fmt.Fprintln(os.Stderr, "mcpstress:", err)
		os.Exit(1)
	}
}

type driver struct {
	ctx   context.Context
	cs    *mcp.ClientSession
	since uint64 // tail_logs cursor
}

func run(url string, n int, op string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpstress", Version: "0.2.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "mcpstress: close session:", closeErr)
		}
	}()

	d := &driver{ctx: ctx, cs: cs}
	fmt.Printf("stress: n=%d op=%s @ %s\n\n", n, op, url)
	if err := d.must("create_document", map[string]any{"type": "part", "name": "stress"}); err != nil {
		return err
	}

	d.phase("parameters", n, func(i int) (string, map[string]any) {
		return "add_parameter", map[string]any{"name": fmt.Sprintf("p%d", i), "expression": fmt.Sprintf("%d mm", i+1)}
	})
	d.sketchPhase(n)
	d.featurePhase(n, op)

	errs, panics := d.drainTrace()
	fmt.Printf("\ntrace: %d error record(s), %d panic(s)\n", errs, panics)
	if panics > 0 {
		return fmt.Errorf("%d kernel panic(s) caught — see the trace dump above", panics)
	}
	return nil
}

// phase runs n calls of one kind, timing each, then prints a latency summary and a trace
// scan for that phase.
func (d *driver) phase(name string, n int, build func(i int) (tool string, args map[string]any)) {
	lat := make([]time.Duration, 0, n)
	failed := 0
	for i := 0; i < n; i++ {
		tool, args := build(i)
		start := time.Now()
		isErr := d.call(tool, args)
		lat = append(lat, time.Since(start))
		if isErr {
			failed++
		}
	}
	report(name, lat, failed)
	d.scanTrace(name)
}

// sketchPhase builds one sketch and adds n entities (alternating lines and circles), then solves.
func (d *driver) sketchPhase(n int) {
	_ = d.must("create_sketch", map[string]any{"plane": "XY"})
	d.phase("sketch entities", n, func(i int) (string, map[string]any) {
		if i%2 == 0 {
			return "add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "line",
				"points": [][]float64{{float64(i), 0}, {float64(i) + 1, 1}}}
		}
		return "add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "circle",
			"points": [][]float64{{float64(i), float64(i)}}, "radius": "1 mm"}
	})
	start := time.Now()
	d.call("solve_sketch", map[string]any{"sketchIndex": 0})
	fmt.Printf("  solve %d-entity sketch: %v\n", n, time.Since(start).Round(time.Microsecond))
}

// featurePhase creates n rectangle sketches and extrudes each, stressing feature recompute
// (and the boolean kernel when op=join).
func (d *driver) featurePhase(n int, op string) {
	lat := make([]time.Duration, 0, n)
	failed := 0
	for i := 0; i < n; i++ {
		var created struct {
			SketchIndex int `json:"sketchIndex"`
		}
		if !d.callJSON("create_sketch", map[string]any{"plane": "XY"}, &created) {
			failed++
			continue
		}
		d.call("sketch_rectangle", map[string]any{"sketchIndex": created.SketchIndex, "width": "4 mm", "height": "3 mm"})
		start := time.Now()
		isErr := d.call("add_feature", map[string]any{"kind": "extrude",
			"args": map[string]any{"sketchIndex": created.SketchIndex, "profileIndex": 0, "distance": "2 mm", "operation": op}})
		lat = append(lat, time.Since(start))
		if isErr {
			failed++
		}
	}
	report("features (extrude)", lat, failed)
	d.scanTrace("features")
}

// call invokes a tool, returning whether the host reported an error (a transport/schema error
// is fatal — that is a bridge problem, not a kernel one).
func (d *driver) call(tool string, args map[string]any) (isErr bool) {
	res, err := d.cs.CallTool(d.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s transport error: %v\n", tool, err)
		return true
	}
	return res.IsError
}

func (d *driver) callJSON(tool string, args map[string]any, v any) bool {
	res, err := d.cs.CallTool(d.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil || res.IsError {
		return false
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return json.Unmarshal([]byte(tc.Text), v) == nil
		}
	}
	return false
}

func (d *driver) must(tool string, args map[string]any) error {
	if d.call(tool, args) {
		return fmt.Errorf("%s failed", tool)
	}
	return nil
}

// scanTrace tails new trace records and prints this phase's error/panic count plus its slowest
// recorded operation.
func (d *driver) scanTrace(phase string) {
	recs := d.tail()
	var errs, panics int
	var slow wire.LogRecord
	for _, r := range recs {
		if r.Panic != "" {
			panics++
		} else if !r.OK && r.Method != "" {
			errs++
		}
		if r.DurationMicros > slow.DurationMicros {
			slow = r
		}
	}
	fmt.Printf("  [trace %-16s] %d err, %d panic; slowest %s %s\n", phase, errs, panics,
		slow.Method, time.Duration(slow.DurationMicros)*time.Microsecond)
}

// drainTrace reads any remaining trace and prints every panic (with stack) and a final tally.
func (d *driver) drainTrace() (errs, panics int) {
	for _, r := range d.tail() {
		if r.Panic != "" {
			panics++
			fmt.Printf("\nPANIC in %s: %s\n%s\n", r.Method, r.Panic, r.Stack)
		} else if !r.OK && r.Method != "" {
			errs++
		}
	}
	return errs, panics
}

// tail fetches trace records newer than the cursor and advances it.
func (d *driver) tail() []wire.LogRecord {
	var out wire.LogsResult
	if !d.callJSON("tail_logs", map[string]any{"sinceSeq": d.since, "max": 100000}, &out) {
		return nil
	}
	d.since = out.NextSeq
	return out.Records
}

// report prints a latency summary (count, failures, p50/p95/max, throughput) for a phase.
func report(name string, lat []time.Duration, failed int) {
	if len(lat) == 0 {
		fmt.Printf("%-20s no samples\n", name)
		return
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	var total time.Duration
	for _, d := range lat {
		total += d
	}
	rate := float64(len(lat)) / total.Seconds()
	fmt.Printf("%-20s n=%d fail=%d  p50=%s p95=%s max=%s  %.0f ops/s\n",
		name, len(lat), failed,
		lat[len(lat)/2].Round(time.Microsecond),
		lat[(len(lat)*95)/100].Round(time.Microsecond),
		lat[len(lat)-1].Round(time.Microsecond), rate)
}
