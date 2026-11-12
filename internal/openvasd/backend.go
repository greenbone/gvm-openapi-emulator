// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package openvasd implements the deterministic, stateful OpenVASD test
// profile. It intentionally does not fall back to generic OpenAPI examples:
// every accepted request is handled here and every other request fails closed.
package openvasd

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/greenbone/gvm-openapi-emulator/internal/samples"
)

const (
	apiVersion      = "1"
	feedVersion     = "202611110000"
	authentication  = "disabled"
	openAPIRevision = "6b0d7ec7e562aff805c8b2f968a8b0e18b5f63b2"
	fixtureRevision = "openvasd-issue-45-v2"

	scenarioHeader   = "X-Emulator-Scenario"
	resetTokenHeader = "X-Emulator-Reset-Token" // #nosec G101 -- HTTP header name, not a credential.
	maxRequestBody   = 1 << 20
)

var knownScenarios = []string{
	"success",
	"failed-scan",
	"http-400",
	"http-500",
	"malformed-body",
	"malformed-header",
	"timeout",
}

type Config struct {
	ResetToken     string
	TimeoutDelay   time.Duration
	SampleProvider samples.ISampleProvider
}

type Backend struct {
	mu sync.Mutex

	resetToken        string
	sampleProvider    samples.ISampleProvider
	timeoutDelay      time.Duration
	nextScanID        uint64
	notReadyResponses int
	scans             map[string]*scan
}

type scan struct {
	request   scanRequest
	status    string
	scenario  string
	startTime int64
	endTime   int64
}

type scanRequest struct {
	ScanID          string              `json:"scan_id,omitempty"`
	Target          target              `json:"target"`
	ScanPreferences []scannerPreference `json:"scan_preferences,omitempty"`
	VTs             []vt                `json:"vts"`
}

type target struct {
	Hosts              []string          `json:"hosts"`
	ExcludedHosts      []string          `json:"excluded_hosts,omitempty"`
	Ports              []portRange       `json:"ports"`
	Credentials        []json.RawMessage `json:"credentials,omitempty"`
	AliveTestPorts     []portRange       `json:"alive_test_ports,omitempty"`
	AliveTestMethods   []string          `json:"alive_test_methods,omitempty"`
	ReverseLookupUnify bool              `json:"reverse_lookup_unify,omitempty"`
	ReverseLookupOnly  bool              `json:"reverse_lookup_only,omitempty"`
}

type portRange struct {
	Protocol string       `json:"protocol,omitempty"`
	Range    []portBounds `json:"range"`
}

type portBounds struct {
	Start int  `json:"start"`
	End   *int `json:"end,omitempty"`
}

type scannerPreference struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type vt struct {
	OID        string        `json:"oid"`
	Parameters []vtParameter `json:"parameters,omitempty"`
}

type vtParameter struct {
	ID    int32  `json:"id"`
	Value string `json:"value"`
}

type scanResponse struct {
	ScanID          string              `json:"scan_id"`
	Target          target              `json:"target"`
	ScanPreferences []scannerPreference `json:"scan_preferences,omitempty"`
	VTs             []vt                `json:"vts"`
}

type scanStatus struct {
	StartTime int64    `json:"start_time,omitempty"`
	EndTime   int64    `json:"end_time,omitempty"`
	Status    string   `json:"status"`
	HostInfo  hostInfo `json:"host_info"`
}

type hostInfo struct {
	All      int      `json:"all"`
	Excluded int      `json:"excluded"`
	Dead     int      `json:"dead"`
	Alive    int      `json:"alive"`
	Queued   int      `json:"queued"`
	Finished int      `json:"finished"`
	Scanning []string `json:"scanning,omitempty"`
}

type result struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	IPAddress string `json:"ip_address"`
	Hostname  string `json:"hostname,omitempty"`
	OID       string `json:"oid"`
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Message   string `json:"message"`
}

type actionRequest struct {
	Action string `json:"action"`
}

