package resolver

import (
	"fmt"
	"strings" // Ensure strings is imported

	"github.com/Suhaibinator/SProto/internal/api"
	"github.com/vbauerster/mpb/v8" // Import mpb
	"go.uber.org/zap"
)

// generateModuleID generates a unique string identifier for a module.
func generateModuleID(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// DependencyResolver is responsible for resolving module dependencies
// and coordinating with the registry client to fetch information.
type DependencyResolver struct {
	client interface { // Define an interface for required client methods
		FetchModuleMetadata(namespace, name string) (*api.ModuleInfo, error)
		FetchModuleVersions(namespace, name string) ([]string, error)
		FetchModuleDependencies(namespace, name string) ([]api.DependencyResponse, error) // Added method
	}
	logger   *zap.Logger
	graph    *DependencyGraph
	visited  map[string]bool // Track visited modules during graph build
	progress *mpb.Progress   // Progress bar container
}

// NewDependencyResolver creates a new instance of the DependencyResolver.
func NewDependencyResolver(client interface { // Use the defined interface
	FetchModuleMetadata(namespace, name string) (*api.ModuleInfo, error)
	FetchModuleVersions(namespace, name string) ([]string, error)
	FetchModuleDependencies(namespace, name string) ([]api.DependencyResponse, error)
},
	logger *zap.Logger, progress *mpb.Progress) *DependencyResolver { // Add progress parameter

	return &DependencyResolver{
		client:   client,
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

	// 1. Fetch module metadata (including import path)
	metaInfo, err := r.client.FetchModuleMetadata(namespace, name)
	if err != nil {
		// If module not found, it's an error in resolution context
		return fmt.Errorf("failed to fetch metadata for module '%s': %w", moduleID, err)
	}
	importPath := ""
	if metaInfo.ImportPath != nil {
		importPath = *metaInfo.ImportPath
	}

	// 2. Fetch available versions
	versions, err := r.client.FetchModuleVersions(namespace, name)
	if err != nil {
		return fmt.Errorf("failed to fetch versions for module '%s': %w", moduleID, err)
	}

	// 3. Add module to graph (if not already added implicitly by AddDependency)
	moduleMeta := ModuleMetadata{
		Namespace:  namespace,
		Name:       name,
		ImportPath: importPath,
		Versions:   versions,
	}
	// Use AddModule to ensure metadata is stored, ignore "already exists" error from DAG vertex add.
	err = r.graph.AddModule(moduleMeta)
	if err != nil && !strings.Contains(err.Error(), "already exists") { // Be careful with error string matching
		return fmt.Errorf("failed to add module '%s' to graph: %w", moduleID, err)
	}

	// 4. Fetch dependencies for this module
	dependencies, err := r.client.FetchModuleDependencies(namespace, name)
	if err != nil {
		// Treat failure to fetch dependencies as potentially non-fatal? Or fail?
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
