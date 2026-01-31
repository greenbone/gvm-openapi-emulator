package samples

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozgen/openapi-sample-emulator/logger"

	"github.com/ozgen/openapi-sample-emulator/config"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockScenarioResolver struct {
	mock.Mock
}

func (m *MockScenarioResolver) ResolveScenarioFile(
	sc *Scenario,
	method string,
	swaggerTpl string,
	actualPath string,
) (file string, state string, err error) {
	args := m.Called(sc, method, swaggerTpl, actualPath)

	file, _ = args.Get(0).(string)
	state, _ = args.Get(1).(string)
	err = args.Error(2)
	return
}

func (m *MockScenarioResolver) TryResetByRequest(method, actualPath string) bool {
	args := m.Called(method, actualPath)
	return args.Bool(0)
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)

	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))

	return p
}

func TestLoadFile_ReadError(t *testing.T) {
	_, err := loadFile("/no/such/dir/missing.json")
	require.Error(t, err)
}

func TestLoadFile_EmptyFile_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "empty.json", "   \n\t  ")

	resp, err := loadFile(p)
	require.NoError(t, err)

	require.Equal(t, 200, resp.Status)
	require.Equal(t, "application/json", resp.Headers["content-type"])
	require.Equal(t, "{}", string(resp.Body))
}

func TestLoadFile_Envelope_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "sample.json", `{"body":{"ok":true}}`)

	resp, err := loadFile(p)
	require.NoError(t, err)

	require.Equal(t, 200, resp.Status)
	require.Equal(t, "application/json", resp.Headers["content-type"])
	require.Equal(t, `{"ok":true}`, string(resp.Body))
}

func TestLoadFile_Envelope_ExplicitValues(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "sample.json", `{
	  "status": 201,
	  "headers": {"content-type":"application/problem+json","x-test":"1"},
	  "body": {"id": 123}
	}`)

	resp, err := loadFile(p)
	require.NoError(t, err)

	require.Equal(t, 201, resp.Status)
	require.Equal(t, "application/problem+json", resp.Headers["content-type"])
	require.Equal(t, "1", resp.Headers["x-test"])
	require.Equal(t, `{"id":123}`, string(resp.Body))
}

func TestLoadFile_Envelope_StatusOnly_DefaultsHeaderAndBody(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "sample.json", `{"status":204}`)

	resp, err := loadFile(p)
	require.NoError(t, err)

	require.Equal(t, 204, resp.Status)
	require.Equal(t, "application/json", resp.Headers["content-type"])
	require.Equal(t, `{}`, string(resp.Body))
}

func TestLoadFile_Envelope_HeadersPresentButBodyMissing(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "hdrs.json", `{"headers":{"content-type":"text/plain"}}`)

	resp, err := loadFile(p)
	require.NoError(t, err)

	require.Equal(t, 200, resp.Status)
	require.Equal(t, "text/plain", resp.Headers["content-type"])
	require.Equal(t, `{}`, string(resp.Body))
}

func TestLoadFile_Envelope_ContentTypeNotInjectedIfAlreadyPresentCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "case.json", `{
	  "headers": {"Content-Type":"text/plain"},
	  "body": {"ok": true}
	}`)

	resp, err := loadFile(p)
	require.NoError(t, err)

	require.Equal(t, "text/plain", resp.Headers["Content-Type"])
	_, injected := resp.Headers["content-type"]
	require.False(t, injected, "did not expect injected lowercase content-type when Content-Type already exists")
	require.Equal(t, `{"ok":true}`, string(resp.Body))
}

