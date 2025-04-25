package api

// ResolveDependenciesResponse defines the structure for the dependency resolution endpoint response.
type ResolveDependenciesResponse struct {
	Root                 ModuleVersionInfoType   `json:"root"`
	Dependencies         []ModuleVersionInfoType `json:"dependencies"`          // Flattened list of resolved dependencies
	ImportPaths          map[string]string       `json:"import_paths"`          // Maps import paths to module versions
	Errors               []string                `json:"errors,omitempty"`      // Any resolution errors encountered
	ResolvedDependencies map[string]string       `json:"resolved_dependencies"` // Maps module IDs to resolved versions
}

// ModuleVersionInfoType contains details for a specific resolved module version.
type ModuleVersionInfoType struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	ImportPath string `json:"import_path,omitempty"`
}
