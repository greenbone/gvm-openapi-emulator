package samples

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScenarioPathForSwagger(t *testing.T) {
	base := "/tmp/base"
	got := ScenarioPathForSwagger(base, "/api/v1/items/{id}", "scenario.json")
	want := filepath.Join(base, filepath.FromSlash("api/v1/items/{id}"), "scenario.json")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMatchTemplatePath(t *testing.T) {
	cases := []struct {
		tpl   string
		act   string
		match bool
	}{
		{"/api/v1/items/{id}", "/api/v1/items/123", true},
		{"/api/v1/items/{id}/sub", "/api/v1/items/123/sub", true},
		{"/api/v1/items/{id}/sub", "/api/v1/items/123/other", false},
		{"/api/v1/items", "/api/v1/items/123", false}, // different segment count
		{"/api/v1/items/{id}", "/api/v1/things/123", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.tpl+"->"+tc.act, func(t *testing.T) {
			got := matchTemplatePath(tc.tpl, tc.act)
			if got != tc.match {
				t.Fatalf("expected %v, got %v", tc.match, got)
			}
		})
	}
}

func TestExtractPathParam(t *testing.T) {
	val, ok := extractPathParam("/api/v1/items/{id}", "/api/v1/items/777", "id")
	if !ok || val != "777" {
		t.Fatalf("expected ok=true val=777, got ok=%v val=%q", ok, val)
	}

	_, ok = extractPathParam("/api/v1/items/{id}", "/api/v1/items/777", "other")
	if ok {
		t.Fatalf("expected ok=false for missing param")
	}

	_, ok = extractPathParam("/api/v1/items/{id}", "/api/v1/items", "id")
	if ok {
		t.Fatalf("expected ok=false for different segment count")
	}
}

func TestLoadScenario_ValidV1_Step(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scenario.json")

	writeF(t, p, `{
	  "version": 1,
	  "mode": "step",
	  "key": {"pathParam":"id"},
	  "sequence": [{"state":"requested","file":"a.json"}],
	  "behavior": {"repeatLast": true}
	}`)

	sc, err := LoadScenario(p)
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	if sc.Version != 1 {
		t.Fatalf("expected version=1 got %d", sc.Version)
	}
	if sc.Mode != "step" {
		t.Fatalf("expected mode=step got %q", sc.Mode)
	}
	if sc.Key.PathParam != "id" {
		t.Fatalf("expected key.pathParam=id got %q", sc.Key.PathParam)
	}
}

func TestLoadScenario_ValidV1_Time(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scenario.json")

	writeF(t, p, `{
	  "version": 1,
	  "mode": "time",
	  "key": {"pathParam":"id"},
	  "timeline": [{"afterMs":0,"state":"ready","file":"t0.json"}],
	  "behavior": {"repeatLast": true}
	}`)

	sc, err := LoadScenario(p)
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	if sc.Mode != "time" {
		t.Fatalf("expected mode=time got %q", sc.Mode)
	}
}

func TestLoadScenario_InvalidVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scenario.json")

	writeF(t, p, `{"version":0,"mode":"step","key":{"pathParam":"id"},"behavior":{"repeatLast":false}}`)
	_, err := LoadScenario(p)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadScenario_InvalidMode(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scenario.json")

	writeF(t, p, `{"version":1,"mode":"wat","key":{"pathParam":"id"},"behavior":{"repeatLast":false}}`)
	_, err := LoadScenario(p)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadScenario_MissingKeyPathParam(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scenario.json")

	writeF(t, p, `{"version":1,"mode":"step","key":{"pathParam":"  "},"behavior":{"repeatLast":false}}`)
	_, err := LoadScenario(p)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestScenarioEngine_ResolveScenarioFile_Step_SelectsFirstThenAdvances(t *testing.T) {
	e := NewScenarioEngine()

	sc := &Scenario{
		Version: 1,
		Mode:    "step",
	}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "requested", File: "a.json"},
		{State: "running", File: "b.json"},
		{State: "done", File: "c.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.RepeatLast = true

	file1, state1, err := e.ResolveScenarioFile("sc.json", sc, "get", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file1 != "a.json" || state1 != "requested" {
		t.Fatalf("expected a.json/requested got %q/%q", file1, state1)
	}

	file2, state2, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file2 != "b.json" || state2 != "running" {
		t.Fatalf("expected b.json/running got %q/%q", file2, state2)
	}

	file3, state3, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file3 != "c.json" || state3 != "done" {
		t.Fatalf("expected c.json/done got %q/%q", file3, state3)
	}

	file4, state4, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file4 != "c.json" || state4 != "done" {
		t.Fatalf("expected c.json/done got %q/%q", file4, state4)
	}
}

func TestScenarioEngine_ResolveScenarioFile_Step_NoAdvanceRule_StaysOnFirst(t *testing.T) {
	e := NewScenarioEngine()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}

	sc.Behavior.AdvanceOn = nil
	sc.Behavior.RepeatLast = true

	file1, _, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/9")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	file2, _, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/9")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file1 != "a.json" || file2 != "a.json" {
		t.Fatalf("expected to stay on a.json, got %q then %q", file1, file2)
	}
}

func TestScenarioEngine_ResolveScenarioFile_Step_RequiresNonEmptySequence(t *testing.T) {
	e := NewScenarioEngine()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = nil

	_, _, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestScenarioEngine_ResolveScenarioFile_Time_ChoosesBasedOnElapsed(t *testing.T) {
	e := NewScenarioEngine()

	sc := &Scenario{Version: 1, Mode: "time"}
	sc.Key.PathParam = "id"
	sc.Timeline = []TimelineEntry{
		{AfterMs: 0, State: "t0", File: "t0.json"},
		{AfterMs: 20, State: "t20", File: "t20.json"},
	}
	sc.Behavior.RepeatLast = true

	file1, state1, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file1 != "t0.json" || state1 != "t0" {
		t.Fatalf("expected t0.json/t0 got %q/%q", file1, state1)
	}

	time.Sleep(30 * time.Millisecond)

	file2, state2, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file2 != "t20.json" || state2 != "t20" {
		t.Fatalf("expected t20.json/t20 got %q/%q", file2, state2)
	}
}

func TestScenarioEngine_ResolveScenarioFile_Time_RequiresNonEmptyTimeline(t *testing.T) {
	e := NewScenarioEngine()

	sc := &Scenario{Version: 1, Mode: "time"}
	sc.Key.PathParam = "id"
	sc.Timeline = nil

	_, _, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestScenarioEngine_ResolveScenarioFile_ResetOn_ClearsState(t *testing.T) {
	e := NewScenarioEngine()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.ResetOn = []MatchRule{{Method: "POST", Path: "/api/v1/items/{id}"}}
	sc.Behavior.RepeatLast = true

	_, _, _ = e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	file2, _, _ := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if file2 != "b.json" {
		t.Fatalf("expected b.json after advancing, got %q", file2)
	}

	_, _, err := e.ResolveScenarioFile("sc.json", sc, "POST", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile(POST reset): %v", err)
	}

	fileAfterReset, _, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile(after reset): %v", err)
	}
	if fileAfterReset != "a.json" {
		t.Fatalf("expected a.json after reset, got %q", fileAfterReset)
	}
}

func TestScenarioEngine_ResolveScenarioFile_KeyExtractionError(t *testing.T) {
	e := NewScenarioEngine()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{{State: "s1", File: "a.json"}}
	sc.Behavior.RepeatLast = true

	_, _, err := e.ResolveScenarioFile("sc.json", sc, "GET", "/api/v1/items/{id}", "/api/v1/items")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func writeF(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
