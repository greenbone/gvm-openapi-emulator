// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcontrol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestPinnedAgentControlAPIAndFixtureProvenance makes the PR #44 inputs and the
// issue #49 behavior revision independently verifiable by downstream jobs.
func TestPinnedAgentControlAPIAndFixtureProvenance(t *testing.T) {
	base := filepath.Join("..", "..", "examples", "agent-folder")
	data, err := os.ReadFile(filepath.Join(base, "swagger.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Swagger string `json:"swagger"`
		Info    struct {
			Upstream string `json:"x-upstream-openapi-revision"`
			Fixture  string `json:"x-emulator-fixture-revision"`
			Baseline string `json:"x-emulator-baseline-revision"`
		} `json:"info"`
		Paths map[string]map[string]struct {
			Responses map[string]any `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode pinned Swagger: %v", err)
	}
	if document.Swagger != "2.0" || document.Info.Upstream != openAPIRevision ||
		document.Info.Fixture != fixtureRevision || document.Info.Baseline != baselineRevision {
		t.Fatalf("unexpected API provenance: %#v", document.Info)
	}

	expected := map[string]map[string][]string{
		"/api/v1/admin/agents": {
			"get":   {"200", "500"},
			"patch": {"200", "400", "422", "500"},
		},
		"/api/v1/admin/agents/delete": {
			"post": {"200", "400", "422", "500"},
		},
		"/api/v1/admin/scan-agent-config": {
			"get": {"200", "500"},
			"put": {"200", "400", "500"},
		},
		"/api/v1/agents/{agentid}/config": {
			"get": {"200"},
		},
		"/agent-control/v2/api/agents/{agentid}/support-bundle": {
			"get": {"200", "404", "500"},
		},
		"/scans": {
			"get":  {"200", "500"},
			"post": {"201", "400", "500"},
		},
		"/scans/{id}/results": {
			"get": {"200", "400", "404", "500"},
		},
	}
	for path, operations := range expected {
		pathItem, ok := document.Paths[path]
		if !ok {
			t.Errorf("pinned contract is missing path %s", path)
			continue
		}
		for method, statuses := range operations {
			operation, ok := pathItem[method]
			if !ok {
				t.Errorf("pinned contract is missing %s %s", method, path)
				continue
			}
			for _, status := range statuses {
				if _, ok := operation.Responses[status]; !ok {
					t.Errorf("pinned contract is missing %s response for %s %s", status, method, path)
				}
			}
		}
	}

	fixtures := []struct {
		name   string
		digest string
	}{
		{
			name:   filepath.Join("sample", "api", "v1", "admin", "agents", "GET.json"),
			digest: "3a7737dbc6ed6344d7ee23cd97c3a621614db53bb72e073acf604efac54d590b",
		},
		{
			name:   filepath.Join("sample", "api", "v1", "admin", "scan-agent-config", "GET.json"),
			digest: "29e31fa2cb09ee4954e474d44847b9ab634f24ea140690d620e375bd01d15bf4",
		},
		{
			name:   filepath.Join("sample", "scans", "{id}", "results", "GET.json"),
			digest: "1874e83212b633d2434938850d25d81130928a87815aabe44fb00dae5b3a3f60",
		},
	}
	for _, expected := range fixtures {
		fixture, err := os.ReadFile(filepath.Join(base, expected.name))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(fixture)
		if actual := hex.EncodeToString(digest[:]); actual != expected.digest {
			t.Errorf("fixture %s: expected SHA-256 %s, got %s", expected.name, expected.digest, actual)
		}
	}
}
