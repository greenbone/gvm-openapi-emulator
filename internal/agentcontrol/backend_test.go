// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcontrol

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greenbone/gvm-openapi-emulator/config"
	"github.com/greenbone/gvm-openapi-emulator/internal/openapi"
	"github.com/greenbone/gvm-openapi-emulator/internal/samples"
	"github.com/sirupsen/logrus"
)

const activeAgentID = "test::2d61a736"

// TestStatefulProfileContractReadBackIsolationAndReset is the issue #49
// regression contract: pagination, grouped mutation, configuration, bundles,
// and agent-backed scans must all read back deterministically and reset cleanly.
func TestStatefulProfileContractReadBackIsolationAndReset(t *testing.T) {
	backend := testBackend(t, 2*time.Millisecond)

	response := request(t, backend, http.MethodGet, "/api/v1/admin/agents?page=2&per_page=2", "", nil)
	assertStatus(t, response, http.StatusOK)
	if response.Header().Get("x-total-count") != "4" || response.Header().Get("x-page") != "2" {
		t.Fatalf("pagination headers do not expose stable inventory size: %#v", response.Header())
	}
	var agents []map[string]any
	decodeResponse(t, response, &agents)
	if len(agents) != 2 || agents[0]["agentid"] != "test::33333" {
		t.Fatalf("unexpected second page: %#v", agents)
	}

	response = request(t, backend, http.MethodGet, "/api/v1/agents/"+activeAgentID+"/config", "", nil)
	assertStatus(t, response, http.StatusUnauthorized)
	response = request(t, backend, http.MethodGet, "/api/v1/agents/"+activeAgentID+"/config", "", map[string]string{
		"Authorization": "Bearer deterministic-test-token",
	})
	assertStatus(t, response, http.StatusOK)
	var agentConfig map[string]any
	decodeResponse(t, response, &agentConfig)
	if agentConfig["heartbeat"] == nil {
		t.Fatalf("agent detail did not expose seeded config: %#v", agentConfig)
	}

	response = request(t, backend, http.MethodPatch, "/api/v1/admin/agents", `{
		"test::2d61a736":{"authorized":false,"update_to_latest":false},
		"test::123123":{"min_interval":120}
	}`, nil)
	assertStatus(t, response, http.StatusOK)
	response = request(t, backend, http.MethodGet, "/api/v1/admin/agents?page=1&per_page=4", "", nil)
	decodeResponse(t, response, &agents)
	first := agentByID(t, agents, activeAgentID)
	if first["authorized"] != false || first["update_to_latest"] != false {
		t.Fatalf("grouped update was not observable: %#v", first)
	}
	second := agentByID(t, agents, "test::123123")
	if second["min_interval"] != float64(120) {
		t.Fatalf("second grouped update was not isolated: %#v", second)
	}

	response = request(t, backend, http.MethodGet,
		"/agent-control/v2/api/agents/"+activeAgentID+"/support-bundle?days=3&encryption=false", "", nil)
	assertStatus(t, response, http.StatusConflict)

	response = request(t, backend, http.MethodPatch, "/api/v1/admin/agents",
		`{"test::2d61a736":{"authorized":true}}`, nil)
	assertStatus(t, response, http.StatusOK)
	response = request(t, backend, http.MethodGet,
		"/agent-control/v2/api/agents/"+activeAgentID+"/support-bundle?days=3&encryption=false", "", nil)
	assertStatus(t, response, http.StatusOK)
	if response.Header().Get("content-type") != "application/octet-stream" ||
		response.Header().Get("x-emulator-support-bundle-revision") != "1" ||
		!strings.Contains(response.Body.String(), "days=3") {
		t.Fatalf("support bundle did not reflect query/state: headers=%#v body=%q",
			response.Header(), response.Body.String())
	}

	response = request(t, backend, http.MethodGet, "/api/v1/admin/scan-agent-config", "", nil)
	assertStatus(t, response, http.StatusOK)
	var defaults map[string]any
	decodeResponse(t, response, &defaults)
	if defaults["agent_defaults"] == nil || defaults["agent_control_defaults"] == nil {
		t.Fatalf("seed configuration is not wrapped per contract: %#v", defaults)
	}
	response = request(t, backend, http.MethodPut, "/api/v1/admin/scan-agent-config",
		`{"agent_control_defaults":{"update_to_latest":false},`+
			`"agent_defaults":{"heartbeat":{"interval_in_seconds":30}}}`, nil)
	assertStatus(t, response, http.StatusOK)
	response = request(t, backend, http.MethodGet, "/api/v1/admin/scan-agent-config", "", nil)
	decodeResponse(t, response, &defaults)
	control := defaults["agent_control_defaults"].(map[string]any)
	if control["update_to_latest"] != false {
		t.Fatalf("configuration update did not read back: %#v", defaults)
	}

	scanBody := `{"target":{"hosts":["192.168.1.101"]},"vts":[{"oid":"1.3.6.1.4.1.25623.1.0.103997"}]}`
	response = request(t, backend, http.MethodPost, "/scans", scanBody, nil)
	assertStatus(t, response, http.StatusCreated)
	var created map[string]any
	decodeResponse(t, response, &created)
	scanID := created["scan_id"].(string)
	if scanID != "00000000-0000-4000-a000-000000000001" {
		t.Fatalf("first scan id is not deterministic: %q", scanID)
	}
	response = request(t, backend, http.MethodGet, "/scans/"+scanID+"/results", "", nil)
	assertStatus(t, response, http.StatusOK)
	var results []scanResult
	decodeResponse(t, response, &results)
	if len(results) == 0 || results[0].IPAddress != "192.168.1.101" ||
		!strings.Contains(results[0].Message, activeAgentID) ||
		!strings.Contains(results[0].Message, scanID) {
		t.Fatalf("scan result lost agent/OpenVASD correlation: %#v", results)
	}
	secondScanBody := `{"target":{"hosts":["192.168.1.102"]},` +
		`"vts":[{"oid":"1.3.6.1.4.1.25623.1.0.100196"}]}`
	response = request(t, backend, http.MethodPost, "/scans", secondScanBody, nil)
	assertStatus(t, response, http.StatusCreated)
	var secondCreated map[string]any
	decodeResponse(t, response, &secondCreated)
	secondScanID := secondCreated["scan_id"].(string)
	response = request(t, backend, http.MethodGet, "/scans/"+secondScanID+"/results", "", nil)
	assertStatus(t, response, http.StatusOK)
	var secondResults []scanResult
	decodeResponse(t, response, &secondResults)
	if len(secondResults) != len(results) || secondResults[0].IPAddress != "192.168.1.102" ||
		!strings.Contains(secondResults[0].Message, "test::123123") ||
		!strings.Contains(secondResults[0].Message, secondScanID) ||
		secondResults[0].Message == results[0].Message {
		t.Fatalf("concurrent scan state was not isolated: first=%#v second=%#v", results, secondResults)
	}

	// A completed scan retains its snapshot when live inventory is mutated.
	response = request(t, backend, http.MethodPost, "/api/v1/admin/agents/delete", `["test::2d61a736"]`, nil)
	assertStatus(t, response, http.StatusOK)
	response = request(t, backend, http.MethodGet, "/scans/"+scanID+"/results", "", nil)
	assertStatus(t, response, http.StatusOK)
	decodeResponse(t, response, &results)
	if !strings.Contains(results[0].Message, activeAgentID) {
		t.Fatalf("stored scan was changed by inventory deletion: %#v", results)
	}

	response = request(t, backend, http.MethodPost, "/_emulator/agent-control/reset", `{}`, map[string]string{
		resetTokenHeader: "reset-secret",
	})
	assertStatus(t, response, http.StatusNoContent)
	response = request(t, backend, http.MethodPost, "/scans", scanBody, nil)
	assertStatus(t, response, http.StatusCreated)
	decodeResponse(t, response, &created)
	if created["scan_id"] != scanID {
		t.Fatalf("reset did not restore deterministic scan IDs: %#v", created)
	}
	response = request(t, backend, http.MethodGet, "/api/v1/admin/agents?page=1&per_page=10", "", nil)
	decodeResponse(t, response, &agents)
	if len(agents) != 4 || agentByID(t, agents, activeAgentID)["authorized"] != true {
		t.Fatalf("reset did not restore declared inventory seed: %#v", agents)
	}
}

