package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDependencyGraph_AddModule(t *testing.T) {
	g := NewDependencyGraph()
	metaA := ModuleMetadata{Namespace: "org", Name: "modA", ImportPath: "path/a"}
	metaB := ModuleMetadata{Namespace: "org", Name: "modB", ImportPath: "path/b"}

	err := g.AddModule(metaA)
	assert.NoError(t, err)
	assert.Contains(t, g.modules, "org/modA")
	assert.Equal(t, metaA, g.modules["org/modA"])

	// Test adding the same module again
	err = g.AddModule(metaA)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	err = g.AddModule(metaB)
	assert.NoError(t, err)
	assert.Contains(t, g.modules, "org/modB")
	assert.Equal(t, metaB, g.modules["org/modB"])

	assert.Len(t, g.modules, 2)
}

func TestDependencyGraph_AddDependency(t *testing.T) {
	g := NewDependencyGraph()
	metaA := ModuleMetadata{Namespace: "org", Name: "modA"}
	metaB := ModuleMetadata{Namespace: "org", Name: "modB"}
	metaC := ModuleMetadata{Namespace: "ext", Name: "modC"}

	require.NoError(t, g.AddModule(metaA))
	require.NoError(t, g.AddModule(metaB))
	require.NoError(t, g.AddModule(metaC))

	// Valid dependency
	err := g.AddDependency("org", "modA", "org", "modB", "v1.0.0")
	assert.NoError(t, err)
	constraint, ok := g.GetConstraint("org/modA", "org/modB")
	assert.True(t, ok)
	assert.Equal(t, "v1.0.0", constraint)

	// Check underlying DAG edge
	children, _ := g.dag.GetChildren("org/modA")
	assert.Contains(t, children, "org/modB")
	parents, _ := g.dag.GetParents("org/modB")
	assert.Contains(t, parents, "org/modA")

	// Dependency to non-existent module
	err = g.AddDependency("org", "modA", "org", "modX", "v1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target module 'org/modX' not found")

	// Dependency from non-existent module
	err = g.AddDependency("org", "modY", "org", "modB", "v1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source module 'org/modY' not found")

	// Duplicate dependency
	err = g.AddDependency("org", "modA", "org", "modB", "v1.1.0") // Different constraint
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	// Verify constraint wasn't updated
	constraint, _ = g.GetConstraint("org/modA", "org/modB")
	assert.Equal(t, "v1.0.0", constraint)
}

func TestDependencyGraph_Getters(t *testing.T) {
	g := NewDependencyGraph()
	metaA := ModuleMetadata{Namespace: "org", Name: "modA"}
	metaB := ModuleMetadata{Namespace: "org", Name: "modB"}
	metaC := ModuleMetadata{Namespace: "ext", Name: "modC"}

	require.NoError(t, g.AddModule(metaA))
	require.NoError(t, g.AddModule(metaB))
	require.NoError(t, g.AddModule(metaC))
	require.NoError(t, g.AddDependency("org", "modA", "org", "modB", "v1"))
	require.NoError(t, g.AddDependency("org", "modA", "ext", "modC", "v2"))
	require.NoError(t, g.AddDependency("org", "modB", "ext", "modC", ">=v2.1"))

	// GetModuleMetadata
	retMetaA, okA := g.GetModuleMetadata("org/modA")
	assert.True(t, okA)
	assert.Equal(t, metaA, retMetaA)
	_, okX := g.GetModuleMetadata("org/modX")
	assert.False(t, okX)

	// GetConstraint
	cAB, okAB := g.GetConstraint("org/modA", "org/modB")
	assert.True(t, okAB)
	assert.Equal(t, "v1", cAB)
	cAC, okAC := g.GetConstraint("org/modA", "ext/modC")
	assert.True(t, okAC)
	assert.Equal(t, "v2", cAC)
	cBC, okBC := g.GetConstraint("org/modB", "ext/modC")
	assert.True(t, okBC)
	assert.Equal(t, ">=v2.1", cBC)
	_, okBA := g.GetConstraint("org/modB", "org/modA") // Reverse
	assert.False(t, okBA)
	_, okXA := g.GetConstraint("org/modX", "org/modA") // Non-existent
	assert.False(t, okXA)

	// GetModules
	allModules := g.GetModules()
	assert.ElementsMatch(t, []string{"org/modA", "org/modB", "ext/modC"}, allModules)

	// GetDependencies
	depsA, errA := g.GetDependencies("org/modA")
	assert.NoError(t, errA)
	assert.ElementsMatch(t, []string{"org/modB", "ext/modC"}, depsA)
	depsB, errB := g.GetDependencies("org/modB")
	assert.NoError(t, errB)
	assert.ElementsMatch(t, []string{"ext/modC"}, depsB)
	depsC, errC := g.GetDependencies("ext/modC")
	assert.NoError(t, errC)
	assert.Empty(t, depsC)
	_, errX := g.GetDependencies("org/modX")
	assert.Error(t, errX)

	// GetDependents
	depentsA, errDepA := g.GetDependents("org/modA")
	assert.NoError(t, errDepA)
	assert.Empty(t, depentsA)
	depentsB, errDepB := g.GetDependents("org/modB")
	assert.NoError(t, errDepB)
	assert.ElementsMatch(t, []string{"org/modA"}, depentsB)
	depentsC, errDepC := g.GetDependents("ext/modC")
	assert.NoError(t, errDepC)
	assert.ElementsMatch(t, []string{"org/modA", "org/modB"}, depentsC)
	_, errDepX := g.GetDependents("org/modX")
	assert.Error(t, errDepX)
}

// TODO: Add tests for GetResolutionOrder once it's correctly implemented.
// func TestDependencyGraph_GetResolutionOrder(t *testing.T) {}

// TODO: Add tests for ResolveVersions once it and GetResolutionOrder are correctly implemented.
// func TestDependencyGraph_ResolveVersions(t *testing.T) {}

// TODO: Add tests for CheckForCycles once it's correctly implemented.
// func TestDependencyGraph_CheckForCycles(t *testing.T) {}
