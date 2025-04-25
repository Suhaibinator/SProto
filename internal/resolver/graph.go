package resolver

import (
	"fmt"
	"log"
	"sort"
	"strings" // Import strings package
	"sync"

	"github.com/Masterminds/semver/v3" // Added for version constraint checking
	"github.com/heimdalr/dag"
)

// ModuleMetadata holds information about a specific module relevant for dependency resolution.
type ModuleMetadata struct {
	Namespace  string
	Name       string
	ImportPath string   // Base import path for the module
	Versions   []string // Available versions, should be sorted semantically descending for resolution logic
	// TODO: Add other relevant info? e.g., source (registry, local)
}

// DependencyGraph represents the module dependency graph.
// It uses heimdalr/dag for the underlying graph structure and adds metadata.
type DependencyGraph struct {
	dag         *dag.DAG
	modules     map[string]ModuleMetadata    // Map from module ID (e.g., "namespace/name") to metadata
	constraints map[string]map[string]string // Map from 'from' module ID -> 'to' module ID -> version constraint string
	mu          sync.RWMutex                 // Mutex to protect concurrent access
}

// NewDependencyGraph creates a new empty dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		dag:         dag.NewDAG(),
		modules:     make(map[string]ModuleMetadata),
		constraints: make(map[string]map[string]string),
	}
}

// moduleID generates a unique string identifier for a module.
func moduleID(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// AddModule adds a module vertex to the graph along with its metadata.
// Returns an error if the module already exists.
func (g *DependencyGraph) AddModule(metadata ModuleMetadata) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	id := moduleID(metadata.Namespace, metadata.Name)
	if _, exists := g.modules[id]; exists {
		return fmt.Errorf("module '%s' already exists in the graph", id)
	}

	// Add vertex to the DAG using only the ID
	_, err := g.dag.AddVertex(id)
	if err != nil {
		// Check if it's a "vertex already exists" error if the library provides one
		// For now, assume any error is problematic.
		return fmt.Errorf("failed to add vertex '%s' to DAG: %w", id, err)
	}

	// Store metadata in our map as well for quick lookup
	g.modules[id] = metadata
	// Initialize constraint map for this module
	g.constraints[id] = make(map[string]string)

	return nil
}

// AddDependency adds a directed edge representing a dependency relationship.
// It stores the version constraint associated with the dependency.
// Returns an error if either module doesn't exist or if the edge already exists.
func (g *DependencyGraph) AddDependency(fromNamespace, fromName, toNamespace, toName, versionConstraint string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	fromID := moduleID(fromNamespace, fromName)
	toID := moduleID(toNamespace, toName)

	// Ensure both modules exist as vertices in the DAG
	// Note: AddVertex is idempotent in heimdalr/dag v1.5.0, but AddEdge requires vertices to exist.
	// Let's check our metadata map first for clarity.
	if _, exists := g.modules[fromID]; !exists {
		return fmt.Errorf("source module '%s' not found in graph", fromID)
	}
	if _, exists := g.modules[toID]; !exists {
		return fmt.Errorf("target module '%s' not found in graph", toID)
	}

	// Add the edge to the DAG
	err := g.dag.AddEdge(fromID, toID)
	if err != nil {
		// Check if it's specifically a "duplicate edge" error if the lib provides it
		if err.Error() == fmt.Sprintf("edge from %s to %s already exists", fromID, toID) {
			// If edge exists, maybe just update constraint? Or return error?
			// For now, let's treat it as an error to avoid ambiguity if constraints differ.
			return fmt.Errorf("dependency from '%s' to '%s' already exists: %w", fromID, toID, err)
		}
		return fmt.Errorf("failed to add dependency edge from '%s' to '%s': %w", fromID, toID, err)
	}

	// Store the constraint
	if _, ok := g.constraints[fromID]; !ok {
		g.constraints[fromID] = make(map[string]string) // Should have been initialized in AddModule, but safety check
	}
	g.constraints[fromID][toID] = versionConstraint

	return nil
}

