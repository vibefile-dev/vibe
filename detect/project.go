package detect

// ProjectInfo holds detected information about the project's language and tooling.
type ProjectInfo struct {
	Language       string            // "go", "node", "python", "rust"
	Framework      string            // "gin", "next", "flask" (optional)
	PackageManager string            // "go", "npm", "pip", "cargo"
	Version        string            // language version from go.mod, .nvmrc, etc.
	BinaryName     string            // inferred from module path or package.json name
	Module         string            // go module path, npm package name, etc.
	Modules        []string          // workspace module directories (e.g. from go.work)
	HasTests       bool              // test files detected
	Metadata       map[string]string // detector-specific extras
}

// SubProject pairs a detected project with its relative directory within a monorepo.
// Dir is "" for root-level projects and e.g. "go" or "ui" for subdirectory projects.
type SubProject struct {
	Dir     string
	Project *ProjectInfo
}

// AddonResult is returned by an Addon when it detects a matching tool/platform.
type AddonResult struct {
	Label   string           // human-readable label, e.g. "Docker", "Helm"
	Targets []TemplateTarget // targets contributed by this addon
}
