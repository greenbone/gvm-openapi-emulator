// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"os"
	"testing"
)

func TestInitConfig_Defaults_AllFields(t *testing.T) {
	_ = os.Unsetenv("SERVER_PORT")
	_ = os.Unsetenv("SPEC_PATH")
	_ = os.Unsetenv("SAMPLES_DIR")
	_ = os.Unsetenv("LOG_LEVEL")
	_ = os.Unsetenv("RUNNING_ENV")
	_ = os.Unsetenv("VALIDATION_MODE")
	_ = os.Unsetenv("FALLBACK_MODE")
	_ = os.Unsetenv("DEBUG_ROUTES")
	_ = os.Unsetenv("LAYOUT_MODE")
	_ = os.Unsetenv("SCENARIO_ENABLED")
	_ = os.Unsetenv("SCENARIO_FILENAME")

	cfg := initConfig()

	if cfg.ServerPort != "8086" {
		t.Fatalf("ServerPort: expected %q, got %q", "8086", cfg.ServerPort)
	}
	if cfg.SpecPath != "/work/swagger.json" {
		t.Fatalf("SpecPath: expected %q, got %q", "/work/swagger.json", cfg.SpecPath)
	}
	if cfg.SamplesDir != "/work/sample" {
		t.Fatalf("SamplesDir: expected %q, got %q", "/work/sample", cfg.SamplesDir)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel: expected %q, got %q", "info", cfg.LogLevel)
	}
	if cfg.RunningEnv != EnvDocker {
		t.Fatalf("RunningEnv: expected %q, got %q", EnvDocker, cfg.RunningEnv)
	}
	if cfg.ValidationMode != ValidationRequired {
		t.Fatalf("ValidationMode: expected %q, got %q", ValidationRequired, cfg.ValidationMode)
	}
	if cfg.FallbackMode != FallbackOpenAPIExample {
		t.Fatalf("FallbackMode: expected %q, got %q", FallbackOpenAPIExample, cfg.FallbackMode)
	}
	if cfg.DebugRoutes != false {
		t.Fatalf("DebugRoutes: expected %v, got %v", false, cfg.DebugRoutes)
	}
	if cfg.Layout != LayoutAuto {
		t.Fatalf("Layout: expected %q, got %q", LayoutAuto, cfg.Layout)
	}

	if cfg.Scenario.Enabled != true {
		t.Fatalf("Scenario.Enabled: expected %v, got %v", true, cfg.Scenario.Enabled)
	}
	if cfg.Scenario.Filename != "scenario.json" {
		t.Fatalf("Scenario.Filename: expected %q, got %q", "scenario.json", cfg.Scenario.Filename)
	}
}

func TestInitConfig_Overrides_AllFields(t *testing.T) {
	t.Setenv("SERVER_PORT", "9999")
	t.Setenv("SPEC_PATH", "/tmp/spec.json")
	t.Setenv("SAMPLES_DIR", "/tmp/samples")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("RUNNING_ENV", "k8s")
	t.Setenv("VALIDATION_MODE", "none")
	t.Setenv("FALLBACK_MODE", "none")
	t.Setenv("DEBUG_ROUTES", "1")
	t.Setenv("LAYOUT_MODE", "folders")

	t.Setenv("SCENARIO_ENABLED", "false")
	t.Setenv("SCENARIO_FILENAME", "my-scenario.json")

	cfg := initConfig()

	if cfg.ServerPort != "9999" {
		t.Fatalf("ServerPort: expected %q, got %q", "9999", cfg.ServerPort)
	}
	if cfg.SpecPath != "/tmp/spec.json" {
		t.Fatalf("SpecPath: expected %q, got %q", "/tmp/spec.json", cfg.SpecPath)
	}
	if cfg.SamplesDir != "/tmp/samples" {
		t.Fatalf("SamplesDir: expected %q, got %q", "/tmp/samples", cfg.SamplesDir)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel: expected %q, got %q", "debug", cfg.LogLevel)
	}
	if cfg.RunningEnv != EnvK8s {
		t.Fatalf("RunningEnv: expected %q, got %q", EnvK8s, cfg.RunningEnv)
	}
	if cfg.ValidationMode != ValidationNone {
		t.Fatalf("ValidationMode: expected %q, got %q", ValidationNone, cfg.ValidationMode)
	}
	if cfg.FallbackMode != FallbackNone {
		t.Fatalf("FallbackMode: expected %q, got %q", FallbackNone, cfg.FallbackMode)
	}
	if cfg.DebugRoutes != true {
		t.Fatalf("DebugRoutes: expected %v, got %v", true, cfg.DebugRoutes)
	}
	if cfg.Layout != LayoutFolders {
		t.Fatalf("Layout: expected %q, got %q", LayoutFolders, cfg.Layout)
	}

	if cfg.Scenario.Enabled != false {
		t.Fatalf("Scenario.Enabled: expected %v, got %v", false, cfg.Scenario.Enabled)
	}
	if cfg.Scenario.Filename != "my-scenario.json" {
		t.Fatalf("Scenario.Filename: expected %q, got %q", "my-scenario.json", cfg.Scenario.Filename)
	}
}

func TestInitConfig_BoolParsing_DebugRoutesVariants(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"random", false},
	}

	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			t.Setenv("DEBUG_ROUTES", tc.val)
			cfg := initConfig()
			if cfg.DebugRoutes != tc.want {
				t.Fatalf("DEBUG_ROUTES=%q: expected %v, got %v", tc.val, tc.want, cfg.DebugRoutes)
			}
		})
	}
}

func TestInitConfig_BoolParsing_ScenarioEnabledVariants(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"random", false},
	}

	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			t.Setenv("SCENARIO_ENABLED", tc.val)
			cfg := initConfig()
			if cfg.Scenario.Enabled != tc.want {
				t.Fatalf("SCENARIO_ENABLED=%q: expected %v, got %v", tc.val, tc.want, cfg.Scenario.Enabled)
			}
		})
	}
}