func TestLoadFile_Fallback_RawJSONOrText(t *testing.T) {
	dir := t.TempDir()

	t.Run("raw json without envelope", func(t *testing.T) {
		p := writeFile(t, dir, "raw.json", `{}`)

		resp, err := loadFile(p)
		require.NoError(t, err)

		require.Equal(t, 200, resp.Status)
		require.Equal(t, "application/json", resp.Headers["content-type"])
		require.Equal(t, `{}`, string(resp.Body))
	})

	t.Run("plain text", func(t *testing.T) {
		p := writeFile(t, dir, "raw.txt", `  hello world  `)

		resp, err := loadFile(p)
		require.NoError(t, err)

		require.Equal(t, 200, resp.Status)
		require.Equal(t, "application/json", resp.Headers["content-type"])
		require.Equal(t, `hello world`, string(resp.Body))
	})
}

func TestBuildCandidates_LayoutFolders(t *testing.T) {
	got := buildCandidates(config.LayoutFolders, "GET", "/api/v1/items", "GET_api_v1_items.json")
	require.Equal(t, []string{filepath.Join("api", "v1", "items", "GET.json")}, got)
}

func TestBuildCandidates_LayoutFlat(t *testing.T) {
	got := buildCandidates(config.LayoutFlat, "GET", "/api/v1/items", "GET_api_v1_items.json")
	require.Equal(t, []string{"GET_api_v1_items.json"}, got)
}

func TestBuildCandidates_LayoutAuto_PrefersFoldersThenFlat(t *testing.T) {
	got := buildCandidates(config.LayoutAuto, "GET", "/api/v1/items", "GET_api_v1_items.json")
	require.Equal(t, []string{
		filepath.Join("api", "v1", "items", "GET.json"),
		"GET_api_v1_items.json",
	}, got)
}

func TestSampleProvider_ResolveAndLoad_FoldersMode_LoadsFolderSample(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerTpl := "/api/v1/items"
	actualPath := "/api/v1/items"
	legacyFlat := "GET_api_v1_items.json"

	writeFile(t, baseDir, filepath.Join("api", "v1", "items", "GET.json"), `{"body":{"ok":true}}`)

	p := NewSampleProvider(ProviderConfig{
		BaseDir: baseDir,
		Layout:  config.LayoutFolders,
	}, logger.GetLogger())

	resp, err := p.ResolveAndLoad(method, swaggerTpl, actualPath, legacyFlat)
	require.NoError(t, err)

	require.Equal(t, 200, resp.Status)
	require.Equal(t, "application/json", resp.Headers["content-type"])
	require.Equal(t, `{"ok":true}`, string(resp.Body))
}

func TestSampleProvider_ResolveAndLoad_FlatMode_LoadsLegacyFlatSample(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerTpl := "/api/v1/items"
	actualPath := "/api/v1/items"
	legacyFlat := "GET_api_v1_items.json"

	writeFile(t, baseDir, legacyFlat, `{"body":{"from":"flat"}}`)

	p := NewSampleProvider(ProviderConfig{
		BaseDir: baseDir,
		Layout:  config.LayoutFlat,
	}, logger.GetLogger())

	resp, err := p.ResolveAndLoad(method, swaggerTpl, actualPath, legacyFlat)
	require.NoError(t, err)

	require.Equal(t, `{"from":"flat"}`, string(resp.Body))
}

func TestSampleProvider_ResolveAndLoad_AutoMode_PrefersFoldersOverFlat(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerTpl := "/api/v1/items"
	actualPath := "/api/v1/items"
	legacyFlat := "GET_api_v1_items.json"

	writeFile(t, baseDir, filepath.Join("api", "v1", "items", "GET.json"), `{"body":{"from":"folders"}}`)
	writeFile(t, baseDir, legacyFlat, `{"body":{"from":"flat"}}`)

	p := NewSampleProvider(ProviderConfig{
		BaseDir: baseDir,
		Layout:  config.LayoutAuto,
	}, logger.GetLogger())

	resp, err := p.ResolveAndLoad(method, swaggerTpl, actualPath, legacyFlat)
	require.NoError(t, err)

	require.Equal(t, `{"from":"folders"}`, string(resp.Body))
}

