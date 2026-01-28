package utils

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetEnv_ReturnsValueWhenSet(t *testing.T) {
	t.Setenv("X_TEST_ENV", "hello")

	got := GetEnv("X_TEST_ENV", "default")
	if got != "hello" {
		t.Fatalf("got %q want %q", got, "hello")
	}
}

func TestGetEnv_ReturnsDefaultWhenMissing(t *testing.T) {
	_ = os.Unsetenv("X_TEST_ENV_MISSING")

	got := GetEnv("X_TEST_ENV_MISSING", "default")
	if got != "default" {
		t.Fatalf("got %q want %q", got, "default")
	}
}

func TestGetEnvAsBool_DefaultWhenMissing(t *testing.T) {
	_ = os.Unsetenv("X_BOOL")

	if got := GetEnvAsBool("X_BOOL", true); got != true {
		t.Fatalf("expected true when missing")
	}
	if got := GetEnvAsBool("X_BOOL", false); got != false {
		t.Fatalf("expected false when missing")
	}
}

func TestGetEnvAsBool_TruthyValues(t *testing.T) {
	cases := []string{"1", "true", "TRUE", "Yes", "yes", "TrUe"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			t.Setenv("X_BOOL", v)
			if got := GetEnvAsBool("X_BOOL", false); got != true {
				t.Fatalf("expected true for %q", v)
			}
		})
	}
}

func TestGetEnvAsBool_FalsyValues(t *testing.T) {
	cases := []string{"0", "false", "no", "NO", "random", "2", "t", "y"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			t.Setenv("X_BOOL", v)
			if got := GetEnvAsBool("X_BOOL", true); got != false {
				t.Fatalf("expected false for %q", v)
			}
		})
	}
}

func TestWriteJSON_WritesHeaderStatusAndBody(t *testing.T) {
	rr := httptest.NewRecorder()

	obj := map[string]any{"ok": true, "n": 1}
	WriteJSON(rr, 201, obj)

	if rr.Code != 201 {
		t.Fatalf("got status %d want %d", rr.Code, 201)
	}
	if ct := rr.Header().Get("content-type"); ct != "application/json" {
		t.Fatalf("got content-type %q want %q", ct, "application/json")
	}

	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%q", err, rr.Body.String())
	}
	if got["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", got)
	}

	if got["n"] != float64(1) {
		t.Fatalf("expected n=1, got %#v", got["n"])
	}
}
