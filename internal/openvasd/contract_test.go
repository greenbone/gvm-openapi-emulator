// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package openvasd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestPinnedOpenAPIContract keeps the profile's externally observable status
// and query contract tied to the checked-in OpenVASD document. Runtime tests in
// backend_test.go exercise the same operations and responses.
func TestPinnedOpenAPIContract(t *testing.T) {
	specPath := filepath.Join("..", "..", "examples", "openvasd", "swagger.json")
	document, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load pinned OpenAPI document: %v", err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate pinned OpenAPI document: %v", err)
	}
	if revision := document.Info.Extensions["x-upstream-openapi-revision"]; revision != openAPIRevision {
		t.Fatalf("upstream OpenAPI revision: expected %q, got %#v", openAPIRevision, revision)
	}
	if revision := document.Info.Extensions["x-emulator-fixture-revision"]; revision != fixtureRevision {
		t.Fatalf("fixture revision: expected %q, got %#v", fixtureRevision, revision)
	}

	expected := map[string]map[string][]string{
		"/health":            {"HEAD": {"200"}},
		"/health/alive":      {"GET": {"200", "503"}},
		"/health/ready":      {"GET": {"200", "503"}},
		"/health/started":    {"GET": {"200", "503"}},
		"/scans":             {"HEAD": {"200", "401"}, "POST": {"201", "400", "409"}},
		"/scans/preferences": {"GET": {"200"}},
		"/scans/{id}": {
			"GET":  {"200", "404"},
			"POST": {"204", "400", "404", "409", "500"}, "DELETE": {"204", "404", "409"},
		},
		"/scans/{id}/results":       {"GET": {"200", "400", "404"}},
		"/scans/{id}/results/{rid}": {"GET": {"200", "400", "404"}},
		"/scans/{id}/status":        {"GET": {"200", "404"}},
		"/vts":                      {"HEAD": {"200"}, "GET": {"200", "503"}},
	}

	for path, operations := range expected {
		pathItem := document.Paths.Find(path)
		if pathItem == nil {
			t.Errorf("pinned contract is missing path %s", path)
			continue
		}
		for method, statuses := range operations {
			operation := pathItem.GetOperation(method)
			if operation == nil {
				t.Errorf("pinned contract is missing %s %s", method, path)
				continue
			}
			for _, status := range statuses {
				if operation.Responses.Value(status) == nil {
					t.Errorf("pinned contract is missing %s response for %s %s", status, method, path)
				}
			}
		}
	}

	vts := document.Paths.Find("/vts").Get
	if vts == nil || len(vts.Parameters) != 1 || vts.Parameters[0].Value == nil ||
		vts.Parameters[0].Value.Name != "information" {
		t.Fatalf("GET /vts must pin the information query parameter")
	}
}
