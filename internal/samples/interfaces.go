package samples

type ISampleProvider interface {
	ResolveAndLoad(method, swaggerTpl, actualPath, legacyFlatFilename string) (*Response, error)
	ResolvePath(method, swaggerTpl, actualPath, legacyFlatFilename string) (string, error)
}

type IScenarioResolver interface {
	ResolveScenarioFile(
		sc *Scenario,
		method string,
		swaggerTpl string,
		actualPath string,
	) (file string, state string, err error)
	TryResetByRequest(method, actualPath string) bool
}
