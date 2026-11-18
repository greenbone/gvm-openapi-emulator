// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package agentcontrol implements the deterministic, stateful Agent Control
// profile. The profile uses the fixtures added by PR #44 as its seed and never
// delegates to the generic success fallback.
package agentcontrol

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/greenbone/gvm-openapi-emulator/internal/openapi"
	"github.com/greenbone/gvm-openapi-emulator/internal/samples"
)

const (
	apiVersion        = "2"
	openAPIRevision   = "b3367ab0648cba4e3c6c709f796cf694cd5ac1b5"
	fixtureRevision   = "agent-control-issue-49-v1"
	baselineRevision  = "9709bd5153f466fbd181e98b8bd96af1655848dd"
	scenarioHeader    = "X-Emulator-Scenario"
	resetTokenHeader  = "X-Emulator-Reset-Token" // #nosec G101 -- header name, not a credential.
	maxRequestBody    = 1 << 20
	defaultPageSize   = 100
	maxPageSize       = 1000
	supportBundleDays = 7
)

var knownScenarios = []string{
	"success",
	"authentication",
	"unavailable",
	"malformed",
	"conflict",
	"rate-limit",
	"timeout",
}

type Config struct {
	ResetToken     string
	TimeoutDelay   time.Duration
	SampleProvider samples.ISampleProvider
	RouterProvider openapi.IRouterProvider
}

type Backend struct {
	mu sync.Mutex

	resetToken     string
	timeoutDelay   time.Duration
	sampleProvider samples.ISampleProvider
	routerProvider openapi.IRouterProvider

	agents       map[string]map[string]any
	agentOrder   []string
	scanConfig   map[string]any
	scans        map[string]*scan
	scanOrder    []string
	nextScanID   uint64
	bundleCounts map[string]uint64
	resultSeed   []scanResult
}

type scan struct {
	Request map[string]any
	Results []scanResult
}

type scanRequest struct {
	Target          target              `json:"target"`
	ScanPreferences []scannerPreference `json:"scan_preferences,omitempty"`
	VTs             []vt                `json:"vts"`
}

