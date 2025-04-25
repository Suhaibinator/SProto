package resolver

import (
	"fmt"
	"testing"

	"github.com/Suhaibinator/SProto/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockRegistryClient provides a mock implementation of the client interface.
type MockRegistryClient struct {
	Modules      map[string]*api.ModuleInfo
	Versions     map[string][]string
	Dependencies map[string][]api.DependencyResponse
}

func (m *MockRegistryClient) FetchModuleMetadata(namespace, name string) (*api.ModuleInfo, error) {
	id := fmt.Sprintf("%s/%s", namespace, name)
	if info, ok := m.Modules[id]; ok {
		return info, nil
	}
	return nil, fmt.Errorf("module not found: %s", id)
}

func (m *MockRegistryClient) FetchModuleVersions(namespace, name string) ([]string, error) {
	id := fmt.Sprintf("%s/%s", namespace, name)
	if versions, ok := m.Versions[id]; ok {
		return versions, nil
	}
	return nil, fmt.Errorf("versions not found for module: %s", id)
}

func (m *MockRegistryClient) FetchModuleDependencies(namespace, name string) ([]api.DependencyResponse, error) {
	id := fmt.Sprintf("%s/%s", namespace, name)
	if deps, ok := m.Dependencies[id]; ok {
		return deps, nil
	}
	// Return empty slice if no dependencies defined, not an error
	return []api.DependencyResponse{}, nil
}

func setupMockClient() *MockRegistryClient {
	return &MockRegistryClient{
		Modules: map[string]*api.ModuleInfo{
			"myorg/app":    {Namespace: "myorg", Name: "app", ImportPath: strPtr("github.com/myorg/app")},
			"myorg/libA":   {Namespace: "myorg", Name: "libA", ImportPath: strPtr("github.com/myorg/libA")},
			"myorg/libB":   {Namespace: "myorg", Name: "libB", ImportPath: strPtr("github.com/myorg/libB")},
			"myorg/common": {Namespace: "myorg", Name: "common", ImportPath: strPtr("github.com/myorg/common")},
			"ext/utils":    {Namespace: "ext", Name: "utils", ImportPath: strPtr("thirdparty.com/utils")},
		},
		Versions: map[string][]string{
			"myorg/app":    {"v1.0.0", "v1.1.0"},
			"myorg/libA":   {"v1.0.0", "v1.0.1", "v1.1.0"},
			"myorg/libB":   {"v0.9.0", "v1.0.0"},
			"myorg/common": {"v1.0.0", "v1.1.0", "v2.0.0"},
			"ext/utils":    {"v1.0.0"},
		},
		Dependencies: map[string][]api.DependencyResponse{
			"myorg/app": {
				{Namespace: "myorg", Name: "libA", VersionConstraint: "^v1.0.0"}, // >=v1.0.0 <v2.0.0
				{Namespace: "myorg", Name: "libB", VersionConstraint: "v1.0.0"},
			},
			"myorg/libA": {
				{Namespace: "myorg", Name: "common", VersionConstraint: "~v1.0.0"}, // >=v1.0.0 <v1.1.0 -> should resolve to v1.0.0
			},
			"myorg/libB": {
				{Namespace: "myorg", Name: "common", VersionConstraint: ">=v1.1.0 <v2.0.0"}, // -> should resolve to v1.1.0
				{Namespace: "ext", Name: "utils", VersionConstraint: "v1.0.0"},
			},
			// common and utils have no dependencies
		},
	}
}

func strPtr(s string) *string { return &s }

func TestDependencyResolver_ResolveRootModule_Simple(t *testing.T) {
	client := setupMockClient()
	logger := zap.NewNop()
	resolver := NewDependencyResolver(client, logger, nil) // Pass nil for progress

	resolved, err := resolver.ResolveRootModule("myorg", "libA", "v1.0.1") // libA depends on common ~v1.0.0
	require.NoError(t, err)
	require.NotNil(t, resolved)

	assert.Len(t, resolved, 2)
	assert.Equal(t, "v1.0.1", resolved["myorg/libA"])
	assert.Equal(t, "v1.0.0", resolved["myorg/common"]) // ~v1.0.0 resolves to v1.0.0
}

func TestDependencyResolver_ResolveRootModule_Diamond(t *testing.T) {
	client := setupMockClient()
	logger := zap.NewNop()
	resolver := NewDependencyResolver(client, logger, nil)

	// app -> libA (^v1.0.0 -> resolves v1.1.0) -> common (~v1.0.0 -> resolves v1.0.0)
	// app -> libB (v1.0.0) -> common (>=v1.1.0 <v2.0.0 -> resolves v1.1.0)
	// app -> libB (v1.0.0) -> utils (v1.0.0)
	// Conflict on common: libA wants v1.0.0, libB wants v1.1.0. Resolution should pick highest compatible = v1.1.0
	resolved, err := resolver.ResolveRootModule("myorg", "app", "v1.1.0")
	require.NoError(t, err)
	require.NotNil(t, resolved)

	// Expected: app, libA, libB, common, utils
	assert.Len(t, resolved, 5)
	assert.Equal(t, "v1.1.0", resolved["myorg/app"])
	assert.Equal(t, "v1.1.0", resolved["myorg/libA"])   // ^v1.0.0 resolves to latest v1.x.x = v1.1.0
	assert.Equal(t, "v1.0.0", resolved["myorg/libB"])   // Exact v1.0.0
	assert.Equal(t, "v1.1.0", resolved["myorg/common"]) // Highest compatible version (v1.1.0 wins over v1.0.0)
	assert.Equal(t, "v1.0.0", resolved["ext/utils"])
}

func TestDependencyResolver_ResolveRootModule_Conflict(t *testing.T) {
	client := setupMockClient()
	// Modify libA dependency to cause conflict
	client.Dependencies["myorg/libA"] = []api.DependencyResponse{
		{Namespace: "myorg", Name: "common", VersionConstraint: "v1.0.0"}, // Exact v1.0.0
	}
	// libB still depends on common >=v1.1.0 <v2.0.0

	logger := zap.NewNop()
	resolver := NewDependencyResolver(client, logger, nil)

	_, err := resolver.ResolveRootModule("myorg", "app", "v1.1.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot resolve dependencies")
	assert.Contains(t, err.Error(), "myorg/common") // Should mention the conflicting module
}

func TestDependencyResolver_ResolveRootModule_ModuleNotFound(t *testing.T) {
	client := setupMockClient()
	logger := zap.NewNop()
	resolver := NewDependencyResolver(client, logger, nil)

	_, err := resolver.ResolveRootModule("nonexistent", "module", "v1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch metadata")
	assert.Contains(t, err.Error(), "nonexistent/module")
}

func TestDependencyResolver_ResolveRootModule_DependencyNotFound(t *testing.T) {
	client := setupMockClient()
	// Add dependency on a module that doesn't exist in the mock client
	client.Dependencies["myorg/app"] = append(client.Dependencies["myorg/app"],
		api.DependencyResponse{Namespace: "myorg", Name: "nonexistent", VersionConstraint: "v1.0.0"},
	)

	logger := zap.NewNop()
	resolver := NewDependencyResolver(client, logger, nil)

	_, err := resolver.ResolveRootModule("myorg", "app", "v1.1.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed processing dependency")
	assert.Contains(t, err.Error(), "myorg/nonexistent")
	assert.Contains(t, err.Error(), "failed to fetch metadata") // The underlying error
}

func TestDependencyResolver_ResolveRootModule_Cycle(t *testing.T) {
	client := setupMockClient()
	// Create a cycle: libA -> libB -> libA
	client.Dependencies["myorg/libA"] = []api.DependencyResponse{
		{Namespace: "myorg", Name: "libB", VersionConstraint: "v1.0.0"},
	}
	client.Dependencies["myorg/libB"] = []api.DependencyResponse{
		{Namespace: "myorg", Name: "libA", VersionConstraint: "v1.0.0"},
	}

	logger := zap.NewNop()
	resolver := NewDependencyResolver(client, logger, nil)

	_, err := resolver.ResolveRootModule("myorg", "libA", "v1.0.0")
	require.Error(t, err)
	// The underlying DAG library should detect the cycle when adding edges
	assert.Contains(t, err.Error(), "failed to add dependency edge")
	assert.Contains(t, err.Error(), "would create a cycle")
}