func TestSampleProvider_ResolvePath_MissingSample_ReturnsError(t *testing.T) {
	baseDir := t.TempDir()

	p := NewSampleProvider(ProviderConfig{
		BaseDir: baseDir,
		Layout:  config.LayoutAuto,
	}, logger.GetLogger())

	_, err := p.ResolvePath("GET", "/api/v1/does-not-exist", "/api/v1/does-not-exist", "GET_api_v1_does_not_exist.json")
	require.Error(t, err)
}

func TestSampleProvider_ScenarioEnabled_UsesScenarioEngine(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerTpl := "/api/v1/items/{id}"
	actualPath := "/api/v1/items/123"
	legacyFlat := "GET_api_v1_items_{id}.json"

	scenarioFilename := "scenario.json"
	scPath := ScenarioPathForSwagger(baseDir, swaggerTpl, scenarioFilename)

	writeFile(t, filepath.Dir(scPath), filepath.Base(scPath), `{
	  "version": 1,
	  "mode": "step",
	  "key": { "pathParam": "id" },
	  "sequence": [{"state":"requested","file":"GET.requested.json"}],
	  "behavior": {}
	}`)

	writeFile(t, filepath.Dir(scPath), "GET.requested.json", `{"body":{"from":"scenario"}}`)

	m := new(MockScenarioResolver)

	m.On("ResolveScenarioFile", mock.Anything, "GET", swaggerTpl, actualPath).
		Return("GET.requested.json", "requested", nil).
		Once()

	m.AssertNotCalled(t, "TryResetByRequest", mock.Anything, mock.Anything)

	p := NewSampleProvider(ProviderConfig{
		BaseDir:          baseDir,
		Layout:           config.LayoutAuto,
		ScenarioEnabled:  true,
		ScenarioFilename: scenarioFilename,
		Engine:           m,
	}, logger.GetLogger())

	resp, err := p.ResolveAndLoad(method, swaggerTpl, actualPath, legacyFlat)
	require.NoError(t, err)
	require.Equal(t, `{"from":"scenario"}`, string(resp.Body))

	m.AssertExpectations(t)
}

