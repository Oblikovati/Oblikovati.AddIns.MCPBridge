// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolDescriptor is the JSON-serializable metadata registered with WebMCP
// (`navigator.modelContext.registerTool`) and browser host bridges.
type ToolDescriptor struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

// InMemoryBridge exposes a [Server] over an in-memory MCP transport so browser
// WebAssembly hosts (which cannot bind TCP sockets) can list tools, invoke tools,
// and forward MCP JSON-RPC 2.0 messages directly.
type InMemoryBridge struct {
	server  *Server
	session *mcp.ClientSession

	mu     sync.Mutex
	closed bool
}

// NewInMemoryBridge connects an in-memory MCP client session to s.
//
//	ib, err := bridge.NewInMemoryBridge(srv)
//	defer ib.Close()
func NewInMemoryBridge(s *Server) (*InMemoryBridge, error) {
	if s == nil {
		return nil, errors.New("bridge: server is nil, expected a non-nil *Server")
	}
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := s.mcp.Connect(ctx, serverT, nil); err != nil {
		return nil, fmt.Errorf("bridge: in-memory server connect: %w", err)
	}
	cli := mcp.NewClient(&mcp.Implementation{Name: "oblikovati-web-host", Version: version}, nil)
	session, err := cli.Connect(ctx, clientT, nil)
	if err != nil {
		return nil, fmt.Errorf("bridge: in-memory client connect: %w", err)
	}
	return &InMemoryBridge{server: s, session: session}, nil
}

// Tools returns all registered MCP tools for WebMCP registration and discovery.
func (b *InMemoryBridge) Tools(ctx context.Context) ([]ToolDescriptor, error) {
	res, err := b.session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	out := make([]ToolDescriptor, len(res.Tools))
	for i, tool := range res.Tools {
		out[i] = ToolDescriptor{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}
	return out, nil
}

// CallTool invokes one registered MCP tool by name with JSON-decoded arguments.
func (b *InMemoryBridge) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	if name == "" {
		return nil, errors.New("bridge: tool name is empty, expected a registered tool name")
	}
	return b.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

// HandleAutomation dispatches browser-host automation methods (`mcp.listTools`,
// `mcp.callTool`, `mcp.jsonrpc`) against the in-memory MCP session.
func (b *InMemoryBridge) HandleAutomation(ctx context.Context, method string, req []byte) ([]byte, error) {
	switch method {
	case "mcp.listTools":
		tools, err := b.Tools(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"tools": tools})
	case "mcp.callTool":
		return b.handleCallToolJSON(ctx, req)
	case "mcp.jsonrpc":
		return b.handleJSONRPC(ctx, req)
	default:
		return nil, fmt.Errorf("bridge: unknown automation method %q, expected mcp.listTools, mcp.callTool, or mcp.jsonrpc", method)
	}
}

// Close shuts down the in-memory MCP client session idempotently.
func (b *InMemoryBridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	return b.session.Close()
}

type callToolRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func (b *InMemoryBridge) handleCallToolJSON(ctx context.Context, req []byte) ([]byte, error) {
	var in callToolRequest
	if err := json.Unmarshal(req, &in); err != nil {
		return nil, fmt.Errorf("bridge: invalid mcp.callTool request: %w", err)
	}
	res, err := b.CallTool(ctx, in.Name, in.Arguments)
	if err != nil {
		return nil, err
	}
	return json.Marshal(res)
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (b *InMemoryBridge) handleJSONRPC(ctx context.Context, req []byte) ([]byte, error) {
	var rpc jsonRPCRequest
	if err := json.Unmarshal(req, &rpc); err != nil {
		return marshalJSONRPCError(nil, -32700, "parse error: "+err.Error())
	}
	result, err := b.dispatchJSONRPCMethod(ctx, rpc.Method, rpc.Params)
	if err != nil {
		return marshalJSONRPCError(rpc.ID, -32603, err.Error())
	}
	if len(rpc.ID) == 0 {
		return []byte{}, nil
	}
	return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: rpc.ID, Result: result})
}

func (b *InMemoryBridge) dispatchJSONRPCMethod(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		return initializeResult(), nil
	case "ping", "notifications/initialized":
		return map[string]any{}, nil
	case "tools/list":
		return b.session.ListTools(ctx, nil)
	case "tools/call":
		return b.dispatchToolsCall(ctx, params)
	case "resources/list":
		return b.session.ListResources(ctx, nil)
	case "resources/read":
		return b.dispatchResourcesRead(ctx, params)
	default:
		return nil, fmt.Errorf("method %q not found", method)
	}
}

func initializeResult() map[string]any {
	return map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
		"serverInfo":      map[string]any{"name": "oblikovati-mcp-bridge", "version": version},
	}
}

func (b *InMemoryBridge) dispatchToolsCall(ctx context.Context, params json.RawMessage) (any, error) {
	var in callToolRequest
	if err := json.Unmarshal(params, &in); err != nil {
		return nil, fmt.Errorf("invalid tools/call params: %w", err)
	}
	return b.CallTool(ctx, in.Name, in.Arguments)
}

func (b *InMemoryBridge) dispatchResourcesRead(ctx context.Context, params json.RawMessage) (any, error) {
	var in struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &in); err != nil {
		return nil, fmt.Errorf("invalid resources/read params: %w", err)
	}
	return b.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: in.URI})
}

func marshalJSONRPCError(id json.RawMessage, code int, msg string) ([]byte, error) {
	return json.Marshal(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: msg},
	})
}