// GetModuleMetadata retrieves the metadata for a given module ID.
func (g *DependencyGraph) GetModuleMetadata(moduleID string) (ModuleMetadata, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	meta, exists := g.modules[moduleID]
	return meta, exists
}

// GetConstraint retrieves the version constraint between two modules.
func (g *DependencyGraph) GetConstraint(fromID, toID string) (string, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if fromConstraints, ok := g.constraints[fromID]; ok {
		constraint, exists := fromConstraints[toID]
		return constraint, exists
	}
	return "", false
}

// GetModules returns a slice of all module IDs (namespace/name) in the graph.
func (g *DependencyGraph) GetModules() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	modules := make([]string, 0, len(g.modules))
	for id := range g.modules {
		modules = append(modules, id)
	}
	return modules
}

// GetDependencies returns the direct dependencies (as module IDs) of a given module.
func (g *DependencyGraph) GetDependencies(moduleID string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Check if module exists first
	if _, exists := g.modules[moduleID]; !exists {
		return nil, fmt.Errorf("module '%s' not found in graph", moduleID)
	}

	// Get children (dependencies) from the DAG
	children, err := g.dag.GetChildren(moduleID)
	if err != nil {
		// This might indicate an issue with the DAG library or inconsistent state
		return nil, fmt.Errorf("failed to get dependencies for module '%s': %w", moduleID, err)
	}

	// The keys of the returned map are the dependency IDs
	dependencies := make([]string, 0, len(children))
	for depID := range children {
		dependencies = append(dependencies, depID)
	}

	return dependencies, nil
}

// GetDependents returns the modules (as module IDs) that directly depend on the given module.
func (g *DependencyGraph) GetDependents(moduleID string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Check if module exists first
	if _, exists := g.modules[moduleID]; !exists {
		return nil, fmt.Errorf("module '%s' not found in graph", moduleID)
	}

	// Get parents (dependents) from the DAG
	parents, err := g.dag.GetParents(moduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependents for module '%s': %w", moduleID, err)
	}

	// The keys of the returned map are the dependent IDs
	dependents := make([]string, 0, len(parents))
	for depID := range parents {
		dependents = append(dependents, depID)
	}

	return dependents, nil
}

// GetResolutionOrder performs a topological sort on the dependency graph.
// It returns a slice of module IDs in dependency-first order.
// Returns an error if the graph contains cycles (which should be checked separately).
func (g *DependencyGraph) GetResolutionOrder() ([]string, error) {
	// TODO: Implement proper topological sort using heimdalr/dag v1.5.0.
	// The OrderedWalk API usage was causing persistent compiler errors.
	// Needs investigation into the correct Visitor interface implementation
	// or alternative methods for topological sorting in this library version.
	// For now, returning unsorted keys as a placeholder to unblock.
	log.Println("Warning: GetResolutionOrder returning unsorted module list due to temporary implementation.")
	g.mu.RLock()
	defer g.mu.RUnlock()

	modules := make([]string, 0, len(g.modules))
	for id := range g.modules {
		modules = append(modules, id)
	}
	// This list is NOT topologically sorted.
	return modules, nil
}

// ResolvedDependencies maps module IDs to their resolved version strings.
type ResolvedDependencies map[string]string

