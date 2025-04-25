package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSProtoConfig_Validate_Valid(t *testing.T) {
	tests := []struct {
		name   string
		config SProtoConfig
	}{
		{
			name: "Valid Basic Config",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule/proto",
			},
		},
		{
			name: "Valid Config with Dependencies",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/anothermodule",
				ImportPath: "github.com/myorg/anothermodule/proto",
				Dependencies: []Dependency{
					{
						Namespace:  "myorg",
						Name:       "common",
						Version:    "v1.0.0",
						ImportPath: "github.com/myorg/common/proto",
					},
					{
						Namespace:  "ext",
						Name:       "public",
						Version:    ">=v2.0.0, <v3.0.0",
						ImportPath: "github.com/ext/public/proto/v2",
					},
				},
			},
		},
		{
			name: "Valid Config with Complex Version Constraint",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/complex",
				ImportPath: "example.com/myorg/complex",
				Dependencies: []Dependency{
					{
						Namespace:  "stable",
						Name:       "api",
						Version:    "~v1.2.3", // >= 1.2.3, < 1.3.0
						ImportPath: "example.com/stable/api",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestSProtoConfig_Validate_Invalid(t *testing.T) {
	tests := []struct {
		name          string
		config        SProtoConfig
		expectedError string // Substring expected in the error message
	}{
		{
			name: "Invalid Version",
			config: SProtoConfig{
				Version:    "v2",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule/proto",
			},
			expectedError: "unsupported configuration version 'v2'",
		},
		{
			name: "Invalid Name Format - Missing Slash",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg-mymodule",
				ImportPath: "github.com/myorg/mymodule/proto",
			},
			expectedError: "invalid module name format 'myorg-mymodule'",
		},
		{
			name: "Invalid Name Format - Invalid Chars",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/my!module",
				ImportPath: "github.com/myorg/mymodule/proto",
			},
			expectedError: "invalid module name format 'myorg/my!module'",
		},
		{
			name: "Invalid Import Path",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/my org/mymodule", // Space is invalid
			},
			expectedError: "invalid import path format 'github.com/my org/mymodule'",
		},
		{
			name: "Invalid Dependency Namespace",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule/proto",
				Dependencies: []Dependency{
					{Namespace: "my org", Name: "common", Version: "v1", ImportPath: "path"},
				},
			},
			expectedError: "dependency #1 (my org/common): invalid namespace format 'my org'",
		},
		{
			name: "Invalid Dependency Name",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule/proto",
				Dependencies: []Dependency{
					{Namespace: "myorg", Name: "common!", Version: "v1", ImportPath: "path"},
				},
			},
			expectedError: "dependency #1 (myorg/common!): invalid name format 'common!'",
		},
		{
			name: "Invalid Dependency Version Constraint",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule/proto",
				Dependencies: []Dependency{
					{Namespace: "myorg", Name: "common", Version: "invalid-version", ImportPath: "path"},
				},
			},
			expectedError: "dependency #1 (myorg/common): invalid version constraint 'invalid-version'",
		},
		{
			name: "Invalid Dependency Import Path",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule/proto",
				Dependencies: []Dependency{
					{Namespace: "myorg", Name: "common", Version: "v1", ImportPath: "invalid path"},
				},
			},
			expectedError: "dependency #1 (myorg/common): invalid import path format 'invalid path'",
		},
		{
			name: "Duplicate Dependency",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule/proto",
				Dependencies: []Dependency{
					{Namespace: "myorg", Name: "common", Version: "v1", ImportPath: "path1"},
					{Namespace: "myorg", Name: "common", Version: "v2", ImportPath: "path2"},
				},
			},
			expectedError: "duplicate dependency detected: 'myorg/common'",
		},
		{
			name: "Multiple Errors",
			config: SProtoConfig{
				Version:    "v0",             // Invalid version
				Name:       "myorg-mymodule", // Invalid name
				ImportPath: "github.com/myorg/mymodule/proto",
				Dependencies: []Dependency{
					{Namespace: "myorg", Name: "common", Version: "invalid", ImportPath: "path"},
				},
			},
			expectedError: "unsupported configuration version 'v0'", // Check if multiple errors are reported
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.expectedError), "Error message mismatch.\nExpected to contain: %s\nActual error: %s", tt.expectedError, err.Error())

			// Specific check for multiple errors case
			if tt.name == "Multiple Errors" {
				assert.True(t, strings.Contains(err.Error(), "invalid module name format 'myorg-mymodule'"), "Missing name format error")
				assert.True(t, strings.Contains(err.Error(), "invalid version constraint 'invalid'"), "Missing version constraint error")
			}
		})
	}
}

func TestParseConfigBytes_Valid(t *testing.T) {
	yamlData := `
version: v1
name: myorg/testmodule
import_path: github.com/myorg/testmodule/proto
dependencies:
  - namespace: deporg
    name: depmod
    version: ">=v1.1.0"
    import_path: github.com/deporg/depmod/proto
`
	config, err := ParseConfigBytes([]byte(yamlData))
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "v1", config.Version)
	assert.Equal(t, "myorg/testmodule", config.Name)
	assert.Equal(t, "github.com/myorg/testmodule/proto", config.ImportPath)
	assert.Len(t, config.Dependencies, 1)
	assert.Equal(t, "deporg", config.Dependencies[0].Namespace)
	assert.Equal(t, "depmod", config.Dependencies[0].Name)
	assert.Equal(t, ">=v1.1.0", config.Dependencies[0].Version)
	assert.Equal(t, "github.com/deporg/depmod/proto", config.Dependencies[0].ImportPath)
}

func TestParseConfigBytes_InvalidYAML(t *testing.T) {
	yamlData := `
version: v1
name: myorg/testmodule
  import_path: github.com/myorg/testmodule/proto # Invalid indentation
`
	_, err := ParseConfigBytes([]byte(yamlData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal sproto config")
}

func TestParseConfigBytes_InvalidContent(t *testing.T) {
	yamlData := `
version: v2 # Invalid version
name: myorg/testmodule
import_path: github.com/myorg/testmodule/proto
`
	_, err := ParseConfigBytes([]byte(yamlData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sproto config")
	assert.Contains(t, err.Error(), "unsupported configuration version 'v2'")
}

func TestSProtoConfig_GetDependencyByName(t *testing.T) {
	config := SProtoConfig{
		Dependencies: []Dependency{
			{Namespace: "org1", Name: "modA", Version: "v1", ImportPath: "pathA"},
			{Namespace: "org2", Name: "modB", Version: "v2", ImportPath: "pathB"},
		},
	}

	// Found
	dep, found := config.GetDependencyByName("org1/modA")
	assert.True(t, found)
	assert.NotNil(t, dep)
	assert.Equal(t, "org1", dep.Namespace)
	assert.Equal(t, "modA", dep.Name)

	// Not Found
	dep, found = config.GetDependencyByName("org1/modC")
	assert.False(t, found)
	assert.Nil(t, dep)

	// Invalid Format
	dep, found = config.GetDependencyByName("org1-modA")
	assert.False(t, found)
	assert.Nil(t, dep)
}
