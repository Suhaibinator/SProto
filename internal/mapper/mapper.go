package mapper

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Suhaibinator/SProto/internal/api" // Import api package
	"github.com/Suhaibinator/SProto/internal/config"
)

// ModuleIdentifier identifies a module by namespace and name.
type ModuleIdentifier struct {
	Namespace string
	Name      string
}

// ImportMapper maps import path prefixes to module identifiers.
type ImportMapper struct {
	mappings map[string]ModuleIdentifier
	prefixes []string // sorted by length for longest-prefix matching
}

// NewImportMapper creates a new empty mapper.
func NewImportMapper() *ImportMapper {
	return &ImportMapper{
		mappings: make(map[string]ModuleIdentifier),
		prefixes: []string{},
	}
}

// AddMapping registers a mapping from an import path prefix to a module.
// Returns error if prefix already mapped to a different module.
func (m *ImportMapper) AddMapping(importPathPrefix string, module ModuleIdentifier) error {
	if existing, ok := m.mappings[importPathPrefix]; ok {
		if existing != module {
			return fmt.Errorf("conflicting mapping for prefix %s: existing %v, new %v", importPathPrefix, existing, module)
		}
		// same mapping, ignore
		return nil
	}
	m.mappings[importPathPrefix] = module
	m.prefixes = append(m.prefixes, importPathPrefix)
	// sort prefixes descending for longest-prefix match
	sort.Slice(m.prefixes, func(i, j int) bool {
		return len(m.prefixes[i]) > len(m.prefixes[j])
	})
	return nil
}

// LoadMappingsFromConfig loads mappings from a SProtoConfig struct,
// including the module itself and its dependencies.
func (m *ImportMapper) LoadMappingsFromConfig(cfg *config.SProtoConfig) error {
	// Self mapping
	if cfg.ImportPath == "" {
		return fmt.Errorf("module import_path is empty for %s", cfg.Name)
	}
	parts := strings.SplitN(cfg.Name, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid module name '%s'", cfg.Name)
	}
	if err := m.AddMapping(cfg.ImportPath, ModuleIdentifier{Namespace: parts[0], Name: parts[1]}); err != nil {
		return err
	}
	// Dependencies
	for _, dep := range cfg.Dependencies {
		if dep.ImportPath == "" {
			return fmt.Errorf("dependency %s/%s has empty import_path", dep.Namespace, dep.Name)
		}
		if err := m.AddMapping(dep.ImportPath, ModuleIdentifier{Namespace: dep.Namespace, Name: dep.Name}); err != nil {
			return err
		}
	}
	return nil
}

// ResolveImport finds the module corresponding to a given import path.
// Returns the module identifier and true if found, false otherwise.
func (m *ImportMapper) ResolveImport(importPath string) (ModuleIdentifier, bool) {
	for _, prefix := range m.prefixes {
		if strings.HasPrefix(importPath, prefix) {
			return m.mappings[prefix], true
		}
	}
	return ModuleIdentifier{}, false
}

// RegistryClient defines the interface needed by the mapper to load data from the registry.
// This helps decouple the mapper from the specific CLI client implementation.
type RegistryClient interface {
	FetchAllModules() ([]api.ModuleInfo, error)
	// TODO: Add other methods if needed, e.g., FetchModuleVersions, FetchModuleDependencies
}

// LoadMappingsFromRegistry loads mappings by querying the registry API.
func (m *ImportMapper) LoadMappingsFromRegistry(client RegistryClient) error {
	modules, err := client.FetchAllModules()
	if err != nil {
		return fmt.Errorf("failed to fetch modules from registry: %w", err)
	}

	for _, moduleInfo := range modules {
		if moduleInfo.ImportPath != nil && *moduleInfo.ImportPath != "" {
			moduleIDParts := strings.SplitN(fmt.Sprintf("%s/%s", moduleInfo.Namespace, moduleInfo.Name), "/", 2)
			if len(moduleIDParts) != 2 {
				// Should not happen if API returns valid module names, but safety check
				continue
			}
			moduleID := ModuleIdentifier{Namespace: moduleIDParts[0], Name: moduleIDParts[1]}
			if err := m.AddMapping(*moduleInfo.ImportPath, moduleID); err != nil {
				// Log the error but continue loading other mappings? Or fail fast?
				// Let's fail fast for now to indicate a problem with the registry data or conflicting mappings.
				return fmt.Errorf("failed to add mapping for module %s/%s (import path %s): %w",
					moduleInfo.Namespace, moduleInfo.Name, *moduleInfo.ImportPath, err)
			}
		}
	}
	return nil
}

// TODO: Add tests for LoadMappingsFromRegistry (Task 2.3.3)
// TODO: Add tests for ResolveImport edge cases (Task 2.3.2)
// TODO: Implement logic to handle well-known import paths (Task 2.3.1)
// TODO: Implement logic to resolve relative import paths (Task 2.3.2)
// TODO: Implement bidirectional mapping (Task 2.1.2 details, but fits here)
