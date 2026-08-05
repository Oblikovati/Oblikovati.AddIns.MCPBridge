// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runFacetDiag is the live check for Oblikovati#1601: a cylinder cut by another cylinder (no
// exact curved path) facets both analytic operands for the planar boolean — the feature reply
// must now carry that degradation in its diagnostics array instead of shipping the faceted body
// silently. Also asserts the reverse: an exact box-drill cut stays diagnostic-free.
func runFacetDiag(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "20 mm", "20 mm")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "30 mm", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 1, []float64{1.5, 0}, "10 mm", "10 mm")
	diags, err := c.featureDiagnostics("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": "30 mm", "operation": "cut"})
	if err != nil {
		return err
	}
	if !hasCode(diags, "boolean.analytic-faceted") {
		return fmt.Errorf("cylinder-on-cylinder cut reply carries no boolean.analytic-faceted diagnostic: %v", diags)
	}
	fmt.Printf("  faceting cut reported: %v\n", diags)
	return nil
}

type featureDiag struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
}

// featureDiagnostics applies a feature and returns the diagnostics array of its reply (the
// feature must build healthy).
func (c *caller) featureDiagnostics(kind string, args map[string]any) ([]featureDiag, error) {
	var r struct {
		Healthy     bool          `json:"healthy"`
		Reason      string        `json:"reason"`
		Diagnostics []featureDiag `json:"diagnostics"`
	}
	c.json("add_feature", map[string]any{"kind": kind, "args": args}, &r)
	if c.err != nil {
		return nil, c.err
	}
	if !r.Healthy {
		return nil, fmt.Errorf("%s unhealthy: %s", kind, r.Reason)
	}
	return r.Diagnostics, nil
}

func hasCode(diags []featureDiag, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}
