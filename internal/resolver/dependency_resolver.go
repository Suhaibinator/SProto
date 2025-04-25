package resolver

import (
	"fmt"
	"strings" // Ensure strings is imported

	// "github.com/Suhaibinator/SProto/internal/api" // Avoid direct dependency on api package types if possible
	"sort" // Added import

	"github.com/Masterminds/semver/v3" // Added import
	"github.com/vbauerster/mpb/v8"     // Import mpb
	"go.uber.org/zap"
)

// --- Helper Functions ---

// sortVersionsDesc sorts a slice of version strings semantically descending.
func sortVersionsDesc(versions []string) {
	semvers := make([]*semver.Version, 0, len(versions))
	for _, vStr := range versions {
		v, err := semver.NewVersion(vStr)
		if err == nil {
			semvers = append(semvers, v)
		} else {
			// Log or handle parse error? For now, just skip unparseable versions.
		}
	}
	sort.Sort(sort.Reverse(semver.Collection(semvers)))
	// Overwrite the original slice with sorted versions, ensuring 'v' prefix
	for i, v := range semvers {
		versions[i] = "v" + v.String()
	}
}

// --- Interfaces for Data Access ---

// ModuleInfo holds basic module metadata needed by the resolver.
// Define locally to avoid direct dependency on api package struct if it changes.
type ModuleInfo struct {
	Namespace  string
	Name       string
	ImportPath *string
}

// DependencyInfo holds dependency details needed by the resolver.
type DependencyInfo struct {
	Namespace         string
	Name              string
	VersionConstraint string
}

// RegistryAccessor defines the interface required by the resolver to fetch data.
// This allows decoupling from the specific client/database implementation.
type RegistryAccessor interface {
	GetModuleInfo(namespace, name string) (*ModuleInfo, error)
	GetModuleVersions(namespace, name string) ([]string, error)
	GetModuleDependencies(namespace, name string) ([]DependencyInfo, error)
}

// --- Resolver Implementation ---

