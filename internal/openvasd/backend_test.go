// SPDX-FileCopyrightText: 2026 Greenbone AG
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package openvasd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/greenbone/gvm-openapi-emulator/config"
	"github.com/greenbone/gvm-openapi-emulator/internal/samples"
	"github.com/sirupsen/logrus"
)

func TestHealthMetadataReadinessAndVTInformation(t *testing.T) {
	backend := New(Config{ResetToken: "reset-secret", TimeoutDelay: time.Millisecond, SampleProvider: vtSampleProvider()})

	response := request(t, backend, http.MethodHead, "/health", "", nil)
	assertStatus(t, response, http.StatusOK)
	assertMetadata(t, response)

	for _, path := range []string{"/health/alive", "/health/started"} {
		response = request(t, backend, http.MethodGet, path, "", nil)
		assertStatus(t, response, http.StatusOK)
		assertMetadata(t, response)
	}

	response = request(t, backend, http.MethodGet, "/health/ready", "", nil)
	assertStatus(t, response, http.StatusServiceUnavailable)
	response = request(t, backend, http.MethodGet, "/health/ready", "", nil)
	assertStatus(t, response, http.StatusOK)

	response = request(t, backend, http.MethodGet, "/vts", "", nil)
	assertStatus(t, response, http.StatusOK)
	var oids []string
	decodeResponse(t, response, &oids)
	if len(oids) == 0 || oids[0] == "" {
		t.Fatalf("unexpected VT OIDs: %#v", oids)
	}

	response = request(t, backend, http.MethodGet, "/vts?information=1", "", nil)
	assertStatus(t, response, http.StatusOK)
	var information []map[string]any
	decodeResponse(t, response, &information)
	if len(information) != len(oids) || information[0]["oid"] != oids[0] {
		t.Fatalf("unexpected VT information: %#v", information)
	}
	for _, field := range []string{"name", "filename", "tag", "category", "family"} {
		if _, exists := information[0][field]; !exists {
			t.Fatalf("VT information missing %q: %#v", field, information[0])
		}
	}

	response = request(t, backend, http.MethodGet, "/vts?information=0", "", nil)
	assertStatus(t, response, http.StatusBadRequest)
	response = request(t, backend, http.MethodGet, "/vts?unknown=1", "", nil)
	assertStatus(t, response, http.StatusBadRequest)
}

// TestVTInformationIncludesGvmdCompatibleSeverityMetadata prevents gvmd from
// rejecting every emulated VT and importing zero VTs during readiness.
func TestVTInformationIncludesGvmdCompatibleSeverityMetadata(t *testing.T) {
	backend := New(Config{SampleProvider: vtSampleProvider()})
	response := request(t, backend, http.MethodGet, "/vts?information=1", "", nil)
	assertStatus(t, response, http.StatusOK)

	var information []struct {
		OID string `json:"oid"`
		Tag struct {
			CVSSBaseVector string `json:"cvss_base_vector"`
		} `json:"tag"`
	}
	decodeResponse(t, response, &information)

	if len(information) == 0 {
		t.Fatal("expected VT information")
	}
	for _, vt := range information {
		if vt.Tag.CVSSBaseVector != "AV:N/AC:L/Au:N/C:N/I:N/A:N" {
			t.Errorf("VT %q has invalid gvmd severity vector %q", vt.OID, vt.Tag.CVSSBaseVector)
		}
	}
}