type target struct {
	Hosts              []string          `json:"hosts"`
	ExcludedHosts      []string          `json:"excluded_hosts,omitempty"`
	Ports              []portRange       `json:"ports,omitempty"`
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

// scanResult intentionally matches the OpenVASD result boundary used by gvmd.
type scanResult struct {
	ID        int          `json:"id"`
	Type      string       `json:"type"`
	IPAddress string       `json:"ip_address"`
	Hostname  string       `json:"hostname,omitempty"`
	OID       string       `json:"oid"`
	Port      int          `json:"port"`
	Protocol  string       `json:"protocol"`
	Message   string       `json:"message"`
	Detail    resultDetail `json:"detail"`
}

type resultDetail struct {
	Name   string       `json:"name"`
	Value  string       `json:"value"`
	Source resultSource `json:"source"`
}

type resultSource struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func New(cfg Config) (*Backend, error) {
	if cfg.SampleProvider == nil {
		return nil, fmt.Errorf("agent control sample provider is not configured")
	}
	if cfg.TimeoutDelay <= 0 {
		cfg.TimeoutDelay = 11 * time.Second
	}
	b := &Backend{
		resetToken:     cfg.ResetToken,
		timeoutDelay:   cfg.TimeoutDelay,
		sampleProvider: cfg.SampleProvider,
		routerProvider: cfg.RouterProvider,
	}
	if err := b.reset(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setMetadataHeaders(w)

	if r.URL.Path == "/_emulator/agent-control/reset" {
		b.handleReset(w, r)
		return
	}
	if r.URL.Path == "/_emulator/agent-control/provenance" {
		b.handleProvenance(w, r)
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
	if scenario != "success" && b.knownOperation(r.Method, r.URL.Path) && injectableRequest(r) {
		b.writeFailureScenario(w, scenario)
		return
	}

	path := canonicalPath(r.URL.Path)
	switch {
	case path == "/health/alive" || path == "/health/ready" || path == "/health/started":
		b.handleHealth(w, r)
	case path == "/api/v1/admin/agents":
		b.handleAgents(w, r)
	case path == "/api/v1/admin/agents/delete":
		b.handleDeleteAgents(w, r)
	case path == "/api/v1/admin/scan-agent-config":
		b.handleScanAgentConfig(w, r)
	case pathSegments(path) == 5 && strings.HasPrefix(path, "/api/v1/agents/") && strings.HasSuffix(path, "/config"):
		b.handleAgentConfig(w, r, pathSegment(path, 3))
	case pathSegments(path) == 5 &&
		strings.HasPrefix(path, "/api/v1/agents/") &&
		strings.HasSuffix(path, "/support-bundle"):
		b.handleSupportBundle(w, r, pathSegment(path, 3))
	case path == "/scans":
		b.handleScans(w, r)
	case pathSegments(path) == 3 && strings.HasPrefix(path, "/scans/") && strings.HasSuffix(path, "/results"):
		b.handleScanResults(w, r, pathSegment(path, 1))
	default:
		b.failClosed(w, r)
	}
}

func (b *Backend) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Backend) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		page, perPage, ok := pagination(r)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid pagination", nil)
			return
		}
		b.mu.Lock()
		items := make([]map[string]any, 0, len(b.agentOrder))
		for _, id := range b.agentOrder {
			if agent := b.agents[id]; agent != nil {
				items = append(items, cloneMap(agent))
			}
		}
		b.mu.Unlock()
		total := len(items)
		start, end := pageBounds(total, page, perPage)
		w.Header().Set("x-total-count", strconv.Itoa(total))
		w.Header().Set("x-page", strconv.Itoa(page))
		w.Header().Set("x-per-page", strconv.Itoa(perPage))
		writeJSON(w, http.StatusOK, items[start:end])
	case http.MethodPatch:
		if !validQuery(r, nil) {
			writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
			return
		}
		var updates map[string]map[string]json.RawMessage
		if err := decodeJSON(r, &updates); err != nil || len(updates) == 0 {
			writeError(w, http.StatusBadRequest, "invalid agent update", errorDetails(err))
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		unknown := make([]string, 0)
		for id := range updates {
			if b.agents[id] == nil {
				unknown = append(unknown, id)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			writeError(w, http.StatusUnprocessableEntity, "unknown agents", map[string]any{"agentids": unknown})
			return
		}
		for id, update := range updates {
			if err := validateAgentUpdate(update); err != nil {
				writeError(w, http.StatusBadRequest, "invalid agent update", map[string]any{
					"agentid": id, "details": err.Error(),
				})
				return
			}
		}
		for id, update := range updates {
			applyAgentUpdate(b.agents[id], update)
		}
		writeJSON(w, http.StatusOK, "")
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPatch)
	}
}

func (b *Backend) handleDeleteAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	var ids []string
	if err := decodeJSON(r, &ids); err != nil || len(ids) == 0 {
		writeError(w, http.StatusBadRequest, "invalid agent deletion", errorDetails(err))
		return
	}
	if hasDuplicates(ids) {
		writeError(w, http.StatusBadRequest, "duplicate agent id", nil)
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	unknown := make([]string, 0)
	for _, id := range ids {
		if b.agents[id] == nil {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "unknown agents", map[string]any{"agentids": unknown})
		return
	}
	for _, id := range ids {
		delete(b.agents, id)
	}
	writeJSON(w, http.StatusOK, "")
}

func (b *Backend) handleAgentConfig(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") ||
		strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")) == "" {
		writeError(w, http.StatusUnauthorized, "authorization required", nil)
		return
	}
	b.mu.Lock()
	agent := cloneMap(b.agents[id])
	b.mu.Unlock()
	if len(agent) == 0 {
		writeError(w, http.StatusNotFound, "agent not found", map[string]any{"agentid": id})
		return
	}
	config, _ := agent["config"].(map[string]any)
	writeJSON(w, http.StatusOK, cloneMap(config))
}

