// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package samples

import (
	"os"
	"path/filepath"
	"strings"
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
	  "timeline": [{"afterSec":0,"state":"ready","file":"t0.json"}],
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

func TestScenarioResolver_ResolveScenarioFile_Step_SelectsFirstThenAdvances(t *testing.T) {
	e := NewScenarioResolver()

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

	file1, state1, err := e.ResolveScenarioFile(sc, "get", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file1 != "a.json" || state1 != "requested" {
		t.Fatalf("expected a.json/requested got %q/%q", file1, state1)
	}

	file2, state2, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file2 != "b.json" || state2 != "running" {
		t.Fatalf("expected b.json/running got %q/%q", file2, state2)
	}

	file3, state3, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file3 != "c.json" || state3 != "done" {
		t.Fatalf("expected c.json/done got %q/%q", file3, state3)
	}

	file4, state4, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file4 != "c.json" || state4 != "done" {
		t.Fatalf("expected c.json/done got %q/%q", file4, state4)
	}
}

func TestScenarioResolver_ResolveScenarioFile_Step_NoAdvanceRule_StaysOnFirst(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}

	sc.Behavior.AdvanceOn = nil
	sc.Behavior.RepeatLast = true

	file1, _, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/9")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	file2, _, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/9")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file1 != "a.json" || file2 != "a.json" {
		t.Fatalf("expected to stay on a.json, got %q then %q", file1, file2)
	}
}

func TestScenarioResolver_ResolveScenarioFile_Step_RequiresNonEmptySequence(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = nil

	_, _, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestScenarioResolver_ResolveScenarioFile_Time_ChoosesBasedOnElapsed(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "time"}
	sc.Key.PathParam = "id"
	sc.Timeline = []TimelineEntry{
		{AfterSec: 0, State: "t0", File: "t0.json"},
		{AfterSec: 1, State: "t20", File: "t20.json"},
	}
	sc.Behavior.RepeatLast = true

	file1, state1, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file1 != "t0.json" || state1 != "t0" {
		t.Fatalf("expected t0.json/t0 got %q/%q", file1, state1)
	}

	time.Sleep(1100 * time.Millisecond)

	file2, state2, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	if err != nil {
		t.Fatalf("ResolveScenarioFile: %v", err)
	}
	if file2 != "t20.json" || state2 != "t20" {
		t.Fatalf("expected t20.json/t20 got %q/%q", file2, state2)
	}
}

func TestScenarioResolver_ResolveScenarioFile_Time_RequiresNonEmptyTimeline(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "time"}
	sc.Key.PathParam = "id"
	sc.Timeline = nil

	_, _, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestScenarioResolver_ResetOn_ClearsState_ViaTryResetByRequest(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.ResetOn = []MatchRule{{Method: "POST", Path: "/api/v1/items/{id}"}}
	sc.Behavior.RepeatLast = true

	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	f2, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if f2 != "b.json" {
		t.Fatalf("expected b.json after advancing, got %q", f2)
	}

	reset := e.TryResetByRequest("POST", "/api/v1/items/1")
	if !reset {
		t.Fatalf("expected reset=true")
	}

	fAfter, _, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("ResolveScenarioFile(after reset): %v", err)
	}
	if fAfter != "a.json" {
		t.Fatalf("expected a.json after reset, got %q", fAfter)
	}
}

func TestScenarioResolver_ResolveScenarioFile_KeyExtractionError(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{{State: "s1", File: "a.json"}}
	sc.Behavior.RepeatLast = true

	_, _, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestScenarioResolver_ResolveScenarioFile_Step_Loop_WrapsToFirst(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.Loop = true
	sc.Behavior.RepeatLast = true

	f1, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	f2, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	f3, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")

	if f1 != "a.json" || f2 != "b.json" || f3 != "a.json" {
		t.Fatalf("expected a,b,a got %q,%q,%q", f1, f2, f3)
	}
}

func TestScenarioResolver_StateIsolation_ByID(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.RepeatLast = true

	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	f1b, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")

	f2a, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/2")

	if f1b != "b.json" {
		t.Fatalf("expected id=1 to be b.json, got %q", f1b)
	}
	if f2a != "a.json" {
		t.Fatalf("expected id=2 to be a.json, got %q", f2a)
	}
}

func TestScenarioResolver_Time_RepeatLast_SticksToLast(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "time"}
	sc.Key.PathParam = "id"
	sc.Timeline = []TimelineEntry{
		{AfterSec: 0, State: "t0", File: "t0.json"},
		{AfterSec: 1, State: "t1", File: "t1.json"},
	}
	sc.Behavior.RepeatLast = true
	sc.Behavior.Loop = false

	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	time.Sleep(1100 * time.Millisecond)

	f2, s2, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	if f2 != "t1.json" || s2 != "t1" {
		t.Fatalf("expected t1.json/t1 got %q/%q", f2, s2)
	}

	time.Sleep(1200 * time.Millisecond)
	f3, s3, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	if f3 != "t1.json" || s3 != "t1" {
		t.Fatalf("expected sticky t1.json/t1 got %q/%q", f3, s3)
	}
}

