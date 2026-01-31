package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ozgen/openapi-sample-emulator/config"
)

func TestNew_LoadsSpecAndBuildsRoutes(t *testing.T) {
	disableScenarioForTests()

	dir := t.TempDir()
	specPath := writeFile(t, dir, "spec.json", minimalSpec())

	s, err := New(Config{
		Port:           "0",
		SpecPath:       specPath,
		SamplesDir:     dir,
		FallbackMode:   config.FallbackOpenAPIExample,
		ValidationMode: config.ValidationRequired,
		Layout:         config.LayoutFolders,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s == nil {
		t.Fatalf("expected server, got nil")
	}
	if s.specProvider == nil {
		t.Fatalf("expected specProvider")
	}
	if s.routerProvider == nil {
		t.Fatalf("expected routerProvider")
	}
	if s.validator == nil {
		t.Fatalf("expected validator")
	}
	if s.sampleProvider == nil {
		t.Fatalf("expected sampleProvider")
	}

	routes := s.routerProvider.GetRoutes()
	if len(routes) == 0 {
		t.Fatalf("expected routes, got none")
	}
}

func TestHandle_HealthEndpoints(t *testing.T) {
	s := newTestServer(t, config.ValidationRequired, config.FallbackOpenAPIExample)

	for _, p := range []string{"/health/alive", "/health/ready", "/health/started"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com"+p, nil)

		s.handle(rr, req)

		if rr.Code != 200 {
			t.Fatalf("path %s: expected 200, got %d", p, rr.Code)
		}
		var m map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &m)
		if m["ok"] != true {
			t.Fatalf("path %s: expected ok=true, got %v", p, m)
		}
	}
}

func TestHandle_NoRoute_404(t *testing.T) {
	s := newTestServer(t, config.ValidationRequired, config.FallbackOpenAPIExample)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/does-not-exist", nil)

	s.handle(rr, req)

	if rr.Code != 404 {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	var m map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &m)
	if m["error"] != "No route" {
		t.Fatalf("unexpected body: %v", m)
	}
}

func TestHandle_ValidationRequired_EmptyBody_400(t *testing.T) {
	s := newTestServer(t, config.ValidationRequired, config.FallbackOpenAPIExample)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader("   "))

	s.handle(rr, req)

	if rr.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &m)
	if m["error"] != "Bad Request" {
		t.Fatalf("unexpected: %v", m)
	}
}