func (b *Backend) handleScanAgentConfig(w http.ResponseWriter, r *http.Request) {
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	switch r.Method {
	case http.MethodGet:
		b.mu.Lock()
		config := cloneMap(b.scanConfig)
		b.mu.Unlock()
		writeJSON(w, http.StatusOK, config)
	case http.MethodPut:
		var update map[string]json.RawMessage
		if err := decodeJSON(r, &update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid scan-agent configuration", errorDetails(err))
			return
		}
		if err := validateScanConfig(update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid scan-agent configuration", map[string]any{"details": err.Error()})
			return
		}
		b.mu.Lock()
		for key, raw := range update {
			var value any
			_ = json.Unmarshal(raw, &value)
			b.scanConfig[key] = value
		}
		b.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}

func (b *Backend) handleSupportBundle(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	days, encryption, ok := supportBundleQuery(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid support-bundle query", nil)
		return
	}
	b.mu.Lock()
	agent := b.agents[id]
	if agent == nil {
		b.mu.Unlock()
		writeError(w, http.StatusNotFound, "agent not found", map[string]any{"agentid": id})
		return
	}
	if agent["authorized"] != true || agent["connection_status"] != "active" {
		b.mu.Unlock()
		writeError(w, http.StatusConflict, "support bundle unavailable for agent state", map[string]any{"agentid": id})
		return
	}
	b.bundleCounts[id]++
	revision := b.bundleCounts[id]
	b.mu.Unlock()

	filename := fmt.Sprintf("support-bundle-%s-%03d.tar.gz", safeFilename(id), revision)
	if encryption {
		filename += ".enc"
	}
	body := fmt.Appendf(nil, "agentid=%s\ndays=%d\nencryption=%t\nrevision=%d\n", id, days, encryption, revision)
	w.Header().Set("content-type", "application/octet-stream")
	w.Header().Set("content-disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("x-emulator-support-bundle-revision", strconv.FormatUint(revision, 10))
	w.WriteHeader(http.StatusOK)
	// #nosec G705 -- this is an octet-stream test fixture for a seeded agent,
	// never HTML; safeFilename also bounds the request-derived identifier.
	_, _ = w.Write(body)
}

func (b *Backend) handleScans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		page, perPage, ok := pagination(r)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid pagination", nil)
			return
		}
		b.mu.Lock()
		items := make([]map[string]any, 0, len(b.scanOrder))
		for _, id := range b.scanOrder {
			item := b.scans[id]
			items = append(items, map[string]any{"scan_id": id, "request": cloneMap(item.Request)})
		}
		b.mu.Unlock()
		total := len(items)
		start, end := pageBounds(total, page, perPage)
		w.Header().Set("x-total-count", strconv.Itoa(total))
		w.Header().Set("x-page", strconv.Itoa(page))
		w.Header().Set("x-per-page", strconv.Itoa(perPage))
		writeJSON(w, http.StatusOK, items[start:end])
	case http.MethodPost:
		if !validQuery(r, nil) {
			writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
			return
		}
		var request scanRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid scan request", errorDetails(err))
			return
		}
		if len(request.Target.Hosts) == 0 {
			writeError(w, http.StatusBadRequest, "scan target must contain at least one host", nil)
			return
		}
		if request.VTs == nil {
			writeError(w, http.StatusBadRequest, "scan request must contain vts", nil)
			return
		}
		for _, requestedVT := range request.VTs {
			if strings.TrimSpace(requestedVT.OID) == "" {
				writeError(w, http.StatusBadRequest, "scan VT OIDs must not be empty", nil)
				return
			}
		}
		b.mu.Lock()
		agentIDs, err := b.resolveScanAgents(request.Target.Hosts)
		if err != nil {
			b.mu.Unlock()
			writeError(
				w,
				http.StatusBadRequest,
				"scan target does not resolve to eligible agents",
				map[string]any{"details": err.Error()},
			)
			return
		}
		b.nextScanID++
		id := fmt.Sprintf("00000000-0000-4000-a000-%012d", b.nextScanID)
		requestMap := structToMap(request)
		results := b.scanResults(id, request, agentIDs)
		b.scans[id] = &scan{Request: requestMap, Results: results}
		b.scanOrder = append(b.scanOrder, id)
		b.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]any{"scan_id": id})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (b *Backend) handleScanResults(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	b.mu.Lock()
	stored := b.scans[id]
	var results []scanResult
	if stored != nil {
		results = slices.Clone(stored.Results)
	}
	b.mu.Unlock()
	if stored == nil {
		writeError(w, http.StatusNotFound, "scan not found", map[string]any{"scan_id": id})
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (b *Backend) resolveScanAgents(hosts []string) ([]string, error) {
	ids := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		var matched string
		for id, agent := range b.agents {
			if id == host || agent["hostname"] == host ||
				stringSliceContains(agent["ip_addresses"], host) {
				if matched != "" {
					return nil, fmt.Errorf("host %q matches multiple agents", host)
				}
				matched = id
			}
		}
		if matched == "" {
			return nil, fmt.Errorf("host %q is not in the agent inventory", host)
		}
		agent := b.agents[matched]
		if agent["authorized"] != true || agent["connection_status"] != "active" {
			return nil, fmt.Errorf("agent %q is not active and authorized", matched)
		}
		if _, duplicate := seen[matched]; duplicate {
			return nil, fmt.Errorf("agent %q is repeated", matched)
		}
		seen[matched] = struct{}{}
		ids = append(ids, matched)
	}
	return ids, nil
}

func (b *Backend) scanResults(scanID string, request scanRequest, agentIDs []string) []scanResult {
	results := make([]scanResult, 0, len(agentIDs)*len(b.resultSeed))
	for index, agentID := range agentIDs {
		agent := b.agents[agentID]
		host := request.Target.Hosts[index]
		hostname, _ := agent["hostname"].(string)
		for _, seeded := range b.resultSeed {
			result := seeded
			result.ID = len(results)
			result.IPAddress = host
			result.Hostname = hostname
			result.Message = fmt.Sprintf(
				"[Agent Control scan %s; agent %s] %s", scanID, agentID, seeded.Message,
			)
			if result.Detail.Source.Description != "" {
				result.Detail.Source.Description += "; "
			}
			result.Detail.Source.Description += "correlated agent " + agentID
			results = append(results, result)
		}
	}
	return results
}

func (b *Backend) handleReset(w http.ResponseWriter, r *http.Request) {
	if b.resetToken == "" {
		writeError(w, http.StatusNotFound, "no route", nil)
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
	if subtle.ConstantTimeCompare([]byte(r.Header.Get(resetTokenHeader)), []byte(b.resetToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid reset token", nil)
		return
	}
	if r.Body != nil {
		var body map[string]json.RawMessage
		if err := decodeJSONAllowEmpty(r, &body); err != nil || len(body) != 0 {
			writeError(w, http.StatusBadRequest, "invalid reset request", errorDetails(err))
			return
		}
	}
	if err := b.reset(); err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			"cannot restore Agent Control seed",
			map[string]any{"details": err.Error()},
		)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (b *Backend) handleProvenance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !validQuery(r, nil) {
		writeError(w, http.StatusBadRequest, "unexpected query parameter", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"api_revision":      openAPIRevision,
		"fixture_revision":  fixtureRevision,
		"baseline_revision": baselineRevision,
		"seed_files": []string{
			"sample/api/v1/admin/agents/GET.json",
			"sample/api/v1/admin/scan-agent-config/GET.json",
			"sample/scans/{id}/results/GET.json",
		},
	})
}

func (b *Backend) reset() error {
	agentSample, err := b.sampleProvider.ResolveAndLoad(
		http.MethodGet, "/api/v1/admin/agents", "/api/v1/admin/agents", "GET__api_v1_admin_agents.json",
	)
	if err != nil {
		return fmt.Errorf("load agent inventory seed: %w", err)
	}
	var agents []map[string]any
	if err := json.Unmarshal(agentSample.Body, &agents); err != nil {
		return fmt.Errorf("invalid agent inventory seed: %w", err)
	}
	if len(agents) == 0 {
		return fmt.Errorf("invalid agent inventory seed: inventory is empty")
	}

	configSample, err := b.sampleProvider.ResolveAndLoad(
		http.MethodGet, "/api/v1/admin/scan-agent-config", "/api/v1/admin/scan-agent-config",
		"GET__api_v1_admin_scan-agent-config.json",
	)
	if err != nil {
		return fmt.Errorf("load scan-agent configuration seed: %w", err)
	}
	var agentDefaults map[string]any
	if err := json.Unmarshal(configSample.Body, &agentDefaults); err != nil {
		return fmt.Errorf("invalid scan-agent configuration seed: %w", err)
	}
	if len(agentDefaults) == 0 {
		return fmt.Errorf("invalid scan-agent configuration seed: configuration is empty")
	}
	resultSample, err := b.sampleProvider.ResolveAndLoad(
		http.MethodGet, "/scans/{id}/results", "/scans/seed/results", "GET__scans_{id}_results.json",
	)
	if err != nil {
		return fmt.Errorf("load scan result seed: %w", err)
	}
	var resultSeed []scanResult
	if err := json.Unmarshal(resultSample.Body, &resultSeed); err != nil {
		return fmt.Errorf("invalid scan result seed: %w", err)
	}
	if len(resultSeed) == 0 {
		return fmt.Errorf("invalid scan result seed: results are empty")
	}

	agentMap := make(map[string]map[string]any, len(agents))
	order := make([]string, 0, len(agents))
	for _, agent := range agents {
		id, _ := agent["agentid"].(string)
		if id == "" || agentMap[id] != nil {
			return fmt.Errorf("invalid or duplicate agent id %q", id)
		}
		agentMap[id] = cloneMap(agent)
		order = append(order, id)
	}

	b.mu.Lock()
	b.agents = agentMap
	b.agentOrder = order
	b.scanConfig = map[string]any{
		"agent_defaults":         cloneMap(agentDefaults),
		"agent_control_defaults": map[string]any{"update_to_latest": true},
	}
	b.scans = make(map[string]*scan)
	b.scanOrder = nil
	b.nextScanID = 0
	b.bundleCounts = make(map[string]uint64)
	b.resultSeed = slices.Clone(resultSeed)
	b.mu.Unlock()
	return nil
}

func (b *Backend) writeFailureScenario(w http.ResponseWriter, scenario string) {
	switch scenario {
	case "authentication":
		writeError(w, http.StatusUnauthorized, "injected authentication failure", nil)
	case "unavailable":
		writeError(w, http.StatusServiceUnavailable,
			"injected Agent Control unavailability", nil)
	case "malformed":
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"malformed":`))
	case "conflict":
		writeError(w, http.StatusConflict, "injected state conflict", nil)
	case "rate-limit":
		w.Header().Set("retry-after", "1")
		writeError(w, http.StatusTooManyRequests, "injected rate limit", nil)
	case "timeout":
		time.Sleep(b.timeoutDelay)
		writeError(w, http.StatusGatewayTimeout, "injected timeout", nil)
	}
}

func (b *Backend) failClosed(w http.ResponseWriter, r *http.Request) {
	if b.routerProvider != nil {
		if route := b.routerProvider.FindRoute(r.Method, r.URL.Path); route != nil {
			writeError(w, http.StatusNotImplemented,
				"known Agent Control operation is not implemented by this profile", map[string]any{
					"method": r.Method,
					"path":   r.URL.Path,
				})
			return
		}
		allowed := make([]string, 0)
		for _, route := range b.routerProvider.GetRoutes() {
			if route.Regex.MatchString(r.URL.Path) {
				allowed = append(allowed, route.Method)
			}
		}
		if len(allowed) > 0 {
			sort.Strings(allowed)
			methodNotAllowed(w, allowed...)
			return
		}
	}
	writeError(w, http.StatusNotFound, "no route", map[string]any{"method": r.Method, "path": r.URL.Path})
}

func (b *Backend) knownOperation(method, path string) bool {
	if path == "/_emulator/agent-control/reset" || path == "/_emulator/agent-control/provenance" {
		return method == http.MethodPost || method == http.MethodGet
	}
	if b.routerProvider != nil && b.routerProvider.FindRoute(method, path) != nil {
		return true
	}
	canonical := canonicalPath(path)
	return method == http.MethodGet &&
		(canonical == "/health/alive" || canonical == "/health/ready" || canonical == "/health/started")
}

func injectableRequest(r *http.Request) bool {
	path := canonicalPath(r.URL.Path)
	switch {
	case path == "/api/v1/admin/agents" && r.Method == http.MethodGet:
		_, _, ok := pagination(r)
		return ok
	case path == "/scans" && r.Method == http.MethodGet:
		_, _, ok := pagination(r)
		return ok
	case strings.HasPrefix(path, "/api/v1/agents/") &&
		strings.HasSuffix(path, "/support-bundle"):
		_, _, ok := supportBundleQuery(r)
		return ok
	default:
		return validQuery(r, nil)
	}
}

func canonicalPath(path string) string {
	if path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	for _, prefix := range []string{"/agent-control/v2/api", "/agent-control/protected/v2/api"} {
		if strings.HasPrefix(path, prefix+"/") {
			rest := strings.TrimPrefix(path, prefix)
			switch {
			case strings.HasPrefix(rest, "/admin/"):
				return "/api/v1" + rest
			case strings.HasPrefix(rest, "/agents/"):
				return "/api/v1" + rest
			case strings.HasPrefix(rest, "/health/"):
				return rest
			case rest == "/version":
				return "/api/v1/control/version"
			case rest == "/scans" || strings.HasPrefix(rest, "/scans/"):
				return rest
			}
		}
	}
	return path
}

func validateAgentUpdate(update map[string]json.RawMessage) error {
	if len(update) == 0 {
		return fmt.Errorf("update must contain at least one field")
	}
	for key, raw := range update {
		switch key {
		case "authorized", "update_to_latest":
			var value bool
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("%s must be a boolean", key)
			}
		case "heartbeat_timeout", "min_interval":
			var value int
			if err := json.Unmarshal(raw, &value); err != nil || value < 0 {
				return fmt.Errorf("%s must be a non-negative integer", key)
			}
		case "config":
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil || value == nil {
				return fmt.Errorf("config must be an object")
			}
		default:
			return fmt.Errorf("unknown update field %q", key)
		}
	}
	return nil
}

func applyAgentUpdate(agent map[string]any, update map[string]json.RawMessage) {
	for key, raw := range update {
		var value any
		_ = json.Unmarshal(raw, &value)
		agent[key] = value
	}
}

func validateScanConfig(update map[string]json.RawMessage) error {
	if len(update) == 0 {
		return fmt.Errorf("configuration must contain at least one section")
	}
	for key, raw := range update {
		switch key {
		case "agent_defaults":
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil || value == nil {
				return fmt.Errorf("agent_defaults must be an object")
			}
		case "agent_control_defaults":
			var value map[string]json.RawMessage
			if err := json.Unmarshal(raw, &value); err != nil || len(value) != 1 {
				return fmt.Errorf("agent_control_defaults must contain update_to_latest")
			}
			rawFlag, ok := value["update_to_latest"]
			if !ok {
				return fmt.Errorf("agent_control_defaults must contain update_to_latest")
			}
			var flag bool
			if err := json.Unmarshal(rawFlag, &flag); err != nil {
				return fmt.Errorf("update_to_latest must be a boolean")
			}
		default:
			return fmt.Errorf("unknown configuration section %q", key)
		}
	}
	return nil
}

func pagination(r *http.Request) (int, int, bool) {
	if !validQuery(r, map[string]func(string) bool{
		"page": func(value string) bool {
			return positiveInt(value, 1, int(^uint(0)>>1))
		},
		"per_page": func(value string) bool { return positiveInt(value, 1, maxPageSize) },
	}) {
		return 0, 0, false
	}
	page := 1
	perPage := defaultPageSize
	if value := r.URL.Query().Get("page"); value != "" {
		page, _ = strconv.Atoi(value)
	}
	if value := r.URL.Query().Get("per_page"); value != "" {
		perPage, _ = strconv.Atoi(value)
	}
	return page, perPage, true
}

func supportBundleQuery(r *http.Request) (int, bool, bool) {
	if !validQuery(r, map[string]func(string) bool{
		"days":       func(value string) bool { return positiveInt(value, 1, 365) },
		"encryption": func(value string) bool { return value == "true" || value == "false" },
	}) {
		return 0, false, false
	}
	days := supportBundleDays
	if value := r.URL.Query().Get("days"); value != "" {
		days, _ = strconv.Atoi(value)
	}
	encryption := true
	if value := r.URL.Query().Get("encryption"); value != "" {
		encryption, _ = strconv.ParseBool(value)
	}
	return days, encryption, true
}

func validQuery(r *http.Request, validators map[string]func(string) bool) bool {
	query := r.URL.Query()
	for name, values := range query {
		validator, ok := validators[name]
		if !ok || len(values) != 1 || !validator(values[0]) {
			return false
		}
	}
	return true
}

func positiveInt(value string, lower, upper int) bool {
	number, err := strconv.Atoi(value)
	return err == nil && number >= lower && number <= upper
}

func pageBounds(total, page, perPage int) (int, int) {
	start := total
	if page-1 <= total/perPage {
		start = (page - 1) * perPage
	}
	return start, min(start+perPage, total)
}

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBody+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func decodeJSONAllowEmpty(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBody+1))
	err := decoder.Decode(target)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("multiple JSON values are not allowed")
	}
	return nil
}

func setMetadataHeaders(w http.ResponseWriter) {
	w.Header().Set("api-version", apiVersion)
	w.Header().Set("x-emulator-openapi-revision", openAPIRevision)
	w.Header().Set("x-emulator-fixture-revision", fixtureRevision)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	if status == http.StatusNoContent {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string, fields map[string]any) {
	body := map[string]any{"error": message, "status": status}
	for key, value := range fields {
		body[key] = value
	}
	writeJSON(w, status, body)
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("allow", strings.Join(methods, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method not allowed", map[string]any{"allowed": methods})
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	data, _ := json.Marshal(input)
	var output map[string]any
	_ = json.Unmarshal(data, &output)
	return output
}

func structToMap(input any) map[string]any {
	data, _ := json.Marshal(input)
	var output map[string]any
	_ = json.Unmarshal(data, &output)
	return output
}

func errorDetails(err error) map[string]any {
	if err == nil {
		return nil
	}
	return map[string]any{"details": err.Error()}
}

func hasDuplicates(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return true
		}
		if _, ok := seen[value]; ok {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func stringSliceContains(value any, needle string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func pathSegment(path string, index int) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if index < 0 || index >= len(parts) {
		return ""
	}
	return parts[index]
}

func pathSegments(path string) int {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "/"))
}

func safeFilename(value string) string {
	return strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' {
			return character
		}
		return '-'
	}, value)
}
