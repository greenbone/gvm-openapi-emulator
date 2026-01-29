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
	_ = os.Unsetenv("STATE_FLOW")
	_ = os.Unsetenv("STATE_STEP_SECONDS")
	_ = os.Unsetenv("STATE_STEP_CALLS")
	_ = os.Unsetenv("STATE_ID_PARAM")
	_ = os.Unsetenv("STATE_RESET_ON_LAST")
	_ = os.Unsetenv("BODY_STATES")

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

	if cfg.StateFlow != "" {
		t.Fatalf("StateFlow: expected %q, got %q", "", cfg.StateFlow)
	}
	if cfg.StateStepSeconds != 0 {
		t.Fatalf("StateStepSeconds: expected %d, got %d", 0, cfg.StateStepSeconds)
	}
	if cfg.StateStepCalls != 1 {
		t.Fatalf("StateStepCalls: expected %d, got %d", 1, cfg.StateStepCalls)
	}
	if cfg.StateResetOnLast != false {
		t.Fatalf("StateResetOnLast: expected %v, got %v", false, cfg.StateResetOnLast)
	}
	if cfg.StateIDParam != "id" {
		t.Fatalf("StateIDParam: expected %q, got %q", "id", cfg.StateIDParam)
	}
	if cfg.BodyStates != "start,stop" {
		t.Fatalf("BodyStates: expected %q, got %q", "start,stop", cfg.BodyStates)
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

	t.Setenv("STATE_FLOW", "requested,running*9,succeeded")
	t.Setenv("STATE_STEP_SECONDS", "5")
	t.Setenv("STATE_STEP_CALLS", "3")
	t.Setenv("STATE_ID_PARAM", "scan_id")
	t.Setenv("STATE_RESET_ON_LAST", "true")
	t.Setenv("BODY_STATES", "created,started,finished")

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

	// State machine overrides
	if cfg.StateFlow != "requested,running*9,succeeded" {
		t.Fatalf("StateFlow: expected %q, got %q", "requested,running*9,succeeded", cfg.StateFlow)
	}
	if cfg.StateStepSeconds != 5 {
		t.Fatalf("StateStepSeconds: expected %d, got %d", 5, cfg.StateStepSeconds)
	}
	if cfg.StateStepCalls != 3 {
		t.Fatalf("StateStepCalls: expected %d, got %d", 3, cfg.StateStepCalls)
	}
	if cfg.StateIDParam != "scan_id" {
		t.Fatalf("StateIDParam: expected %q, got %q", "scan_id", cfg.StateIDParam)
	}
	if cfg.StateResetOnLast != true {
		t.Fatalf("StateResetOnLast: expected %v, got %v", true, cfg.StateResetOnLast)
	}
	if cfg.BodyStates != "created,started,finished" {
		t.Fatalf("BodyStates: expected %q, got %q", "created,started,finished", cfg.BodyStates)
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

func TestInitConfig_BoolParsing_StateResetOnLastVariants(t *testing.T) {
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
			t.Setenv("STATE_RESET_ON_LAST", tc.val)
			cfg := initConfig()
			if cfg.StateResetOnLast != tc.want {
				t.Fatalf("STATE_RESET_ON_LAST=%q: expected %v, got %v", tc.val, tc.want, cfg.StateResetOnLast)
			}
		})
	}
}
