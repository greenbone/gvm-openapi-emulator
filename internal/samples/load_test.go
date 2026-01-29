package samples

import (
	"github.com/ozgen/openapi-sample-emulator/config"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ReadError(t *testing.T) {
	_, err := Load("/no/such/dir", "missing.json")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoad_EmptyFile_ReturnsDefaultJSONEnvelope(t *testing.T) {
	dir := t.TempDir()
	name := "empty.json"

	writeFile(t, dir, name, "   \n\t  ")

	resp, err := Load(dir, name)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if resp.Status != 200 {
		t.Fatalf("expected 200, got %d", resp.Status)
	}
	if resp.Headers == nil || resp.Headers["content-type"] != "application/json" {
		t.Fatalf("expected default content-type, got %#v", resp.Headers)
	}
	if string(resp.Body) != "{}" {
		t.Fatalf("expected body {}, got %q", string(resp.Body))
	}
}

func TestLoad_Envelope_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	name := "sample.json"

	writeFile(t, dir, name, `{"body":{"ok":true}}`)

	resp, err := Load(dir, name)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if resp.Status != 200 {
		t.Fatalf("expected default status 200, got %d", resp.Status)
	}
	if resp.Headers == nil || resp.Headers["content-type"] != "application/json" {
		t.Fatalf("expected default content-type header, got %#v", resp.Headers)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
}

func TestLoad_Envelope_ExplicitValues(t *testing.T) {
	dir := t.TempDir()
	name := "sample.json"

	writeFile(t, dir, name, `{
	  "status": 201,
	  "headers": {"content-type":"application/problem+json","x-test":"1"},
	  "body": {"id": 123}
	}`)

	resp, err := Load(dir, name)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if resp.Status != 201 {
		t.Fatalf("expected 201, got %d", resp.Status)
	}
	if resp.Headers["content-type"] != "application/problem+json" || resp.Headers["x-test"] != "1" {
		t.Fatalf("unexpected headers: %#v", resp.Headers)
	}
	if string(resp.Body) != `{"id":123}` {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
}

func TestLoad_Envelope_StatusOnly_DefaultsHeaderAndBody(t *testing.T) {
	dir := t.TempDir()
	name := "sample.json"

	writeFile(t, dir, name, `{"status":204}`)

	resp, err := Load(dir, name)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if resp.Status != 204 {
		t.Fatalf("expected 204, got %d", resp.Status)
	}
	if resp.Headers["content-type"] != "application/json" {
		t.Fatalf("expected default content-type, got %#v", resp.Headers)
	}
	if string(resp.Body) != "{}" {
		t.Fatalf("expected body {}, got %q", string(resp.Body))
	}
}

func TestLoad_Envelope_WhenHeadersPresentButBodyMissing(t *testing.T) {
	dir := t.TempDir()
	name := "hdrs.json"

	writeFile(t, dir, name, `{"headers":{"content-type":"text/plain"}}`)

	resp, err := Load(dir, name)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("expected default status 200, got %d", resp.Status)
	}
	if resp.Headers["content-type"] != "text/plain" {
		t.Fatalf("unexpected headers: %#v", resp.Headers)
	}
	if string(resp.Body) != "{}" {
		t.Fatalf("expected body {}, got %q", string(resp.Body))
	}
}

func TestLoad_Envelope_ContentType_DefaultNotAddedIfAlreadyPresentCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	name := "case.json"

	writeFile(t, dir, name, `{
	  "headers": {"Content-Type":"text/plain"},
	  "body": {"ok": true}
	}`)

	resp, err := Load(dir, name)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if resp.Headers["Content-Type"] != "text/plain" {
		t.Fatalf("expected Content-Type=text/plain, got %#v", resp.Headers)
	}
	if _, ok := resp.Headers["content-type"]; ok {
		t.Fatalf("did not expect injected lowercase content-type when Content-Type already exists: %#v", resp.Headers)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
}

func TestLoad_Fallback_PlainJSONOrText(t *testing.T) {
	dir := t.TempDir()

	t.Run("raw json without envelope", func(t *testing.T) {
		name := "raw.json"
		writeFile(t, dir, name, `{}`)

		resp, err := Load(dir, name)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if resp.Status != 200 {
			t.Fatalf("expected 200, got %d", resp.Status)
		}
		if resp.Headers["content-type"] != "application/json" {
			t.Fatalf("expected application/json, got %#v", resp.Headers)
		}
		if string(resp.Body) != `{}` {
			t.Fatalf("unexpected body: %q", string(resp.Body))
		}
	})

	t.Run("plain text", func(t *testing.T) {
		name := "raw.txt"
		writeFile(t, dir, name, `  hello world  `)

		resp, err := Load(dir, name)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if resp.Status != 200 {
			t.Fatalf("expected 200, got %d", resp.Status)
		}
		if resp.Headers["content-type"] != "application/json" {
			t.Fatalf("expected application/json, got %#v", resp.Headers)
		}
		if string(resp.Body) != `hello world` {
			t.Fatalf("unexpected body: %q", string(resp.Body))
		}
	})
}

func TestLoadResolved_FoldersMode_LoadsFolderSample(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerPath := "/api/v1/items"
	legacyFlat := "GET_api_v1_items.json"

	writeFile(t, baseDir, filepath.Join("api", "v1", "items", "GET.json"), `{"body":{"ok":true}}`)

	resp, err := LoadResolved(baseDir, method, swaggerPath, legacyFlat, "", config.LayoutFolders)
	if err != nil {
		t.Fatalf("LoadResolved: %v", err)
	}

	if resp.Status != 200 {
		t.Fatalf("expected 200, got %d", resp.Status)
	}
	if resp.Headers["content-type"] != "application/json" {
		t.Fatalf("expected application/json, got %#v", resp.Headers)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
}

func TestLoadResolved_FlatMode_LoadsLegacyFlatSample(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerPath := "/api/v1/items"
	legacyFlat := "GET_api_v1_items.json"

	writeFile(t, baseDir, legacyFlat, `{"body":{"from":"flat"}}`)

	resp, err := LoadResolved(baseDir, method, swaggerPath, legacyFlat, "", config.LayoutFlat)
	if err != nil {
		t.Fatalf("LoadResolved: %v", err)
	}

	if string(resp.Body) != `{"from":"flat"}` {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
}

func TestLoadResolved_AutoMode_PrefersFoldersOverFlat(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerPath := "/api/v1/items"
	legacyFlat := "GET_api_v1_items.json"

	writeFile(t, baseDir, filepath.Join("api", "v1", "items", "GET.json"), `{"body":{"from":"folders"}}`)
	writeFile(t, baseDir, legacyFlat, `{"body":{"from":"flat"}}`)

	resp, err := LoadResolved(baseDir, method, swaggerPath, legacyFlat, "", config.LayoutAuto)
	if err != nil {
		t.Fatalf("LoadResolved: %v", err)
	}

	if string(resp.Body) != `{"from":"folders"}` {
		t.Fatalf("expected folders sample to win, got body: %q", string(resp.Body))
	}
}

func TestLoadResolved_MissingSample_ReturnsError(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerPath := "/api/v1/does-not-exist"
	legacyFlat := "GET_api_v1_does_not_exist.json"

	_, err := LoadResolved(baseDir, method, swaggerPath, legacyFlat, "", config.LayoutAuto)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}