func TestVTSampleMatchesScanResultOIDs(t *testing.T) {
	baseDir := filepath.Join("..", "..", "examples", "openvasd", "sample")
	resultSample, err := os.ReadFile(filepath.Join(baseDir, "scans", "{id}", "results", "GET.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected := make(map[string]bool)
	for _, oid := range regexp.MustCompile(`1\.3\.6\.1\.4\.1\.25623(?:\.\d+)+`).FindAllString(string(resultSample), -1) {
		expected[oid] = true
	}
	if len(expected) == 0 {
		t.Fatal("scan result sample must reference VTs")
	}

	backend := New(Config{SampleProvider: vtSampleProvider()})
	for _, query := range []string{"information=1", "information=true"} {
		response := request(t, backend, http.MethodGet, "/vts?"+query, "", nil)
		assertStatus(t, response, http.StatusOK)
		var information []vt
		decodeResponse(t, response, &information)
		if len(information) != len(expected) {
			t.Fatalf("expected %d referenced VTs, got %d", len(expected), len(information))
		}
		for _, item := range information {
			if !expected[item.OID] {
				t.Errorf("VT %q is not referenced by the scan result sample", item.OID)
			}
		}
		response = request(t, backend, http.MethodGet, "/vts", "", nil)
		assertStatus(t, response, http.StatusOK)
		var oids []string
		decodeResponse(t, response, &oids)
		if len(oids) != len(information) {
			t.Fatalf("VT inventory and metadata lengths differ")
		}
		for index, oid := range oids {
			if oid != information[index].OID {
				t.Errorf("inventory OID %q does not match metadata OID %q", oid, information[index].OID)
			}
		}
	}
}

func TestVTResponsesFailClosedWithoutValidSample(t *testing.T) {
	for _, body := range []string{"missing", `{`, `null`, `[]`, `[{}]`, `[{"oid":"one"},{"oid":"one"}]`} {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			if body != "missing" {
				if err := os.WriteFile(filepath.Join(dir, "GET__vts.json"), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			provider := samples.NewSampleProvider(samples.ProviderConfig{
				BaseDir: dir,
				Layout:  config.LayoutFlat,
			}, logrus.New())
			backend := New(Config{SampleProvider: provider})
			for _, path := range []string{"/vts", "/vts?information=1"} {
				response := request(t, backend, http.MethodGet, path, "", nil)
				assertStatus(t, response, http.StatusInternalServerError)
				assertErrorBody(t, response)
			}
		})
	}
}

func vtSampleProvider() samples.ISampleProvider {
	return samples.NewSampleProvider(samples.ProviderConfig{
		BaseDir: filepath.Join("..", "..", "examples", "openvasd", "sample"),
		Layout:  config.LayoutFolders,
	}, logrus.New())
}

func TestResultsCoverEveryTargetAndVT(t *testing.T) {
	backend := New(Config{ResetToken: "reset-secret", TimeoutDelay: time.Millisecond})
	body := `{"target":{"hosts":["192.0.2.1","192.0.2.2"],"ports":[{"protocol":"tcp","range":[{"start":443}]}]},"scan_preferences":[{"id":"safe_checks","value":"yes"}],"vts":[{"oid":"1.3.6.1.4.1.25623.1.0.100196"},{"oid":"1.3.6.1.4.1.25623.1.0.100127"}]}`
	response := request(t, backend, http.MethodPost, "/scans", body, nil)
	assertStatus(t, response, http.StatusCreated)
	var id string
	decodeResponse(t, response, &id)
	response = request(t, backend, http.MethodPost, "/scans/"+id, `{"action":"start"}`, nil)
	assertStatus(t, response, http.StatusNoContent)
	for range 2 {
		response = request(t, backend, http.MethodGet, "/scans/"+id+"/status", "", nil)
		assertStatus(t, response, http.StatusOK)
	}
	response = request(t, backend, http.MethodGet, "/scans/"+id+"/results", "", nil)
	assertStatus(t, response, http.StatusOK)
	var results []result
	decodeResponse(t, response, &results)
	if len(results) != 4 {
		t.Fatalf("expected one result per target and VT, got %#v", results)
	}
	pairs := make(map[string]struct{}, len(results))
	for index, item := range results {
		if item.ID != index || !strings.Contains(item.Message,
			`preferences [{"id":"safe_checks","value":"yes"}]`) {
			t.Fatalf("result did not preserve configuration correlation: %#v", item)
		}
		pairs[item.IPAddress+"/"+item.OID] = struct{}{}
	}
	if len(pairs) != 4 {
		t.Fatalf("target/VT pairs are not isolated: %#v", results)
	}
}

func TestConcurrentScanIsolation(t *testing.T) {
	backend := New(Config{ResetToken: "reset-secret", TimeoutDelay: time.Millisecond})
	const scanCount = 12

	ids := make(chan string, scanCount)
	errors := make(chan error, scanCount)
	var group sync.WaitGroup
	for index := range scanCount {
		group.Add(1)
		go func() {
			defer group.Done()
			host := fmt.Sprintf("192.0.2.%d", index+1)
			body := fmt.Sprintf(
				`{"target":{"hosts":[%q],"ports":[{"protocol":"tcp","range":[{"start":443}]}]},"vts":[{"oid":"1.3.6.1.4.1.25623.1.0.100196"}]}`,
				host,
			)
			response := request(t, backend, http.MethodPost, "/scans", body, nil)
			if response.Code != http.StatusCreated {
				errors <- fmt.Errorf("create %s: status %d: %s", host, response.Code, response.Body.String())
				return
			}
			var id string
			if err := json.Unmarshal(response.Body.Bytes(), &id); err != nil {
				errors <- fmt.Errorf("create %s: decode id: %w", host, err)
				return
			}
			ids <- id

			response = request(t, backend, http.MethodPost, "/scans/"+id, `{"action":"start"}`, nil)
			if response.Code != http.StatusNoContent {
				errors <- fmt.Errorf("start %s: status %d", id, response.Code)
				return
			}
			for range 3 {
				response = request(t, backend, http.MethodGet, "/scans/"+id+"/status", "", nil)
				if response.Code != http.StatusOK {
					errors <- fmt.Errorf("status %s: status %d", id, response.Code)
					return
				}
			}
			response = request(t, backend, http.MethodGet, "/scans/"+id+"/results", "", nil)
			var results []result
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &results) != nil ||
				len(results) != 1 || results[0].IPAddress != host ||
				!strings.Contains(results[0].Message, id) {
				errors <- fmt.Errorf("results %s are not isolated: status=%d body=%s",
					id, response.Code, response.Body.String())
			}
		}()
	}
	group.Wait()
	close(ids)
	close(errors)

	for err := range errors {
		t.Error(err)
	}
	unique := make(map[string]struct{}, scanCount)
	for id := range ids {
		unique[id] = struct{}{}
	}
	if len(unique) != scanCount {
		t.Fatalf("expected %d unique scan IDs, got %d: %#v", scanCount, len(unique), unique)
	}
}

func TestScanLifecycleCorrelationAndIsolation(t *testing.T) {
	backend := New(Config{ResetToken: "reset-secret", TimeoutDelay: time.Millisecond})

	firstID := createScan(t, backend, "192.0.2.10", "1.3.6.1.4.1.25623.1.0.100196", "")
	secondID := createScan(t, backend, "198.51.100.20", "1.3.6.1.4.1.25623.1.0.100127", "")
	if firstID == secondID {
		t.Fatalf("scan IDs are not unique: %q", firstID)
	}

	response := request(t, backend, http.MethodGet, "/scans/"+firstID, "", nil)
	assertStatus(t, response, http.StatusOK)
	var first scanResponse
	decodeResponse(t, response, &first)
	if first.ScanID != firstID || first.Target.Hosts[0] != "192.0.2.10" || first.VTs[0].OID == "" {
		t.Fatalf("scan configuration lost correlation: %#v", first)
	}

	response = request(t, backend, http.MethodPost, "/scans/"+firstID, `{"action":"start"}`, nil)
	assertStatus(t, response, http.StatusNoContent)
	if response.Body.Len() != 0 {
		t.Fatalf("204 action response has a body: %q", response.Body.String())
	}

	// Starting a scheduled scan is idempotent.
	response = request(t, backend, http.MethodPost, "/scans/"+firstID, `{"action":"start"}`, nil)
	assertStatus(t, response, http.StatusNoContent)

	for _, expected := range []string{"requested", "running", "succeeded"} {
		response = request(t, backend, http.MethodGet, "/scans/"+firstID+"/status", "", nil)
		assertStatus(t, response, http.StatusOK)
		var status scanStatus
		decodeResponse(t, response, &status)
		if status.Status != expected {
			t.Fatalf("expected status %q, got %#v", expected, status)
		}
	}

	// The overlapping scan has independent state.
	response = request(t, backend, http.MethodGet, "/scans/"+secondID+"/status", "", nil)
	assertStatus(t, response, http.StatusOK)
	var secondStatus scanStatus
	decodeResponse(t, response, &secondStatus)
	if secondStatus.Status != "stored" {
		t.Fatalf("second scan was advanced by first scan: %#v", secondStatus)
	}

	response = request(t, backend, http.MethodGet, "/scans/"+firstID+"/results", "", nil)
	assertStatus(t, response, http.StatusOK)
	var results []result
	decodeResponse(t, response, &results)
	if len(results) != 1 || results[0].IPAddress != "192.0.2.10" || results[0].OID != first.VTs[0].OID {
		t.Fatalf("result lost target/VT correlation: %#v", results)
	}
	if !strings.Contains(results[0].Message, firstID) {
		t.Fatalf("result does not identify its scan: %#v", results[0])
	}

	response = request(t, backend, http.MethodGet, "/scans/"+firstID+"/results/0", "", nil)
	assertStatus(t, response, http.StatusOK)
	var single result
	decodeResponse(t, response, &single)
	if single != results[0] {
		t.Fatalf("single result differs from collection: %#v != %#v", single, results[0])
	}

	response = request(t, backend, http.MethodGet, "/scans/"+firstID+"/results?range=0-0", "", nil)
	assertStatus(t, response, http.StatusOK)
	decodeResponse(t, response, &results)
	if len(results) != 1 {
		t.Fatalf("unexpected ranged results: %#v", results)
	}

	response = request(t, backend, http.MethodPost, "/scans/"+firstID, `{"action":"start"}`, nil)
	assertStatus(t, response, http.StatusConflict)

	response = request(t, backend, http.MethodDelete, "/scans/"+firstID, "", nil)
	assertStatus(t, response, http.StatusNoContent)
	response = request(t, backend, http.MethodDelete, "/scans/"+firstID, "", nil)
	assertStatus(t, response, http.StatusNotFound)
}

func TestStopDeleteAndFailedScanTransitions(t *testing.T) {
	backend := New(Config{ResetToken: "reset-secret", TimeoutDelay: time.Millisecond})

	stoppedID := createScan(t, backend, "203.0.113.10", "1.3.6.1.4.1.25623.1.0.100196", "")
	response := request(t, backend, http.MethodPost, "/scans/"+stoppedID, `{"action":"start"}`, nil)
	assertStatus(t, response, http.StatusNoContent)
	response = request(t, backend, http.MethodDelete, "/scans/"+stoppedID, "", nil)
	assertStatus(t, response, http.StatusConflict)
	response = request(t, backend, http.MethodPost, "/scans/"+stoppedID, `{"action":"stop"}`, nil)
	assertStatus(t, response, http.StatusNoContent)
	response = request(t, backend, http.MethodPost, "/scans/"+stoppedID, `{"action":"stop"}`, nil)
	assertStatus(t, response, http.StatusNoContent)
	response = request(t, backend, http.MethodGet, "/scans/"+stoppedID+"/status", "", nil)
	assertStatus(t, response, http.StatusOK)
	var status scanStatus
	decodeResponse(t, response, &status)
	if status.Status != "stopped" {
		t.Fatalf("expected stopped, got %#v", status)
	}
	response = request(t, backend, http.MethodDelete, "/scans/"+stoppedID, "", nil)
	assertStatus(t, response, http.StatusNoContent)

	failedID := createScan(t, backend, "203.0.113.20", "1.3.6.1.4.1.25623.1.0.100127", "failed-scan")
	response = request(t, backend, http.MethodPost, "/scans/"+failedID, `{"action":"start"}`, nil)
	assertStatus(t, response, http.StatusNoContent)
	for _, expected := range []string{"requested", "running", "failed"} {
		response = request(t, backend, http.MethodGet, "/scans/"+failedID+"/status", "", nil)
		assertStatus(t, response, http.StatusOK)
		decodeResponse(t, response, &status)
		if status.Status != expected {
			t.Fatalf("expected status %q, got %#v", expected, status)
		}
	}
}

func TestFailureScenariosResetAndFailClosedBehavior(t *testing.T) {
	backend := New(Config{ResetToken: "reset-secret", TimeoutDelay: 20 * time.Millisecond})

	for scenario, expected := range map[string]int{
		"http-400": http.StatusBadRequest,
		"http-500": http.StatusInternalServerError,
	} {
		response := request(t, backend, http.MethodGet, "/vts", "", map[string]string{
			scenarioHeader: scenario,
		})
		assertStatus(t, response, expected)
		assertErrorBody(t, response)
	}

	response := request(t, backend, http.MethodGet, "/vts", "", map[string]string{
		scenarioHeader: "malformed-body",
	})
	assertStatus(t, response, http.StatusOK)
	if json.Valid(response.Body.Bytes()) {
		t.Fatalf("malformed-body returned valid JSON: %q", response.Body.String())
	}

	response = request(t, backend, http.MethodGet, "/vts", "", map[string]string{
		scenarioHeader: "malformed-header",
	})
	assertStatus(t, response, http.StatusOK)
	if response.Header().Get("api-version") != "malformed" {
		t.Fatalf("malformed-header did not alter api-version")
	}

	started := time.Now()
	response = request(t, backend, http.MethodGet, "/vts", "", map[string]string{
		scenarioHeader: "timeout",
	})
	assertStatus(t, response, http.StatusOK)
	if time.Since(started) < 20*time.Millisecond {
		t.Fatalf("timeout scenario returned too early")
	}

	for _, test := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodGet, "/does-not-exist", "", http.StatusNotFound},
		{http.MethodPut, "/vts", "", http.StatusMethodNotAllowed},
		{http.MethodGet, "/health/alive?fixture=unknown", "", http.StatusBadRequest},
		{http.MethodPost, "/scans", `{`, http.StatusBadRequest},
		{http.MethodPost, "/scans", `{"target":{"hosts":["192.0.2.1"],"ports":[]},"vts":[]}`, http.StatusBadRequest},
		{http.MethodGet, "/scans/missing/status", "", http.StatusNotFound},
		{http.MethodGet, "/scans/missing/results?range=bad", "", http.StatusBadRequest},
	} {
		response = request(t, backend, test.method, test.path, test.body, nil)
		assertStatus(t, response, test.status)
		assertErrorBody(t, response)
	}

	response = request(t, backend, http.MethodGet, "/vts", "", map[string]string{
		scenarioHeader: "unknown",
	})
	assertStatus(t, response, http.StatusBadRequest)
	response = request(t, backend, http.MethodGet, "/does-not-exist", "", map[string]string{
		scenarioHeader: "http-500",
	})
	assertStatus(t, response, http.StatusNotFound)
	response = request(t, backend, http.MethodPut, "/vts", "", map[string]string{
		scenarioHeader: "http-500",
	})
	assertStatus(t, response, http.StatusMethodNotAllowed)
	response = request(t, backend, http.MethodGet, "/vts?unknown=1", "", map[string]string{
		scenarioHeader: "http-500",
	})
	assertStatus(t, response, http.StatusBadRequest)

	createdID := createScan(t, backend, "192.0.2.30", "1.3.6.1.4.1.25623.1.0.100196", "")
	response = request(t, backend, http.MethodPost, "/_emulator/reset", "", nil)
	assertStatus(t, response, http.StatusUnauthorized)
	response = request(t, backend, http.MethodPost, "/_emulator/reset", "", map[string]string{
		resetTokenHeader: "wrong",
	})
	assertStatus(t, response, http.StatusUnauthorized)
	response = request(t, backend, http.MethodPost, "/_emulator/reset",
		`{"not_ready_responses":2}`, map[string]string{resetTokenHeader: "reset-secret"})
	assertStatus(t, response, http.StatusNoContent)

	response = request(t, backend, http.MethodGet, "/scans/"+createdID, "", nil)
	assertStatus(t, response, http.StatusNotFound)
	for _, expected := range []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK} {
		response = request(t, backend, http.MethodGet, "/health/ready", "", nil)
		assertStatus(t, response, expected)
	}
	reusedID := createScan(t, backend, "192.0.2.31", "1.3.6.1.4.1.25623.1.0.100196", "")
	if reusedID != createdID {
		t.Fatalf("reset did not restore deterministic IDs: %q != %q", reusedID, createdID)
	}

	disabled := New(Config{TimeoutDelay: time.Millisecond})
	response = request(t, disabled, http.MethodPost, "/_emulator/reset", "", map[string]string{
		resetTokenHeader: "anything",
	})
	assertStatus(t, response, http.StatusNotFound)
}

