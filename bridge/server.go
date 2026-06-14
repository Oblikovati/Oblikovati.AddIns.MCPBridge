// SPDX-License-Identifier: GPL-2.0-only

// Package bridge is the pure-Go MCP server for the oblikovati-mcp-bridge add-in: it
// exposes the host's JSON method contract as MCP tools (and, later, resources) over
// streamable HTTP/SSE. Its only dependency on the host is the HostCaller interface,
// so it has no cgo and is fully testable with a fake caller and the SDK's in-memory
// transport.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/client"
)

// version is the add-in/server version reported to MCP clients.
const version = "0.2.0"

// HostCaller is the bridge's sole dependency on the host: forward a JSON method
// request, get a JSON result (or an error carrying the host's message). It is the
// Apache-2.0 contract's [client.Caller] — the same abstraction the typed
// api/client uses — so the cgo layer implements it over the C ABI, tests use a
// fake, and any add-in built on this transport stays decoupled from the GPL host.
type HostCaller = client.Caller

// Server is the MCP server exposed over streamable HTTP. Start/Stop manage the HTTP
// listener; the *mcp.Server holds the registered tools/resources.
type Server struct {
	caller HostCaller
	mcp    *mcp.Server
	http   *http.Server
	events *eventBuffer
	addr   string // resolved listen address, valid after Start
}

// NewServer builds the MCP server and registers all tools. It errors on a nil caller.
func NewServer(caller HostCaller) (*Server, error) {
	if caller == nil {
		return nil, errors.New("bridge: HostCaller is nil")
	}
	impl := &mcp.Implementation{Name: "oblikovati-mcp-bridge", Title: "Oblikovati", Version: version}
	s := &Server{
		caller: caller,
		mcp:    mcp.NewServer(impl, &mcp.ServerOptions{Instructions: instructions}),
		events: &eventBuffer{},
	}
	s.registerTools()
	s.registerResources()
	s.registerEventsResource()
	return s, nil
}

// Start serves the MCP endpoint at /mcp on addr. It binds synchronously (so a port
// conflict surfaces immediately) then serves on a goroutine until Stop.
func (s *Server) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("bridge: listen %s: %w", addr, err)
	}
	s.addr = ln.Addr().String()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcp }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	s.http = &http.Server{Handler: mux}
	go func() { _ = s.http.Serve(ln) }()
	return nil
}

// Addr is the resolved listen address (host:port), valid after Start. Useful when
// Start was given a :0 port.
func (s *Server) Addr() string { return s.addr }

// Stop gracefully shuts the HTTP server down (no-op if never started).
func (s *Server) Stop() error {
	if s.http == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}