func TestLoadScenario_Time_RejectsUnsortedTimeline(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scenario.json")

	writeF(t, p, `{
	  "version": 1,
	  "mode": "time",
	  "key": {"pathParam":"id"},
	  "timeline": [
		{"afterSec": 2, "state":"t2", "file":"t2.json"},
		{"afterSec": 1, "state":"t1", "file":"t1.json"}
	  ],
	  "behavior": {"repeatLast": true}
	}`)

	_, err := LoadScenario(p)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadScenario_Step_RequiresNonEmptySequence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scenario.json")

	writeF(t, p, `{
	  "version": 1,
	  "mode": "step",
	  "key": {"pathParam":"id"},
	  "behavior": {"repeatLast": true}
	}`)

	_, err := LoadScenario(p)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "step mode requires non-empty sequence") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadScenario_Time_RequiresNonEmptyTimeline(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scenario.json")

	writeF(t, p, `{
	  "version": 1,
	  "mode": "time",
	  "key": {"pathParam":"id"},
	  "behavior": {"repeatLast": true}
	}`)

	_, err := LoadScenario(p)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "time mode requires non-empty timeline") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScenarioResolver_Step_WhenPastEnd_ClampsToLastEvenWithoutRepeatLast(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.RepeatLast = false
	sc.Behavior.Loop = false

	f1, _, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	f2, _, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	f3, _, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if f1 != "a.json" || f2 != "b.json" || f3 != "b.json" {
		t.Fatalf("expected a,b,b got %q,%q,%q", f1, f2, f3)
	}
}

func TestScenarioResolver_Step_AdvanceOn_MethodMatchIsCaseInsensitive(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "get"}} // lowercase
	sc.Behavior.RepeatLast = true

	f1, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	f2, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")

	if f1 != "a.json" || f2 != "b.json" {
		t.Fatalf("expected a then b, got %q then %q", f1, f2)
	}
}

func TestScenarioResolver_ResetOn_PathMismatch_DoesNotReset(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.ResetOn = []MatchRule{{Method: "POST", Path: "/api/v1/other/{id}"}}
	sc.Behavior.RepeatLast = true

	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	f2, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if f2 != "b.json" {
		t.Fatalf("expected b.json after advancing, got %q", f2)
	}

	_, _, err := e.ResolveScenarioFile(sc, "POST", "/api/v1/items/{id}", "/api/v1/items/1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	fAfter, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if fAfter != "b.json" {
		t.Fatalf("expected still b.json (no reset), got %q", fAfter)
	}
}

func TestScenarioResolver_Time_Loop_Wraps(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "time"}
	sc.Key.PathParam = "id"
	sc.Timeline = []TimelineEntry{
		{AfterSec: 0, State: "t0", File: "t0.json"},
		{AfterSec: 1, State: "t1", File: "t1.json"},
	}
	sc.Behavior.Loop = true
	sc.Behavior.RepeatLast = false

	f1, s1, err := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if f1 != "t0.json" || s1 != "t0" {
		t.Fatalf("expected t0 first, got %q/%q", f1, s1)
	}

	time.Sleep(1100 * time.Millisecond)
	f2, s2, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	if f2 != "t1.json" || s2 != "t1" {
		t.Fatalf("expected t1 after ~1s, got %q/%q", f2, s2)
	}

	time.Sleep(1200 * time.Millisecond)
	f3, s3, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/5")
	if f3 != "t0.json" || s3 != "t0" {
		t.Fatalf("expected wrap to t0, got %q/%q", f3, s3)
	}
}

func TestScenarioResolver_Time_StateIsolation_ByID(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "time"}
	sc.Key.PathParam = "id"
	sc.Timeline = []TimelineEntry{
		{AfterSec: 0, State: "t0", File: "t0.json"},
		{AfterSec: 1, State: "t1", File: "t1.json"},
	}
	sc.Behavior.RepeatLast = true

	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	time.Sleep(1100 * time.Millisecond)

	f1, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/1")
	if f1 != "t1.json" {
		t.Fatalf("expected id=1 to be t1.json, got %q", f1)
	}

	f2, _, _ := e.ResolveScenarioFile(sc, "GET", "/api/v1/items/{id}", "/api/v1/items/2")
	if f2 != "t0.json" {
		t.Fatalf("expected id=2 to start at t0.json, got %q", f2)
	}
}

