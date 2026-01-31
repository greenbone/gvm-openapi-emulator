package samples

import "github.com/ozgen/openapi-sample-emulator/config"

type Envelope struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
}

type Response struct {
	Status  int
	Headers map[string]string
	Body    []byte
}

type ProviderConfig struct {
	BaseDir          string
	Layout           config.LayoutMode
	ScenarioEnabled  bool
	ScenarioFilename string
	Engine           IScenarioResolver
}

type Scenario struct {
	Version int    `json:"version"`
	Mode    string `json:"mode"` // "step" | "time"

	Key struct {
		PathParam string `json:"pathParam"`
	} `json:"key"`

	// step mode
	Sequence []ScenarioEntry `json:"sequence,omitempty"`

	// time mode
	Timeline []TimelineEntry `json:"timeline,omitempty"`

	Behavior Behavior `json:"behavior"`
}

type ScenarioEntry struct {
	State string `json:"state"`
	File  string `json:"file"`
}

type TimelineEntry struct {
	AfterSec int64  `json:"afterSec"`
	State    string `json:"state"`
	File     string `json:"file"`
}

type Behavior struct {
	AdvanceOn  []MatchRule `json:"advanceOn,omitempty"`
	ResetOn    []MatchRule `json:"resetOn,omitempty"`
	StartOn    []MatchRule `json:"startOn,omitempty"`
	RepeatLast bool        `json:"repeatLast"`
	Loop       bool        `json:"loop,omitempty"`
}

type MatchRule struct {
	Method string `json:"method"`
	Path   string `json:"path,omitempty"`
}

type ResetRule struct {
	Method  string
	PathTpl string
}