// TestGroupedMutationIsAtomicAndRejectsUnknownFixtureKeys prevents partial
// writes and success-biased acceptance of fields outside the pinned schema.
func TestGroupedMutationIsAtomicAndRejectsUnknownFixtureKeys(t *testing.T) {
	backend := testBackend(t, time.Millisecond)
	response := request(t, backend, http.MethodPatch, "/api/v1/admin/agents", `{
		"test::2d61a736":{"authorized":false},
		"missing-agent":{"authorized":true}
	}`, nil)
	assertStatus(t, response, http.StatusUnprocessableEntity)
	response = request(t, backend, http.MethodGet, "/api/v1/admin/agents?page=1&per_page=4", "", nil)
	var agents []map[string]any
	decodeResponse(t, response, &agents)
	if agentByID(t, agents, activeAgentID)["authorized"] != true {
		t.Fatal("atomic grouped update partially modified inventory")
	}

	response = request(t, backend, http.MethodPatch, "/api/v1/admin/agents",
		`{"test::2d61a736":{"unsupported_fixture_key":true}}`, nil)
	assertStatus(t, response, http.StatusBadRequest)
	response = request(t, backend, http.MethodPut, "/api/v1/admin/scan-agent-config",
		`{"unknown_section":{}}`, nil)
	assertStatus(t, response, http.StatusBadRequest)
	response = request(t, backend, http.MethodGet, "/api/v1/admin/agents?page=1&page=2", "", nil)
	assertStatus(t, response, http.StatusBadRequest)
	response = request(t, backend, http.MethodGet,
		"/api/v1/admin/agents?page=2147483647&per_page=1000", "", nil)
	assertStatus(t, response, http.StatusOK)
	decodeResponse(t, response, &agents)
	if len(agents) != 0 {
		t.Fatalf("out-of-range page must be empty, got %#v", agents)
	}
}