func TestHandle_ValidationRequired_NonEmptyBody_AllowsSample(t *testing.T) {
	s := newTestServer(t, config.ValidationRequired, config.FallbackOpenAPIExample)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://example.com/items", strings.NewReader(`{"x":1}`))

	s.handle(rr, req)

	if rr.Code != 201 {
		t.Fatalf("expected 201 from sample envelope, got %d", rr.Code)
	}
	if ct := rr.Header().Get("content-type"); ct != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", ct)
	}
	if strings.TrimSpace(rr.Body.String()) != `{"created":true}` {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandle_SampleFound_WritesHeadersStatusBody(t *testing.T) {
	s := newTestServer(t, config.ValidationRequired, config.FallbackOpenAPIExample)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/items/123", nil)

	s.handle(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("x-sample") != "1" {
		t.Fatalf("expected x-sample header, got %q", rr.Header().Get("x-sample"))
	}
	if strings.TrimSpace(rr.Body.String()) != `{"id":"123"}` {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandle_SampleMissing_FallbackOpenAPIExample_200(t *testing.T) {
	disableScenarioForTests()

	dir := t.TempDir()
	specPath := writeFile(t, dir, "spec.json", minimalSpec())

	s, err := New(Config{
		Port:           "0",
		SpecPath:       specPath,
		SamplesDir:     dir,
		FallbackMode:   config.FallbackOpenAPIExample,
		ValidationMode: config.ValidationRequired,
		Layout:         config.LayoutFolders,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/items/999", nil)

	s.handle(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("content-type") != "application/json" {
		t.Fatalf("expected application/json")
	}
	if strings.TrimSpace(rr.Body.String()) != `{"id":"example"}` {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandle_SampleMissing_NoFallback_501(t *testing.T) {
	disableScenarioForTests()

	dir := t.TempDir()
	specPath := writeFile(t, dir, "spec.json", minimalSpec())

	s, err := New(Config{
		Port:           "0",
		SpecPath:       specPath,
		SamplesDir:     dir,
		FallbackMode:   config.FallbackNone,
		ValidationMode: config.ValidationRequired,
		Layout:         config.LayoutFolders,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/items/1", nil)

	s.handle(rr, req)

	if rr.Code != 501 {
		t.Fatalf("expected 501, got %d", rr.Code)
	}
	var m map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &m)

	if m["error"] != "No sample file for route" {
		t.Fatalf("unexpected: %v", m)
	}
	if m["swaggerPath"] != "/items/{id}" {
		t.Fatalf("expected swaggerPath=/items/{id}, got %v", m["swaggerPath"])
	}
	if _, ok := m["legacyFlatFilename"]; !ok {
		t.Fatalf("legacyFlatFilename missing: %v", m)
	}
	if m["layout"] != string(config.LayoutFolders) {
		t.Fatalf("expected layout=%q, got %v", string(config.LayoutFolders), m["layout"])
	}
}

func TestDebugRoutes_NotEmptyAndContainsMappings(t *testing.T) {
	s := newTestServer(t, config.ValidationRequired, config.FallbackOpenAPIExample)

	out := s.DebugRoutes()
	if out == "" {
		t.Fatalf("expected non-empty debug routes")
	}
	if !strings.Contains(out, "GET /items/{id} ->") {
		t.Fatalf("unexpected DebugRoutes output:\n%s", out)
	}
	if !strings.Contains(out, "POST /items ->") {
		t.Fatalf("unexpected DebugRoutes output:\n%s", out)
	}
}

func newTestServer(t *testing.T, validation config.ValidationMode, fallback config.FallbackMode) *Server {
	t.Helper()
	disableScenarioForTests()

	dir := t.TempDir()
	specPath := writeFile(t, dir, "spec.json", minimalSpec())

	writeFileWithDirs(t, dir, filepath.Join("items", "{id}", "GET.json"), `{
	  "status": 200,
	  "headers": {"content-type":"application/json","x-sample":"1"},
	  "body": {"id":"123"}
	}`)

	writeFileWithDirs(t, dir, filepath.Join("items", "POST.json"),
		`{"status":201,"body":{"created":true}}`)

	s, err := New(Config{
		Port:           "0",
		SpecPath:       specPath,
		SamplesDir:     dir,
		FallbackMode:   fallback,
		ValidationMode: validation,
		Layout:         config.LayoutFolders,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func disableScenarioForTests() {
	config.Envs.Scenario.Enabled = false
	if strings.TrimSpace(config.Envs.Scenario.Filename) == "" {
		config.Envs.Scenario.Filename = "scenario.json"
	}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func writeFileWithDirs(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func minimalSpec() string {
	// OAS3:
	// - GET /items/{id} has a 200 response example used for fallback mode
	// - POST /items has requestBody required=true so validation branch is exercised
	return `{
	  "openapi":"3.0.3",
	  "info":{"title":"t","version":"1"},
	  "paths":{
		"/items/{id}":{
		  "get":{
			"responses":{
			  "200":{
				"description":"ok",
				"content":{
				  "application/json":{
					"example":{"id":"example"}
				  }
				}
			  }
			}
		  }
		},
		"/items":{
		  "post":{
			"requestBody":{
			  "required": true,
			  "content":{
				"application/json":{
				  "schema":{"type":"object"}
				}
			  }
			},
			"responses":{
			  "201":{
				"description":"created",
				"content":{
				  "application/json":{
					"example":{"created":true}
				  }
				}
			  }
			}
		  }
		}
	  }
	}`
}
