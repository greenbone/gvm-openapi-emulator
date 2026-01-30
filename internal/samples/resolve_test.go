package samples

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ozgen/openapi-sample-emulator/config"
)

type fakeScenarioEngine struct {
	file   string
	status int
	err    error
}

func (f *fakeScenarioEngine) ResolveScenarioFile(
	scenarioPath string,
	sc *Scenario,
	method string,
	swaggerTpl string,
	actualPath string,
) (file string, state string, err error) {
	return f.file, "", f.err
}

const validScenarioV1 = `{"version":1,"mode": "step", "key": { "pathParam": "id" }}`

func TestBuildCandidates(t *testing.T) {
	tests := []struct {
		name   string
		cfg    ResolverConfig
		method string
		path   string
		flat   string
		want   []string
	}{
		{
			name:   "auto adds folder then flat",
			cfg:    ResolverConfig{Layout: config.LayoutAuto},
			method: "GET",
			path:   "/api/v1/items",
			flat:   "GET_api_v1_items.json",
			want: []string{
				filepath.Join(filepath.FromSlash("api/v1/items"), "GET.json"),
				"GET_api_v1_items.json",
			},
		},
		{
			name:   "folders only no flat",
			cfg:    ResolverConfig{Layout: config.LayoutFolders},
			method: "POST",
			path:   "/api/v1/items",
			flat:   "POST_api_v1_items.json",
			want: []string{
				filepath.Join(filepath.FromSlash("api/v1/items"), "POST.json"),
			},
		},
		{
			name:   "flat only no folders",
			cfg:    ResolverConfig{Layout: config.LayoutFlat, State: "ignored"},
			method: "PUT",
			path:   "/api/v1/items/{id}",
			flat:   "PUT_api_v1_items_{id}.json",
			want: []string{
				"PUT_api_v1_items_{id}.json",
			},
		},
		{
			name:   "empty layout defaults to auto",
			cfg:    ResolverConfig{Layout: ""},
			method: "GET",
			path:   "/api/v1/items",
			flat:   "GET_api_v1_items.json",
			want: []string{
				filepath.Join(filepath.FromSlash("api/v1/items"), "GET.json"),
				"GET_api_v1_items.json",
			},
		},
		{
			name:   "auto with empty flat still includes empty string candidate (caller should guard) ",
			cfg:    ResolverConfig{Layout: config.LayoutAuto},
			method: "GET",
			path:   "/api/v1/items",
			flat:   "",
			want: []string{
				filepath.Join(filepath.FromSlash("api/v1/items"), "GET.json"),
				"",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := buildCandidates(tt.cfg, tt.method, tt.path, tt.flat)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildCandidates() mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}

func TestResolveSamplePath_Auto_PrefersFoldersOverFlat(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutAuto}

	writeFileWithDirs(t, baseDir, filepath.Join("api", "v1", "items", "GET.json"), "folder")
	writeFileWithDirs(t, baseDir, "GET_api_v1_items.json", "flat")

	got, err := ResolveSamplePath(
		cfg,
		"GET",
		"/api/v1/items",
		"/api/v1/items",
		"GET_api_v1_items.json",
		false,
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("ResolveSamplePath: %v", err)
	}

	want := filepath.Join(baseDir, "api", "v1", "items", "GET.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveSamplePath_FlatMode_FindsLegacy(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutFlat, State: "ignored"}

	writeFileWithDirs(t, baseDir, "GET_api_v1_items.json", "flat")

	got, err := ResolveSamplePath(
		cfg,
		"GET",
		"/api/v1/items",
		"/api/v1/items",
		"GET_api_v1_items.json",
		false,
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("ResolveSamplePath: %v", err)
	}

	want := filepath.Join(baseDir, "GET_api_v1_items.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveSamplePath_UppercasesMethod(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutFolders}

	writeFileWithDirs(t, baseDir, filepath.Join("api", "v1", "items", "GET.json"), "x")

	got, err := ResolveSamplePath(
		cfg,
		"gEt",
		"/api/v1/items",
		"/api/v1/items",
		"ignored.json",
		false,
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("ResolveSamplePath: %v", err)
	}

	want := filepath.Join(baseDir, "api", "v1", "items", "GET.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveSamplePath_NoSampleFound_ReturnsError(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutAuto}

	_, err := ResolveSamplePath(
		cfg,
		"GET",
		"/api/v1/items",
		"/api/v1/items",
		"GET_api_v1_items.json",
		false,
		"",
		nil,
	)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveSamplePath_NoCandidates_ReturnsError(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutFlat}

	_, err := ResolveSamplePath(
		cfg,
		"GET",
		"/api/v1/items",
		"/api/v1/items",
		"",
		false,
		"",
		nil,
	)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveSamplePath_ScenarioEnabled_EngineNil_ReturnsError_WhenScenarioFileExists(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutAuto}

	scenarioFilename := "scenario.json"
	swaggerTpl := "/api/v1/items"
	actualPath := "/api/v1/items"

	scPath := ScenarioPathForSwagger(baseDir, swaggerTpl, scenarioFilename)
	writeFileWithDirs(t, filepath.Dir(scPath), filepath.Base(scPath), `{}`)

	_, err := ResolveSamplePath(
		cfg,
		"GET",
		swaggerTpl,
		actualPath,
		"GET_api_v1_items.json",
		true,
		scenarioFilename,
		nil,
	)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveSamplePath_ScenarioEnabled_ScenarioFileMissing_FallsBackToNonScenario(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutFolders}

	writeFileWithDirs(t, baseDir, filepath.Join("api", "v1", "items", "GET.json"), "x")

	got, err := ResolveSamplePath(
		cfg,
		"GET",
		"/api/v1/items",
		"/api/v1/items",
		"GET_api_v1_items.json",
		true,
		"scenario.json",
		nil,
	)
	if err != nil {
		t.Fatalf("ResolveSamplePath: %v", err)
	}

	want := filepath.Join(baseDir, "api", "v1", "items", "GET.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveSamplePath_ScenarioEnabled_LoadScenarioError(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutAuto}

	swaggerTpl := "/api/v1/items"
	actualPath := "/api/v1/items"
	scenarioFilename := "scenario.json"

	scPath := ScenarioPathForSwagger(baseDir, swaggerTpl, scenarioFilename)
	writeFileWithDirs(t, filepath.Dir(scPath), filepath.Base(scPath), `this is not json`)

	_, err := ResolveSamplePath(
		cfg,
		"GET",
		swaggerTpl,
		actualPath,
		"GET_api_v1_items.json",
		true,
		scenarioFilename,
		nil,
	)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveSamplePath_ScenarioEnabled_ResolveError(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutAuto}

	swaggerTpl := "/api/v1/items"
	actualPath := "/api/v1/items"
	scenarioFilename := "scenario.json"

	scPath := ScenarioPathForSwagger(baseDir, swaggerTpl, scenarioFilename)
	writeFileWithDirs(t, filepath.Dir(scPath), filepath.Base(scPath), `{}`)

	engine := &fakeScenarioEngine{err: fmt.Errorf("boom")}

	_, err := ResolveSamplePath(
		cfg,
		"GET",
		swaggerTpl,
		actualPath,
		"GET_api_v1_items.json",
		true,
		scenarioFilename,
		engine,
	)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveSamplePath_ScenarioEnabled_FileReturnedButMissing_ReturnsError(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutAuto}

	swaggerTpl := "/api/v1/items"
	actualPath := "/api/v1/items"
	scenarioFilename := "scenario.json"

	scPath := ScenarioPathForSwagger(baseDir, swaggerTpl, scenarioFilename)
	writeFileWithDirs(t, filepath.Dir(scPath), filepath.Base(scPath), `{}`)

	engine := &fakeScenarioEngine{file: "GET.json"} // but we won't create GET.json

	_, err := ResolveSamplePath(
		cfg,
		"GET",
		swaggerTpl,
		actualPath,
		"GET_api_v1_items.json",
		true,
		scenarioFilename,
		engine,
	)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveSamplePath_ScenarioEnabled_FileReturnedAndExists_ReturnsScenarioFile(t *testing.T) {
	baseDir := t.TempDir()
	cfg := ResolverConfig{BaseDir: baseDir, Layout: config.LayoutAuto}

	swaggerTpl := "/api/v1/items"
	actualPath := "/api/v1/items"
	scenarioFilename := "scenario.json"

	scPath := ScenarioPathForSwagger(baseDir, swaggerTpl, scenarioFilename)
	writeFileWithDirs(t, filepath.Dir(scPath), filepath.Base(scPath), validScenarioV1)

	engine := &fakeScenarioEngine{file: "GET.scenario.json"}

	writeFileWithDirs(t, filepath.Dir(scPath), "GET.scenario.json", "ok")

	got, err := ResolveSamplePath(
		cfg,
		"GET",
		swaggerTpl,
		actualPath,
		"GET_api_v1_items.json",
		true,
		scenarioFilename,
		engine,
	)
	if err != nil {
		t.Fatalf("ResolveSamplePath: %v", err)
	}

	want := filepath.Join(filepath.Dir(scPath), "GET.scenario.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func writeFileWithDirs(t *testing.T, baseDir, relPath, content string) {
	t.Helper()
	full := filepath.Join(baseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}