func TestMatchTemplatePathSuffix(t *testing.T) {
	cases := []struct {
		tpl   string
		act   string
		match bool
	}{
		{"/scans/{id}", "/scans/123", true},
		{"/scans/{id}", "/api/v1/scans/123", true},
		{"/scans/{id}/status", "/scans/123/status", true},
		{"/scans/{id}/status", "/x/y/scans/123/status", true},
		{"/scans/{id}", "/scans", false},
		{"/scans/{id}", "/scans/123/status", false},
		{"/scans/{id}/status", "/scans/123/results", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.tpl+"->"+tc.act, func(t *testing.T) {
			got := matchTemplatePathSuffix(tc.tpl, tc.act)
			if got != tc.match {
				t.Fatalf("expected %v, got %v", tc.match, got)
			}
		})
	}
}

func TestScenarioResolver_RegistersResetRules_OnFirstResolve(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{{State: "s1", File: "a.json"}}
	sc.Behavior.ResetOn = []MatchRule{
		{Method: "DELETE", Path: "/scans/{id}"},
	}

	_, _, err := e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	eng := e.(*ScenarioResolver)
	eng.mu.Lock()
	defer eng.mu.Unlock()

	k := scenarioRuntimeKey("/scans/{id}", "1")
	rules := eng.resetRules[k]
	if len(rules) != 1 {
		t.Fatalf("expected 1 reset rule registered, got %d", len(rules))
	}
	if rules[0].Method != "DELETE" || rules[0].PathTpl != "/scans/{id}" {
		t.Fatalf("unexpected rule: %#v", rules[0])
	}
}

func TestScenarioResolver_TryResetByRequest_ResetsRunningScenario(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.ResetOn = []MatchRule{{Method: "DELETE", Path: "/scans/{id}"}}
	sc.Behavior.RepeatLast = true

	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1")
	f2, _, _ := e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1")
	if f2 != "b.json" {
		t.Fatalf("expected b.json after advancing, got %q", f2)
	}

	reset := e.TryResetByRequest("DELETE", "/scans/1")
	if !reset {
		t.Fatalf("expected reset=true")
	}

	fAfter, _, _ := e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1")
	if fAfter != "a.json" {
		t.Fatalf("expected a.json after reset, got %q", fAfter)
	}
}

func TestScenarioResolver_TryResetByRequest_WrongMethod_DoesNotReset(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.ResetOn = []MatchRule{{Method: "DELETE", Path: "/scans/{id}"}}
	sc.Behavior.RepeatLast = true

	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1")
	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1") // now at b

	reset := e.TryResetByRequest("POST", "/scans/1")
	if reset {
		t.Fatalf("expected reset=false")
	}

	fAfter, _, _ := e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1")
	if fAfter != "b.json" {
		t.Fatalf("expected still b.json (no reset), got %q", fAfter)
	}
}

func TestScenarioResolver_TryResetByRequest_PathMismatch_DoesNotReset(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}
	sc.Behavior.ResetOn = []MatchRule{{Method: "DELETE", Path: "/scans/{id}"}}
	sc.Behavior.RepeatLast = true

	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1")
	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1") // now at b

	reset := e.TryResetByRequest("DELETE", "/other/1")
	if reset {
		t.Fatalf("expected reset=false")
	}

	fAfter, _, _ := e.ResolveScenarioFile(sc, "GET", "/scans/{id}", "/scans/1")
	if fAfter != "b.json" {
		t.Fatalf("expected still b.json (no reset), got %q", fAfter)
	}
}

func TestScenarioResolver_TryResetByRequest_ResetsScenarioRegisteredFromDifferentTpl(t *testing.T) {
	e := NewScenarioResolver()

	sc := &Scenario{Version: 1, Mode: "step"}
	sc.Key.PathParam = "id"
	sc.Sequence = []ScenarioEntry{
		{State: "s1", File: "a.json"},
		{State: "s2", File: "b.json"},
	}
	sc.Behavior.AdvanceOn = []MatchRule{{Method: "GET"}}

	sc.Behavior.ResetOn = []MatchRule{{Method: "DELETE", Path: "/scans/{id}"}}
	sc.Behavior.RepeatLast = true

	_, _, _ = e.ResolveScenarioFile(sc, "GET", "/scans/{id}/status", "/scans/1/status")
	f2, _, _ := e.ResolveScenarioFile(sc, "GET", "/scans/{id}/status", "/scans/1/status")
	if f2 != "b.json" {
		t.Fatalf("expected b.json after advancing, got %q", f2)
	}

	reset := e.TryResetByRequest("DELETE", "/scans/1")
	if !reset {
		t.Fatalf("expected reset=true")
	}

	fAfter, _, _ := e.ResolveScenarioFile(sc, "GET", "/scans/{id}/status", "/scans/1/status")
	if fAfter != "a.json" {
		t.Fatalf("expected a.json after reset, got %q", fAfter)
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
