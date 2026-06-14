// SPDX-License-Identifier: GPL-2.0-only

// Command mcpcheck connects to a running oblikovati-mcp-bridge MCP endpoint and runs
// a read-only smoke check: initialize, list the tools, and read the active part's
// model tree. It is a convenience for verifying a live bridge (the same path an LLM
// client takes) without a full MCP client.
//
// Usage: mcpcheck [--url http://127.0.0.1:7800/mcp]
package main

import (
	"context"
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
		fmt.Fprintln(os.Stderr, "mcpcheck:", err)
		os.Exit(1)
	}
}

func run(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "mcpcheck", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "mcpcheck: close session:", closeErr)
		}
	}()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	fmt.Printf("connected: %d tools\n", len(tools.Tools))

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_model_tree"})
	if err != nil {
		return fmt.Errorf("get_model_tree: %w", err)
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			fmt.Println("model_tree:", tc.Text)
		}
	}
	return nil
}