// TestNamedFailuresAndFailClosedRouting is the regression boundary that keeps
// every named fault explicit while unknown or unimplemented operations remain
// non-2xx even when an injection header is present.
func TestNamedFailuresAndFailClosedRouting(t *testing.T) {
	backend := testBackend(t, 5*time.Millisecond)
	for scenario, expected := range map[string]int{
		"authentication": http.StatusUnauthorized,
		"unavailable":    http.StatusServiceUnavailable,
		"conflict":       http.StatusConflict,
		"rate-limit":     http.StatusTooManyRequests,
		"timeout":        http.StatusGatewayTimeout,
	} {
		started := time.Now()
		response := request(t, backend, http.MethodGet, "/api/v1/admin/agents", "", map[string]string{
			scenarioHeader: scenario,
		})
		assertStatus(t, response, expected)
		assertErrorBody(t, response)
		if scenario == "rate-limit" && response.Header().Get("retry-after") != "1" {
			t.Fatal("rate-limit scenario omitted Retry-After")
		}
		if scenario == "timeout" && time.Since(started) < 5*time.Millisecond {
			t.Fatal("timeout scenario returned before its configured delay")
		}
	}
	response := request(t, backend, http.MethodGet, "/api/v1/admin/agents", "", map[string]string{
		scenarioHeader: "malformed",
	})
	assertStatus(t, response, http.StatusOK)
	if json.Valid(response.Body.Bytes()) {
		t.Fatalf("malformed scenario returned valid JSON: %q", response.Body.String())
	}

	response = request(t, backend, http.MethodGet, "/api/v1/admin/agents", "", map[string]string{
		scenarioHeader: "unknown-fixture",
	})
	assertStatus(t, response, http.StatusBadRequest)
	response = request(t, backend, http.MethodGet, "/api/v1/certificates/server-cert-fingerprint", "", nil)
	assertStatus(t, response, http.StatusNotImplemented)
	response = request(t, backend, http.MethodGet, "/does-not-exist", "", map[string]string{
		scenarioHeader: "unavailable",
	})
	assertStatus(t, response, http.StatusNotFound)
	response = request(t, backend, http.MethodPost, "/api/v1/admin/agents", `{}`, map[string]string{
		scenarioHeader: "unavailable",
	})
	assertStatus(t, response, http.StatusMethodNotAllowed)
	response = request(t, backend, http.MethodGet, "/health/alive?unexpected=1", "", map[string]string{
		scenarioHeader: "unavailable",
	})
	assertStatus(t, response, http.StatusBadRequest)
	response = request(t, backend, http.MethodGet, "/scans/missing/results", "", nil)
	assertStatus(t, response, http.StatusNotFound)

	response = request(t, backend, http.MethodPost, "/_emulator/agent-control/reset", `{}`, nil)
	assertStatus(t, response, http.StatusUnauthorized)
	response = request(t, backend, http.MethodGet, "/_emulator/agent-control/provenance", "", nil)
	assertStatus(t, response, http.StatusOK)
	assertMetadata(t, response)
	var provenance map[string]any
	decodeResponse(t, response, &provenance)
	if provenance["api_revision"] != openAPIRevision ||
		provenance["fixture_revision"] != fixtureRevision {
		t.Fatalf("provenance endpoint does not expose exact pins: %#v", provenance)
	}
}

