package server

import (
	"reflect"
	"testing"
)

func TestParseBodyStateRules(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty -> nil", "", nil},
		{"whitespace -> nil", "   \n\t ", nil},
		{"single token", "start", []string{"start"}},
		{"trims tokens", " start , stop ", []string{"start", "stop"}},
		{"drops empty parts", "start,, ,stop,  ,", []string{"start", "stop"}},
		{"keeps order", "a,b,c", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBodyStateRules(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseBodyStateRules(%q)\nwant: %#v\ngot:  %#v", tt.in, tt.want, got)
			}
		})
	}
}

func TestStateFromBodyContains(t *testing.T) {
	t.Run("no states -> false", func(t *testing.T) {
		if st, ok := StateFromBodyContains(`{"action":"start"}`, nil); ok || st != "" {
			t.Fatalf("expected ok=false, st='', got ok=%v st=%q", ok, st)
		}
	})

	t.Run("no match -> false", func(t *testing.T) {
		if st, ok := StateFromBodyContains(`{"action":"noop"}`, []string{"start", "stop"}); ok || st != "" {
			t.Fatalf("expected ok=false, st='', got ok=%v st=%q", ok, st)
		}
	})

	t.Run("match -> returns token", func(t *testing.T) {
		st, ok := StateFromBodyContains(`{"action":"start"}`, []string{"start", "stop"})
		if !ok {
			t.Fatalf("expected ok=true")
		}
		if st != "start" {
			t.Fatalf("expected %q got %q", "start", st)
		}
	})

	t.Run("first match wins", func(t *testing.T) {
		body := `{"action":"start_stop"}`
		st, ok := StateFromBodyContains(body, []string{"stop", "start"})
		if !ok {
			t.Fatalf("expected ok=true")
		}
		if st != "stop" {
			t.Fatalf("expected %q got %q", "stop", st)
		}
	})
}