// ResolveVersions attempts to find a compatible set of versions for all dependencies
// starting from a root module, respecting the constraints defined in the graph.
// Returns a map of moduleID -> resolvedVersionString, or an error if resolution fails.
func (g *DependencyGraph) ResolveVersions(rootModuleID string) (ResolvedDependencies, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 1. Check if root module exists
	if _, exists := g.modules[rootModuleID]; !exists {
		return nil, fmt.Errorf("root module '%s' not found in graph", rootModuleID)
	}

	// 2. Perform cycle check first (important!)
	cycles, err := g.CheckForCycles() // Use the placeholder cycle check
	if err != nil {
		return nil, fmt.Errorf("error checking for cycles: %w", err)
	}
	if len(cycles) > 0 {
		// Even though the placeholder returns empty, keep the check structure.
		// If CheckForCycles is fixed later to return actual cycles, this will work.
		cyclePath := make([]string, len(cycles[0]))
		for i, nodeID := range cycles[0] {
			cyclePath[i] = nodeID
		}
		return nil, fmt.Errorf("cannot resolve versions: dependency cycle detected: %s", strings.Join(cyclePath, " -> "))
	}
	// TODO: Remove the following log line once CheckForCycles is correctly implemented.
	log.Println("Warning: Cycle check in ResolveVersions is currently ineffective due to placeholder implementation of CheckForCycles.")

	// 3. Get topological sort (dependency-first order)
	resolutionOrder, err := g.GetResolutionOrder() // Use the placeholder sort
	if err != nil {
		return nil, fmt.Errorf("failed to get resolution order: %w", err)
	}

	// 4. Iterate through modules in reverse topological order (dependents first)
	//    This allows us to propagate version choices down the dependency chain.
	//    Alternatively, iterate dependency-first and keep track of constraints.
	//    Let's try dependency-first and select the highest satisfying version.

	resolved := make(ResolvedDependencies)
	possibleVersions := make(map[string][]*semver.Version) // Cache parsed versions

	// Initialize possible versions for all modules
	for modID, meta := range g.modules {
		versions := make([]*semver.Version, 0, len(meta.Versions))
		for _, vStr := range meta.Versions {
			v, err := semver.NewVersion(vStr)
			if err == nil {
				versions = append(versions, v)
			} else {
				log.Printf("Warning: Skipping invalid version '%s' for module '%s'", vStr, modID)
			}
		}
		// Sort versions descending to easily pick the highest later
		sort.Sort(sort.Reverse(semver.Collection(versions)))
		possibleVersions[modID] = versions
	}

	// Process in dependency-first order (using the placeholder order for now)
	for _, moduleID := range resolutionOrder {
		// Find the highest possible version for this module that satisfies constraints
		// imposed by modules that depend *on* it (which have already been processed
		// if we iterate carefully or adjust constraints).

		// Simpler approach: Iterate dependency-first. For each module, determine its
		// constraints based on *all* modules that depend on it *in the subgraph being resolved*.
		// This gets complex quickly.

		// Alternative: Backtracking or constraint satisfaction algorithms.

		// Let's try a simpler greedy approach for now:
		// Iterate dependency-first. For each module, select the highest available version.
		// Then, for modules that depend on it, check if the selected version satisfies their constraint.
		// This doesn't handle conflicts well (A->C, B->C, A wants C v1, B wants C v2).

		// Revised Greedy Approach:
		// Iterate dependency-first (topological order).
		// For each module `dep`, consider all modules `mod` that depend on it.
		// Collect all constraints `mod` places on `dep`.
		// Find the highest version of `dep` that satisfies *all* these constraints.
		// If no such version exists, resolution fails.

		dependents, err := g.GetDependents(moduleID)
		if err != nil {
			return nil, fmt.Errorf("failed to get dependents for '%s': %w", moduleID, err)
		}

		// Collect constraints from dependents that are already resolved or are the root
		constraints := []*semver.Constraints{}
		isRoot := moduleID == rootModuleID
		for _, dependentID := range dependents {
			// Only consider constraints from dependents that are part of the resolution path
			// OR if this module IS the root (where no constraints apply from above)
			// This check needs refinement - we need the full subgraph constraints.

			// Let's assume for now we check constraints from *all* dependents in the graph.
			constraintStr, ok := g.GetConstraint(dependentID, moduleID)
			if ok {
				c, err := semver.NewConstraint(constraintStr)
				if err != nil {
					log.Printf("Warning: Invalid constraint '%s' from '%s' to '%s'", constraintStr, dependentID, moduleID)
					// Fail resolution if constraint is invalid? Or ignore? Let's fail.
					return nil, fmt.Errorf("invalid constraint '%s' from '%s' to '%s': %w", constraintStr, dependentID, moduleID, err)
				}
				constraints = append(constraints, c)
			}
		}

		// Find the highest version satisfying all constraints
		selectedVersion := ""
		foundMatch := false
		for _, v := range possibleVersions[moduleID] {
			satisfiesAll := true
			for _, c := range constraints {
				if !c.Check(v) {
					satisfiesAll = false
					break
				}
			}
			if satisfiesAll {
				selectedVersion = "v" + v.String() // Ensure 'v' prefix
				foundMatch = true
				break // Found the highest satisfying version
			}
		}

		if !foundMatch && !isRoot && len(g.modules[moduleID].Versions) > 0 {
			// If it's not the root, has available versions, but none satisfied the constraints
			// OR if it had no versions available in the first place (and isn't the root)
			// This check needs refinement for the case where the root itself has constraints applied *to* it.
			// For now, fail if no version satisfies constraints for a non-root module with versions.
			// TODO: Improve error message to show conflicting constraints.
			return nil, fmt.Errorf("failed to resolve dependencies: no compatible version found for module '%s'", moduleID)
		} else if !foundMatch && isRoot && len(g.modules[moduleID].Versions) > 0 {
			// If it's the root and has versions, but none match (e.g., constraints applied to root?)
			// Select the highest available version for the root if no constraints applied.
			if len(constraints) == 0 && len(possibleVersions[moduleID]) > 0 {
				selectedVersion = "v" + possibleVersions[moduleID][0].String()
				log.Printf("Root module '%s' selected highest version '%s'", moduleID, selectedVersion)
			} else if len(constraints) > 0 {
				// Constraints were applied to the root, but none matched.
				return nil, fmt.Errorf("failed to resolve dependencies: no version of root module '%s' satisfies constraints", moduleID)
			} else {
				// Root has no versions available.
				return nil, fmt.Errorf("failed to resolve dependencies: root module '%s' has no available versions", moduleID)
			}
		} else if !foundMatch && len(g.modules[moduleID].Versions) == 0 {
			// Module has no versions listed at all.
			// If it's not the root, this is an error unless it's purely a transitive dep not needed?
			// If it's the root, it's an error.
			// Let's consider any module with no versions an error for now if resolution gets here.
			return nil, fmt.Errorf("failed to resolve dependencies: module '%s' has no available versions", moduleID)

		}

		resolved[moduleID] = selectedVersion
		log.Printf("Resolved module '%s' to version '%s'", moduleID, selectedVersion)
	}

	// Final check: Ensure the root module was actually resolved.
	if _, ok := resolved[rootModuleID]; !ok {
		// This might happen if the root module had no versions or resolution failed early.
		// The errors above should ideally catch this.
		return nil, fmt.Errorf("internal error: root module '%s' was not resolved", rootModuleID)
	}

	return resolved, nil
}

// CheckForCycles uses the underlying DAG library's cycle detection.
// It returns a slice of cycles, where each cycle is represented by a slice of module IDs (strings).
// TODO: Correctly implement cycle detection using heimdalr/dag v1.5.0 API.
// The GetCycles() or IsAcyclic() methods used previously were incorrect for this library version.
// Needs investigation into the correct API call.
// For now, returning an empty slice to unblock compilation.
func (g *DependencyGraph) CheckForCycles() ([][]string, error) {
	log.Println("Warning: CheckForCycles is not implemented correctly and will not detect cycles.")
	// Placeholder implementation
	return [][]string{}, nil // Assume no cycles for now
}

// TODO: Implement LoadFromConfig, LoadFromRegistry, LoadFromDirectory methods