func testBackend(t *testing.T, timeout time.Duration) *Backend {
	t.Helper()
	base := filepath.Join("..", "..", "examples", "agent-folder")
	log := logrus.New()
	provider := samples.NewSampleProvider(samples.ProviderConfig{
		BaseDir: filepath.Join(base, "sample"),
		Layout:  config.LayoutFolders,
	}, log)
	specProvider, err := openapi.NewSpecProvider(filepath.Join(base, "swagger.json"), log)
	if err != nil {
		t.Fatalf("load Agent Control spec: %v", err)
	}
	backend, err := New(Config{
		ResetToken:     "reset-secret",
		TimeoutDelay:   timeout,
		SampleProvider: provider,
		RouterProvider: openapi.NewRouterProvider(specProvider.GetSpec()),
	})
	if err != nil {
		t.Fatalf("create Agent Control backend: %v", err)
	}
	return backend
}

func request(
	t *testing.T,
	backend http.Handler,
	method, path, body string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "http://emulator.example"+path, reader)
	if body != "" {
		req.Header.Set("content-type", "application/json")
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	backend.ServeHTTP(response, req)
	assertMetadata(t, response)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %d %q: %v", response.Code, response.Body.String(), err)
	}
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if response.Code != expected {
		t.Fatalf("expected status %d, got %d: %s", expected, response.Code, response.Body.String())
	}
}

func assertMetadata(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	for name, expected := range map[string]string{
		"api-version":                 apiVersion,
		"x-emulator-openapi-revision": openAPIRevision,
		"x-emulator-fixture-revision": fixtureRevision,
	} {
		if actual := response.Header().Get(name); actual != expected {
			t.Fatalf("header %s: expected %q, got %q", name, expected, actual)
		}
	}
}

func assertErrorBody(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]any
	decodeResponse(t, response, &body)
	if message, ok := body["error"].(string); !ok || message == "" {
		t.Fatalf("expected explicit error body, got %#v", body)
	}
}

func agentByID(t *testing.T, agents []map[string]any, id string) map[string]any {
	t.Helper()
	for _, agent := range agents {
		if agent["agentid"] == id {
			return agent
		}
	}
	t.Fatalf("agent %q not found in %#v", id, agents)
	return nil
}