type resetRequest struct {
	NotReadyResponses *int `json:"not_ready_responses,omitempty"`
}

func New(cfg Config) *Backend {
	if cfg.TimeoutDelay <= 0 {
		cfg.TimeoutDelay = 11 * time.Second
	}
	return &Backend{
		resetToken:        cfg.ResetToken,
		sampleProvider:    cfg.SampleProvider,
		timeoutDelay:      cfg.TimeoutDelay,
		notReadyResponses: 1,
		scans:             make(map[string]*scan),
	}
}

func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setMetadataHeaders(w)

	if r.URL.Path == "/_emulator/reset" {
		b.handleReset(w, r)
		return
	}

	scenario := strings.TrimSpace(r.Header.Get(scenarioHeader))
	if scenario == "" {
		scenario = "success"
	}
	if !slices.Contains(knownScenarios, scenario) {
		writeError(w, http.StatusBadRequest, "unknown scenario", map[string]any{
			"scenario": scenario,
			"known":    knownScenarios,
		})
		return
	}
	if scenario == "failed-scan" && (r.Method != http.MethodPost || r.URL.Path != "/scans") {
		writeError(w, http.StatusBadRequest, "failed-scan is only valid when creating a scan", nil)
		return
	}
	if scenario != "success" && scenario != "failed-scan" {
		if injectableRequest(r) {
			b.writeFailureScenario(w, scenario)
			return
		}
		// Invalid routes, methods, and query parameters retain their normal
		// fail-closed response instead of being masked by fault injection.
		scenario = "success"
	}

	switch r.URL.Path {
	case "/health":
		b.handleHeadOnly(w, r)
		return
	case "/health/alive":
		b.handleHealth(w, r, http.StatusOK)
		return
	case "/health/ready":
		b.handleReady(w, r)
		return
	case "/health/started":
		b.handleHealth(w, r, http.StatusOK)
		return
	case "/vts":
		b.handleVTs(w, r)
		return
	case "/scans":
		b.handleScans(w, r, scenario)
		return
	case "/scans/preferences":
		b.handlePreferences(w, r)
		return
	}

	parts := splitPath(r.URL.Path)
	if len(parts) >= 2 && parts[0] == "scans" && parts[1] != "" {
		switch {
		case len(parts) == 2:
			b.handleScan(w, r, parts[1])
			return
		case len(parts) == 3 && parts[2] == "status":
			b.handleStatus(w, r, parts[1])
			return
		case len(parts) == 3 && parts[2] == "results":
			b.handleResults(w, r, parts[1])
			return
		case len(parts) == 4 && parts[2] == "results":
			b.handleResult(w, r, parts[1], parts[3])
			return
		}
	}

	writeError(w, http.StatusNotFound, "no route", map[string]any{
		"method": r.Method,
		"path":   r.URL.Path,
	})
}

