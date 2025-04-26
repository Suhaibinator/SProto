package mapper

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Suhaibinator/SProto/internal/api" // Import api package
	"github.com/Suhaibinator/SProto/internal/config"
	"go.uber.org/zap" // Added for logging
)

// WellKnownMappings defines standard import path prefixes and their assumed module identifiers.
// Adjust these as needed based on how standard protos are stored in your registry.
var WellKnownMappings = map[string]ModuleIdentifier{
	"google/protobuf": {Namespace: "google", Name: "protobuf"},
	// Add other well-known prefixes if necessary (e.g., googleapis)
	// "google/api":      {Namespace: "google", Name: "api"},
}

// ModuleIdentifier identifies a module by namespace and name.
type ModuleIdentifier struct {
	Namespace string
	Name      string
}

// ImportMapper maps import path prefixes to module identifiers.
type ImportMapper struct {
	mappings map[string]ModuleIdentifier
	prefixes []string // sorted by length for longest-prefix matching
	logger   *zap.Logger
}

// NewImportMapper creates a new empty mapper and adds well-known mappings.
func NewImportMapper(logger *zap.Logger) *ImportMapper {
	mapper := &ImportMapper{
		mappings: make(map[string]ModuleIdentifier),
		prefixes: []string{},
		logger:   logger.Named("import_mapper"), // Create a named logger
	}
	mapper.addWellKnownMappings()
	return mapper
}

// addWellKnownMappings pre-populates the mapper with standard mappings.
func (m *ImportMapper) addWellKnownMappings() {
	for prefix, moduleID := range WellKnownMappings {
		err := m.AddMapping(prefix, moduleID)
		if err != nil {
			// Log error but don't fail initialization; allows overriding well-known paths if needed.
			m.logger.Warn("Failed to add well-known mapping (potential override)",
				zap.String("prefix", prefix),
				zap.String("module", fmt.Sprintf("%s/%s", moduleID.Namespace, moduleID.Name)),
				zap.Error(err))
		} else {
			m.logger.Debug("Added well-known mapping",
				zap.String("prefix", prefix),
				zap.String("module", fmt.Sprintf("%s/%s", moduleID.Namespace, moduleID.Name)))
		}
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

// isLikelyRelative determines if an import path is relative.
// A path is considered relative if it starts with '.' or '..' or doesn't contain a '/'.
func isLikelyRelative(importPath string) bool {
	return strings.HasPrefix(importPath, ".") || strings.HasPrefix(importPath, "..") || !strings.Contains(importPath, "/")
}

// ResolveImport finds the module corresponding to a given import path.
// It handles both absolute paths (e.g., github.com/...) and relative paths
// based on the context of the importing file's path.
// Returns the module identifier and true if found, false otherwise.
func (m *ImportMapper) ResolveImport(importPath string, importerPath string) (ModuleIdentifier, bool) {
	if importPath == "" {
		return ModuleIdentifier{}, false
	}

	resolvedPath := importPath // Start with the original path
	isRelative := isLikelyRelative(importPath)

	if isRelative && importerPath != "" {
		// It's relative, resolve it based on the importer's path directory
		importerDir := path.Dir(importerPath)
		joinedPath := path.Join(importerDir, importPath)
		resolvedPath = path.Clean(joinedPath) // Clean the path (removes .., .)
		m.logger.Debug("Resolved relative import",
			zap.String("original_import", importPath),
			zap.String("importer_path", importerPath),
			zap.String("resolved_path", resolvedPath))
	} else {
		// Assume absolute-like, clean it just in case
		resolvedPath = path.Clean(importPath)
	}

	// Now perform longest-prefix matching on the resolved path
	for _, prefix := range m.prefixes {
		// For prefix matching to work properly:
		// 1. The resolvedPath must have content after the prefix (hence the prefix + "/" check)
		// 2. If the resolvedPath exactly equals the prefix, it's not a valid import (no file specified)
		if resolvedPath == prefix {
			// Not a valid import path - needs to include a file
			continue
		}
		if strings.HasPrefix(resolvedPath, prefix+"/") {
			m.logger.Debug("Resolved import path to module",
				zap.String("resolved_path", resolvedPath),
				zap.String("matched_prefix", prefix),
				zap.Any("module", m.mappings[prefix]))
			return m.mappings[prefix], true
		}
	}
	return ModuleIdentifier{}, false
}

// RegistryClient defines the interface needed by the mapper to load data from the registry.
// This helps decouple the mapper from the specific CLI client implementation.
type RegistryClient interface {
	FetchAllModules() ([]api.ModuleInfo, error)
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

// GetModuleImportPrefix returns the import path prefix for a given module ID.
// This provides bidirectional mapping capability.
// Returns the import path prefix and true if found, false otherwise.
func (m *ImportMapper) GetModuleImportPrefix(namespace, name string) (string, bool) {
	target := ModuleIdentifier{Namespace: namespace, Name: name}

	for prefix, module := range m.mappings {
		if module == target {
			return prefix, true
		}
	}

	return "", false
}