func TestSampleProvider_ScenarioEnabled_EngineNil_ReturnsError(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerTpl := "/api/v1/items/{id}"
	actualPath := "/api/v1/items/123"

	scenarioFilename := "scenario.json"
	scPath := ScenarioPathForSwagger(baseDir, swaggerTpl, scenarioFilename)

	writeFile(t, filepath.Dir(scPath), filepath.Base(scPath), `{
	  "version": 1,
	  "mode": "step",
	  "key": { "pathParam": "id" },
	  "sequence": [{"state":"requested","file":"GET.requested.json"}],
	  "behavior": {}
	}`)

	p := NewSampleProvider(ProviderConfig{
		BaseDir:          baseDir,
		Layout:           config.LayoutAuto,
		ScenarioEnabled:  true,
		ScenarioFilename: scenarioFilename,
		Engine:           nil,
	}, logger.GetLogger())

	_, err := p.ResolvePath(method, swaggerTpl, actualPath, "legacy.json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "engine is nil")
}

func TestSampleProvider_ScenarioEnabled_FileReturnedButMissing_ReturnsError(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerTpl := "/api/v1/items/{id}"
	actualPath := "/api/v1/items/123"

	scenarioFilename := "scenario.json"
	scPath := ScenarioPathForSwagger(baseDir, swaggerTpl, scenarioFilename)

	writeFile(t, filepath.Dir(scPath), filepath.Base(scPath), `{
	  "version": 1,
	  "mode": "step",
	  "key": { "pathParam": "id" },
	  "sequence": [{"state":"requested","file":"GET.requested.json"}],
	  "behavior": {}
	}`)

	m := new(MockScenarioResolver)
	m.On("ResolveScenarioFile", mock.Anything, "GET", swaggerTpl, actualPath).
		Return("GET.requested.json", "requested", nil).
		Once()

	p := NewSampleProvider(ProviderConfig{
		BaseDir:          baseDir,
		Layout:           config.LayoutAuto,
		ScenarioEnabled:  true,
		ScenarioFilename: scenarioFilename,
		Engine:           m,
	}, logger.GetLogger())

	_, err := p.ResolvePath(method, swaggerTpl, actualPath, "legacy.json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "scenario file not found")

	m.AssertExpectations(t)
}

func TestSampleProvider_ScenarioEnabled_NoScenarioFile_CallsTryResetByRequest(t *testing.T) {
	baseDir := t.TempDir()

	method := "DELETE"
	swaggerTpl := "/scans/{id}"
	actualPath := "/scans/123"

	legacyFlat := "DELETE__scans_{id}.json"

	m := new(MockScenarioResolver)
	m.On("TryResetByRequest", "DELETE", actualPath).Return(true).Once()

	p := NewSampleProvider(ProviderConfig{
		BaseDir:          baseDir,
		Layout:           config.LayoutAuto,
		ScenarioEnabled:  true,
		ScenarioFilename: "scenario.json",
		Engine:           m,
	}, logger.GetLogger())

	_, err := p.ResolvePath(method, swaggerTpl, actualPath, legacyFlat)
	require.Error(t, err)
	m.AssertExpectations(t)
}

func TestSampleProvider_ScenarioEnabled_NoScenarioFile_TryResetFalse_StillCalled(t *testing.T) {
	baseDir := t.TempDir()

	method := "DELETE"
	swaggerTpl := "/scans/{id}"
	actualPath := "/scans/123"
	legacyFlat := "DELETE__scans_{id}.json"

	m := new(MockScenarioResolver)
	m.On("TryResetByRequest", "DELETE", actualPath).Return(false).Once()

	p := NewSampleProvider(ProviderConfig{
		BaseDir:          baseDir,
		Layout:           config.LayoutAuto,
		ScenarioEnabled:  true,
		ScenarioFilename: "scenario.json",
		Engine:           m,
	}, logger.GetLogger())

	_, err := p.ResolvePath(method, swaggerTpl, actualPath, legacyFlat)
	require.Error(t, err)
	m.AssertExpectations(t)
}

func TestSampleProvider_ScenarioEnabled_ScenarioFileExists_DoesNotCallTryResetByRequest(t *testing.T) {
	baseDir := t.TempDir()

	method := "GET"
	swaggerTpl := "/api/v1/items/{id}"
	actualPath := "/api/v1/items/123"
	legacyFlat := "GET__api_v1_items_{id}.json"

	scenarioFilename := "scenario.json"
	scPath := ScenarioPathForSwagger(baseDir, swaggerTpl, scenarioFilename)

	writeFile(t, filepath.Dir(scPath), filepath.Base(scPath), `{
	  "version": 1,
	  "mode": "step",
	  "key": { "pathParam": "id" },
	  "sequence": [{"state":"requested","file":"GET.requested.json"}],
	  "behavior": {}
	}`)

	writeFile(t, filepath.Dir(scPath), "GET.requested.json", `{"body":{"ok":true}}`)

	m := new(MockScenarioResolver)
	m.On("ResolveScenarioFile", mock.Anything, "GET", swaggerTpl, actualPath).
		Return("GET.requested.json", "requested", nil).
		Once()

	p := NewSampleProvider(ProviderConfig{
		BaseDir:          baseDir,
		Layout:           config.LayoutAuto,
		ScenarioEnabled:  true,
		ScenarioFilename: scenarioFilename,
		Engine:           m,
	}, logger.GetLogger())

	_, err := p.ResolveAndLoad(method, swaggerTpl, actualPath, legacyFlat)
	require.NoError(t, err)

	m.AssertNotCalled(t, "TryResetByRequest", mock.Anything, mock.Anything)
	m.AssertExpectations(t)
}