func createScan(t *testing.T, backend *Backend, host, oid, scenario string) string {
	t.Helper()
	body := `{"target":{"hosts":["` + host + `"],"ports":[{"protocol":"tcp","range":[{"start":443}]}]},"scan_preferences":[{"id":"safe_checks","value":"yes"}],"vts":[{"oid":"` + oid + `"}]}`
	headers := map[string]string{}
	if scenario != "" {
		headers[scenarioHeader] = scenario
	}
	response := request(t, backend, http.MethodPost, "/scans", body, headers)
	assertStatus(t, response, http.StatusCreated)
	var id string
	decodeResponse(t, response, &id)
	if id == "" {
		t.Fatal("create scan returned an empty ID")
	}
	return id
}

func request(t *testing.T, backend http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "http://emulator.example"+path, reader)
	if body != "" {
		req.Header.Set("content-type", "application/json")
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	backend.ServeHTTP(response, req)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %d %q: %v", response.Code, response.Body.String(), err)
	}
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if response.Code != expected {
		t.Fatalf("expected status %d, got %d: %s", expected, response.Code, response.Body.String())
	}
}

func assertMetadata(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	for name, expected := range map[string]string{
		"api-version":                 apiVersion,
		"feed-version":                feedVersion,
		"authentication":              authentication,
		"x-emulator-openapi-revision": openAPIRevision,
		"x-emulator-fixture-revision": fixtureRevision,
	} {
		if actual := response.Header().Get(name); actual != expected {
			t.Fatalf("header %s: expected %q, got %q", name, expected, actual)
		}
	}
}

func assertErrorBody(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]any
	decodeResponse(t, response, &body)
	if errorMessage, ok := body["error"].(string); !ok || errorMessage == "" {
		t.Fatalf("expected a non-empty error body, got %#v", body)
	}
}
