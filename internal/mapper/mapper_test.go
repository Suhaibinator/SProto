package mapper

import (
	"errors"
	"testing"

	"github.com/Suhaibinator/SProto/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRegistryClient is a mock implementation of the RegistryClient interface.
type MockRegistryClient struct {
	mock.Mock
}

func (m *MockRegistryClient) FetchAllModules() ([]api.ModuleInfo, error) {
	args := m.Called()
	return args.Get(0).([]api.ModuleInfo), args.Error(1)
}

func TestImportMapper_LoadMappingsFromRegistry(t *testing.T) {
	mapper := NewImportMapper()
	mockClient := new(MockRegistryClient)

	// Test case 1: Successful loading with modules
	mockModules := []api.ModuleInfo{
		{Namespace: "org1", Name: "modA", ImportPath: stringPtr("github.com/org1/modA/proto"), LatestVersion: "v1.0.0"},
		{Namespace: "org2", Name: "modB", ImportPath: stringPtr("example.com/org2/modB"), LatestVersion: "v2.1.0"},
		{Namespace: "org3", Name: "modC", ImportPath: nil, LatestVersion: "v0.5.0"},           // Module without import path
		{Namespace: "org4", Name: "modD", ImportPath: stringPtr(""), LatestVersion: "v1.0.0"}, // Module with empty import path
	}
	mockClient.On("FetchAllModules").Return(mockModules, nil).Once()

	err := mapper.LoadMappingsFromRegistry(mockClient)
	assert.NoError(t, err)

	// Verify mappings were added
	modA, okA := mapper.ResolveImport("github.com/org1/modA/proto/file.proto")
	assert.True(t, okA)
	assert.Equal(t, "org1", modA.Namespace)
	assert.Equal(t, "modA", modA.Name)

	modB, okB := mapper.ResolveImport("example.com/org2/modB/types.proto")
	assert.True(t, okB)
	assert.Equal(t, "org2", modB.Namespace)
	assert.Equal(t, "modB", modB.Name)

	// Modules without import paths should not be mapped
	_, okC := mapper.ResolveImport("org3/modC/file.proto")
	assert.False(t, okC)
	_, okD := mapper.ResolveImport("org4/modD/file.proto")
	assert.False(t, okD)

	// Verify prefixes are sorted correctly (longest first)
	assert.Equal(t, []string{"github.com/org1/modA/proto", "example.com/org2/modB"}, mapper.prefixes)

	mockClient.AssertExpectations(t)

	// Test case 2: Error fetching modules
	mapper = NewImportMapper() // Reset mapper
	fetchError := errors.New("network error")
	mockClient.On("FetchAllModules").Return([]api.ModuleInfo{}, fetchError).Once()

	err = mapper.LoadMappingsFromRegistry(mockClient)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch modules from registry")
	assert.Empty(t, mapper.mappings)
	assert.Empty(t, mapper.prefixes)

	mockClient.AssertExpectations(t)

	// Test case 3: Conflicting mappings (should fail fast)
	mapper = NewImportMapper() // Reset mapper
	conflictingModules := []api.ModuleInfo{
		{Namespace: "org1", Name: "modA", ImportPath: stringPtr("github.com/common/proto"), LatestVersion: "v1.0.0"},
		{Namespace: "org2", Name: "modB", ImportPath: stringPtr("github.com/common/proto"), LatestVersion: "v2.0.0"}, // Conflict
	}
	mockClient.On("FetchAllModules").Return(conflictingModules, nil).Once()

	err = mapper.LoadMappingsFromRegistry(mockClient)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting mapping for prefix github.com/common/proto")
	// Mapper state is undefined after error, but should ideally not have added the conflicting one.
	// Depending on AddMapping's behavior on error, the state might vary.
	// Let's just check that an error was returned.

	mockClient.AssertExpectations(t)
}

// Helper to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}

// TODO: Add tests for ResolveImport edge cases (Task 2.3.2)
// TODO: Add tests for handling well-known import paths (Task 2.3.1)
// TODO: Add tests for bidirectional mapping (Task 2.1.2 details, but fits here)