func (b *Backend) writeFailureScenario(w http.ResponseWriter, scenario string) {
	switch scenario {
	case "http-400":
		writeError(w, http.StatusBadRequest, "injected client failure", nil)
	case "http-500":
		writeError(w, http.StatusInternalServerError, "injected server failure", nil)
	case "malformed-body":
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"malformed":`))
	case "malformed-header":
		w.Header().Set("api-version", "malformed")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "timeout":
		time.Sleep(b.timeoutDelay)
		writeJSON(w, http.StatusOK, map[string]any{"scenario": "timeout"})
	}
}

func (b *Backend) handleHeadOnly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodHead {
		methodNotAllowed(w, http.MethodHead)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Backend) handleHealth(w http.ResponseWriter, r *http.Request, status int) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	w.WriteHeader(status)
}

func (b *Backend) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}

	b.mu.Lock()
	status := http.StatusOK
	if b.notReadyResponses > 0 {
		b.notReadyResponses--
		status = http.StatusServiceUnavailable
	}
	b.mu.Unlock()
	w.WriteHeader(status)
}

func (b *Backend) handleVTs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		if !validQuery(r, nil) {
			writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet, http.MethodHead)
		return
	}
	if !validQuery(r, map[string]func(string) bool{
		"information": func(value string) bool { return value == "1" || value == "true" },
	}) {
		writeError(w, http.StatusBadRequest, "unexpected or invalid query parameter", nil)
		return
	}

	if b.sampleProvider == nil {
		writeError(w, http.StatusInternalServerError, "VT sample provider is not configured", nil)
		return
	}
	sample, err := b.sampleProvider.ResolveAndLoad(http.MethodGet, "/vts", "/vts", "GET__vts.json")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load VT sample", nil)
		return
	}
	var information []vt
	if err := json.Unmarshal(sample.Body, &information); err != nil || len(information) == 0 {
		writeError(w, http.StatusInternalServerError, "invalid VT sample", nil)
		return
	}
	oids := make([]string, 0, len(information))
	for _, item := range information {
		if item.OID == "" || slices.Contains(oids, item.OID) {
			writeError(w, http.StatusInternalServerError, "invalid VT sample OID", nil)
			return
		}
		oids = append(oids, item.OID)
	}
	if r.URL.Query().Has("information") {
		writeJSON(w, http.StatusOK, json.RawMessage(sample.Body))
		return
	}
	writeJSON(w, http.StatusOK, oids)
}

func (b *Backend) handleScans(w http.ResponseWriter, r *http.Request, scenario string) {
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}

	switch r.Method {
	case http.MethodHead:
		w.WriteHeader(http.StatusOK)
	case http.MethodPost:
		var request scanRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid scan request", map[string]any{"details": err.Error()})
			return
		}
		if err := validateScanRequest(request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid scan request", map[string]any{"details": err.Error()})
			return
		}

		b.mu.Lock()
		id := request.ScanID
		if id == "" {
			b.nextScanID++
			id = fmt.Sprintf("00000000-0000-4000-8000-%012d", b.nextScanID)
		}
		if _, exists := b.scans[id]; exists {
			b.mu.Unlock()
			writeError(w, http.StatusConflict, "scan id already in use", map[string]any{"scan_id": id})
			return
		}
		request.ScanID = id
		b.scans[id] = &scan{request: request, status: "stored", scenario: scenario}
		b.mu.Unlock()
		writeJSON(w, http.StatusCreated, id)
	default:
		methodNotAllowed(w, http.MethodHead, http.MethodPost)
	}
}

func (b *Backend) handlePreferences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	writeJSON(w, http.StatusOK, []map[string]string{
		{
			"id":          "safe_checks",
			"type":        "boolean",
			"name":        "Safe checks",
			"description": "Run only safe vulnerability tests",
			"default":     "yes",
			"values":      "yes,no",
		},
	})
}

func (b *Backend) handleScan(w http.ResponseWriter, r *http.Request, id string) {
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		b.mu.Lock()
		stored, exists := b.scans[id]
		if exists {
			response := scanResponse{
				ScanID:          id,
				Target:          stored.request.Target,
				ScanPreferences: stored.request.ScanPreferences,
				VTs:             stored.request.VTs,
			}
			b.mu.Unlock()
			writeJSON(w, http.StatusOK, response)
			return
		}
		b.mu.Unlock()
		writeError(w, http.StatusNotFound, "scan not found", map[string]any{"scan_id": id})
	case http.MethodPost:
		var action actionRequest
		if err := decodeJSON(r, &action); err != nil {
			writeError(w, http.StatusBadRequest, "invalid scan action", map[string]any{"details": err.Error()})
			return
		}
		if action.Action != "start" && action.Action != "stop" {
			writeError(w, http.StatusBadRequest, "invalid scan action", map[string]any{
				"action": action.Action,
			})
			return
		}
		b.applyAction(w, id, action.Action)
	case http.MethodDelete:
		b.deleteScan(w, id)
	default:
		methodNotAllowed(w, http.MethodDelete, http.MethodGet, http.MethodPost)
	}
}

func (b *Backend) applyAction(w http.ResponseWriter, id, action string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	stored, exists := b.scans[id]
	if !exists {
		writeError(w, http.StatusNotFound, "scan not found", map[string]any{"scan_id": id})
		return
	}

	switch action {
	case "start":
		switch stored.status {
		case "stored":
			stored.status = "requested"
			stored.startTime = 1_700_000_000
		case "requested", "running":
			// Starting an already scheduled scan is idempotent.
		default:
			writeError(w, http.StatusConflict, "scan cannot be started in current state", map[string]any{
				"status": stored.status,
			})
			return
		}
	case "stop":
		switch stored.status {
		case "requested", "running":
			stored.status = "stopped"
			stored.endTime = stored.startTime + 2
		case "stopped":
			// Stopping a stopped scan is idempotent.
		default:
			writeError(w, http.StatusConflict, "scan cannot be stopped in current state", map[string]any{
				"status": stored.status,
			})
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (b *Backend) deleteScan(w http.ResponseWriter, id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	stored, exists := b.scans[id]
	if !exists {
		writeError(w, http.StatusNotFound, "scan not found", map[string]any{"scan_id": id})
		return
	}
	if stored.status == "requested" || stored.status == "running" {
		writeError(w, http.StatusConflict, "running scan cannot be deleted", map[string]any{
			"status": stored.status,
		})
		return
	}
	delete(b.scans, id)
	w.WriteHeader(http.StatusNoContent)
}

func (b *Backend) handleStatus(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}

	b.mu.Lock()
	stored, exists := b.scans[id]
	if !exists {
		b.mu.Unlock()
		writeError(w, http.StatusNotFound, "scan not found", map[string]any{"scan_id": id})
		return
	}
	response := b.statusResponse(stored)
	switch stored.status {
	case "requested":
		stored.status = "running"
	case "running":
		if stored.scenario == "failed-scan" {
			stored.status = "failed"
		} else {
			stored.status = "succeeded"
		}
		stored.endTime = stored.startTime + 2
	}
	b.mu.Unlock()
	writeJSON(w, http.StatusOK, response)
}

func (b *Backend) statusResponse(stored *scan) scanStatus {
	hosts := len(stored.request.Target.Hosts)
	info := hostInfo{All: hosts, Excluded: len(stored.request.Target.ExcludedHosts)}
	switch stored.status {
	case "stored":
		info.Queued = hosts
	case "requested":
		info.Queued = hosts
	case "running":
		info.Alive = hosts
		info.Scanning = append([]string(nil), stored.request.Target.Hosts...)
	case "succeeded":
		info.Alive = hosts
		info.Finished = hosts
	case "failed", "stopped":
		info.Finished = hosts
	}
	return scanStatus{
		StartTime: stored.startTime,
		EndTime:   stored.endTime,
		Status:    stored.status,
		HostInfo:  info,
	}
}

func (b *Backend) handleResults(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, map[string]func(string) bool{"range": validResultRange}) {
		writeError(w, http.StatusBadRequest, "unexpected or invalid query parameter", nil)
		return
	}

	results, exists := b.scanResults(id)
	if !exists {
		writeError(w, http.StatusNotFound, "scan not found", map[string]any{"scan_id": id})
		return
	}
	if value := r.URL.Query().Get("range"); value != "" {
		results = applyResultRange(results, value)
	}
	writeJSON(w, http.StatusOK, results)
}

func (b *Backend) handleResult(w http.ResponseWriter, r *http.Request, id, rawResultID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	resultID, err := strconv.Atoi(rawResultID)
	if err != nil || resultID < 0 {
		writeError(w, http.StatusBadRequest, "invalid result id", map[string]any{
			"result_id": rawResultID,
		})
		return
	}
	results, exists := b.scanResults(id)
	if !exists {
		writeError(w, http.StatusNotFound, "scan not found", map[string]any{"scan_id": id})
		return
	}
	if resultID >= len(results) {
		writeError(w, http.StatusNotFound, "result not found", map[string]any{"result_id": resultID})
		return
	}
	writeJSON(w, http.StatusOK, results[resultID])
}

func (b *Backend) scanResults(id string) ([]result, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	stored, exists := b.scans[id]
	if !exists {
		return nil, false
	}
	if stored.status == "stored" || stored.status == "requested" {
		return []result{}, true
	}

	port := 0
	protocol := "tcp"
	if len(stored.request.Target.Ports) > 0 {
		ports := stored.request.Target.Ports[0]
		if ports.Protocol != "" {
			protocol = ports.Protocol
		}
		if len(ports.Range) > 0 {
			port = ports.Range[0].Start
		}
	}
	preferences, err := json.Marshal(stored.request.ScanPreferences)
	if err != nil {
		return nil, false
	}
	results := make([]result, 0, len(stored.request.Target.Hosts)*len(stored.request.VTs))
	for _, host := range stored.request.Target.Hosts {
		for _, item := range stored.request.VTs {
			resultID := len(results)
			message := fmt.Sprintf(
				"deterministic %s result for scan %s, target %s, VT %s, preferences %s",
				stored.scenario, id, host, item.OID, preferences,
			)
			results = append(results, result{
				ID:        resultID,
				Type:      "alarm",
				IPAddress: host,
				OID:       item.OID,
				Port:      port,
				Protocol:  protocol,
				Message:   message,
			})
		}
	}
	return results, true
}

func (b *Backend) handleReset(w http.ResponseWriter, r *http.Request) {
	if b.resetToken == "" {
		writeError(w, http.StatusNotFound, "no route", map[string]any{"path": r.URL.Path})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	provided := r.Header.Get(resetTokenHeader)
	if subtle.ConstantTimeCompare([]byte(provided), []byte(b.resetToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid reset token", nil)
		return
	}

	notReadyResponses := 1
	if r.Body != nil && r.ContentLength != 0 {
		var request resetRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid reset request", map[string]any{"details": err.Error()})
			return
		}
		if request.NotReadyResponses != nil {
			if *request.NotReadyResponses < 0 || *request.NotReadyResponses > 100 {
				writeError(w, http.StatusBadRequest, "not_ready_responses must be between 0 and 100", nil)
				return
			}
			notReadyResponses = *request.NotReadyResponses
		}
	}

	b.mu.Lock()
	b.nextScanID = 0
	b.notReadyResponses = notReadyResponses
	b.scans = make(map[string]*scan)
	b.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func validateScanRequest(request scanRequest) error {
	if len(request.Target.Hosts) == 0 {
		return fmt.Errorf("target.hosts must contain at least one host")
	}
	for _, host := range request.Target.Hosts {
		if strings.TrimSpace(host) == "" {
			return fmt.Errorf("target.hosts must not contain an empty host")
		}
	}
	if len(request.Target.Ports) == 0 {
		return fmt.Errorf("target.ports must contain at least one port range")
	}
	for _, ports := range request.Target.Ports {
		if ports.Protocol != "" && ports.Protocol != "tcp" && ports.Protocol != "udp" {
			return fmt.Errorf("target.ports protocol %q is invalid", ports.Protocol)
		}
		if len(ports.Range) == 0 {
			return fmt.Errorf("target.ports range must not be empty")
		}
		for _, bounds := range ports.Range {
			if bounds.Start < 0 || bounds.Start > 65535 {
				return fmt.Errorf("target.ports start must be between 0 and 65535")
			}
			if bounds.End != nil && (*bounds.End < bounds.Start || *bounds.End > 65535) {
				return fmt.Errorf("target.ports end must be between start and 65535")
			}
		}
	}
	if len(request.VTs) == 0 {
		return fmt.Errorf("vts must contain at least one VT")
	}
	for _, item := range request.VTs {
		if strings.TrimSpace(item.OID) == "" {
			return fmt.Errorf("vts must not contain an empty oid")
		}
	}
	return nil
}

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain exactly one JSON value")
		}
		return err
	}
	return nil
}

func validQuery(r *http.Request, allowed map[string]func(string) bool) bool {
	query := r.URL.Query()
	if len(query) != len(allowed) && len(query) > 0 {
		// The exact-key loop below handles optional allowed keys; this fast path
		// only rejects queries when no keys are allowed.
		if len(allowed) == 0 {
			return false
		}
	}
	for key, values := range query {
		validator, ok := allowed[key]
		if !ok || len(values) != 1 || !validator(values[0]) {
			return false
		}
	}
	return true
}

func injectableRequest(r *http.Request) bool {
	switch r.URL.Path {
	case "/health":
		return r.Method == http.MethodHead && validQuery(r, nil)
	case "/health/alive", "/health/ready", "/health/started":
		return r.Method == http.MethodGet && validQuery(r, nil)
	case "/vts":
		if r.Method == http.MethodHead {
			return validQuery(r, nil)
		}
		return r.Method == http.MethodGet && validQuery(r, map[string]func(string) bool{
			"information": func(value string) bool {
				return value == "1" || value == "true"
			},
		})
	case "/scans":
		return (r.Method == http.MethodHead || r.Method == http.MethodPost) && validQuery(r, nil)
	case "/scans/preferences":
		return r.Method == http.MethodGet && validQuery(r, nil)
	}

	parts := splitPath(r.URL.Path)
	switch {
	case len(parts) == 2 && parts[0] == "scans":
		return (r.Method == http.MethodGet || r.Method == http.MethodPost ||
			r.Method == http.MethodDelete) && validQuery(r, nil)
	case len(parts) == 3 && parts[0] == "scans" && parts[2] == "status":
		return r.Method == http.MethodGet && validQuery(r, nil)
	case len(parts) == 3 && parts[0] == "scans" && parts[2] == "results":
		return r.Method == http.MethodGet && validQuery(r, map[string]func(string) bool{
			"range": validResultRange,
		})
	case len(parts) == 4 && parts[0] == "scans" && parts[2] == "results":
		return r.Method == http.MethodGet && validQuery(r, nil)
	default:
		return false
	}
}

func validResultRange(value string) bool {
	if value == "" {
		return false
	}
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return false
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil || start < 0 {
		return false
	}
	if len(parts) == 2 {
		end, err := strconv.Atoi(parts[1])
		return err == nil && end >= start
	}
	return true
}

func applyResultRange(results []result, value string) []result {
	parts := strings.Split(value, "-")
	start, _ := strconv.Atoi(parts[0])
	if start >= len(results) {
		return []result{}
	}
	end := len(results) - 1
	if len(parts) == 2 {
		end, _ = strconv.Atoi(parts[1])
		if end >= len(results) {
			end = len(results) - 1
		}
	}
	return results[start : end+1]
}

func splitPath(path string) []string {
	if path == "" || path == "/" || strings.HasSuffix(path, "/") {
		return nil
	}
	return strings.Split(strings.TrimPrefix(path, "/"), "/")
}

func setMetadataHeaders(w http.ResponseWriter) {
	w.Header().Set("api-version", apiVersion)
	w.Header().Set("feed-version", feedVersion)
	w.Header().Set("authentication", authentication)
	w.Header().Set("x-emulator-openapi-revision", openAPIRevision)
	w.Header().Set("x-emulator-fixture-revision", fixtureRevision)
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("allow", strings.Join(methods, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method not allowed", map[string]any{"allowed": methods})
}

func writeError(w http.ResponseWriter, status int, message string, extra map[string]any) {
	body := map[string]any{"error": message, "status": status}
	for key, value := range extra {
		body[key] = value
	}
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