// generateModuleID generates a unique string identifier for a module.
func generateModuleID(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// DependencyResolver is responsible for resolving module dependencies.
type DependencyResolver struct {
	accessor RegistryAccessor // Use the defined interface
	logger   *zap.Logger
	graph    *DependencyGraph
	visited  map[string]bool // Track visited modules during graph build
	progress *mpb.Progress   // Progress bar container
}

// NewDependencyResolver creates a new instance of the DependencyResolver.
func NewDependencyResolver(accessor RegistryAccessor, logger *zap.Logger, progress *mpb.Progress) *DependencyResolver {
	return &DependencyResolver{
		accessor: accessor,
		logger:   logger,
		graph:    NewDependencyGraph(),
		visited:  make(map[string]bool),
		progress: progress, // Store progress container
	}
}

// ResolveRootModule builds the dependency graph starting from a root module
// and then resolves the versions.
func (r *DependencyResolver) ResolveRootModule(rootNamespace, rootName, rootVersion string) (ResolvedDependencies, error) {
	r.logger.Info("Starting dependency resolution",
		zap.String("root_module", fmt.Sprintf("%s/%s@%s", rootNamespace, rootName, rootVersion)))

	// Reset visited map for this resolution run
	r.visited = make(map[string]bool)

	// Start recursive graph building
	err := r.buildGraphRecursive(rootNamespace, rootName)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	r.logger.Info("Dependency graph built successfully", zap.Int("modules", len(r.graph.GetModules())))

	// Now resolve versions using the built graph
	rootID := generateModuleID(rootNamespace, rootName)
	resolved, err := r.graph.ResolveVersions(rootID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve versions: %w", err)
	}

	r.logger.Info("Version resolution successful", zap.Int("resolved_count", len(resolved)))
	return resolved, nil
}

// buildGraphRecursive fetches metadata and dependencies for a module and adds them to the graph.
// It recursively calls itself for dependencies.
func (r *DependencyResolver) buildGraphRecursive(namespace, name string) error {
	moduleID := generateModuleID(namespace, name) // Use local helper

	// Check if already visited to prevent infinite loops (cycle handling)
	if r.visited[moduleID] {
		r.logger.Debug("Module already visited, skipping", zap.String("module_id", moduleID))
		return nil
	}
	r.visited[moduleID] = true
	r.logger.Debug("Building graph node", zap.String("module_id", moduleID))

	// 1. Fetch module metadata (including import path) via accessor
	metaInfo, err := r.accessor.GetModuleInfo(namespace, name)
	if err != nil {
		// Check specific error types or inspect error message to differentiate
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no rows") {
			// Module not found in registry
			return fmt.Errorf("module '%s' not found in registry: %w", moduleID, err)
		}
		// Other errors (connectivity, permission, etc.)
		return fmt.Errorf("failed to fetch metadata for module '%s': %w", moduleID, err)
	}
	importPath := ""
	if metaInfo.ImportPath != nil {
		importPath = *metaInfo.ImportPath
	}

	// 2. Fetch available versions via accessor
	versions, err := r.accessor.GetModuleVersions(namespace, name)
	if err != nil {
		return fmt.Errorf("failed to fetch versions for module '%s': %w", moduleID, err)
	}

	// Sort versions descending semantically for resolver logic
	sortVersionsDesc(versions) // Use the helper function defined above

	// 3. Add module to graph (if not already added implicitly by AddDependency)
	moduleMeta := ModuleMetadata{
		Namespace:  namespace,
		Name:       name,
		ImportPath: importPath,
		Versions:   versions, // Use sorted versions
	}
	// Use AddModule to ensure metadata is stored, ignore "already exists" error from DAG vertex add.
	err = r.graph.AddModule(moduleMeta)
	if err != nil && !strings.Contains(err.Error(), "already exists") { // Be careful with error string matching
		return fmt.Errorf("failed to add module '%s' to graph: %w", moduleID, err)
	}

	// 4. Fetch dependencies for this module via accessor
	dependencies, err := r.accessor.GetModuleDependencies(namespace, name)
	if err != nil {
		// Treat failure to fetch dependencies as potentially non-fatal? Or fail?
		// Let's fail for now, as it indicates an incomplete graph.
		// Let's fail for now, as it indicates an incomplete graph.
		return fmt.Errorf("failed to fetch dependencies for module '%s': %w", moduleID, err)
	}

	if len(dependencies) == 0 {
		r.logger.Debug("Module has no dependencies", zap.String("module_id", moduleID))
		return nil // Base case for recursion
	}

	r.logger.Debug("Found dependencies", zap.String("module_id", moduleID), zap.Int("count", len(dependencies)))

	// 5. Add dependencies to graph and recurse
	for _, dep := range dependencies {
		depID := generateModuleID(dep.Namespace, dep.Name) // Use local helper
		r.logger.Debug("Adding dependency edge",
			zap.String("from", moduleID),
			zap.String("to", depID),
			zap.String("constraint", dep.VersionConstraint))

		// Add the dependency edge (AddDependency checks if modules exist)
		// Need to ensure the target module exists first before adding edge,
		// or modify AddDependency to handle this. Let's fetch/add target first.

		// Ensure target module is processed (this will add it if needed)
		err = r.buildGraphRecursive(dep.Namespace, dep.Name)
		if err != nil {
			// Propagate error from recursive call
			return fmt.Errorf("failed processing dependency '%s' for module '%s': %w", depID, moduleID, err)
		}

		// Now add the edge (both modules should exist)
		err = r.graph.AddDependency(namespace, name, dep.Namespace, dep.Name, dep.VersionConstraint)
		if err != nil && !strings.Contains(err.Error(), "already exists") { // Ignore duplicate edge errors
			return fmt.Errorf("failed to add dependency edge from '%s' to '%s': %w", moduleID, depID, err)
		}
	}

	return nil
}
