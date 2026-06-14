// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStaticDocResource(t *testing.T) {
	cs := connect(t, &fakeHost{reply: []byte("{}")})
	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "oblikovati://docs/getting-started"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) == 0 || !strings.Contains(res.Contents[0].Text, "get_model_tree") {
		t.Fatalf("getting-started resource missing expected content: %+v", res.Contents)
	}
}

func TestLiveSchemaResource(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"kinds":[{"kind":"extrude"}]}`)}
	cs := connect(t, host)
	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "oblikovati://schema/feature-kinds"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if host.lastMethod != "features.list" {
		t.Fatalf("schema resource called %q, want features.list", host.lastMethod)
	}
	if len(res.Contents) == 0 || !strings.Contains(res.Contents[0].Text, "extrude") {
		t.Fatalf("feature-kinds resource = %+v, want host reply", res.Contents)
	}
}

func TestRibbonSchemaResource(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"key":"Part","tabs":[]}`)}
	cs := connect(t, host)
	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "oblikovati://schema/ribbon"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if host.lastMethod != "ribbon.list" {
		t.Fatalf("ribbon resource called %q, want ribbon.list", host.lastMethod)
	}
	if len(res.Contents) == 0 || !strings.Contains(res.Contents[0].Text, "Part") {
		t.Fatalf("ribbon resource = %+v, want host reply", res.Contents)
	}
}

func TestResourcesListed(t *testing.T) {
	cs := connect(t, &fakeHost{reply: []byte("{}")})
	res, err := cs.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(res.Resources) < 9 {
		t.Fatalf("listed %d resources, want >= 9", len(res.Resources))
	}
}
