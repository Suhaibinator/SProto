package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfigBytes_Valid(t *testing.T) {
	yamlData := `
version: v1
name: myorg/mymodule
import_path: github.com/myorg/mymodule
dependencies:
  - namespace: myorg
    name: common
    version: ">=v1.0.0 <v2.0.0"
    import_path: github.com/myorg/common
  - namespace: google
    name: protobuf
    version: v1.28.0
    import_path: google/protobuf
generate:
  - name: go
    output: gen/go
    options:
      paths: source_relative
      go_opt: module=github.com/myorg/mymodule/gen/go
    plugins:
      - protoc-gen-go=path/to/go_plugin
  - name: grpc-gateway
    output: gen/gw
    options:
      logtostderr: "true"
      paths: source_relative
`
	config, err := ParseConfigBytes([]byte(yamlData))
	require.NoError(t, err)
	require.NotNil(t, config)

	assert.Equal(t, "v1", config.Version)
	assert.Equal(t, "myorg/mymodule", config.Name)
	assert.Equal(t, "github.com/myorg/mymodule", config.ImportPath)
	require.Len(t, config.Dependencies, 2)
	assert.Equal(t, "myorg", config.Dependencies[0].Namespace)
	assert.Equal(t, "common", config.Dependencies[0].Name)
	assert.Equal(t, ">=v1.0.0 <v2.0.0", config.Dependencies[0].Version)
	assert.Equal(t, "github.com/myorg/common", config.Dependencies[0].ImportPath)
	assert.Equal(t, "google", config.Dependencies[1].Namespace)
	assert.Equal(t, "protobuf", config.Dependencies[1].Name)
	assert.Equal(t, "v1.28.0", config.Dependencies[1].Version)
	assert.Equal(t, "google/protobuf", config.Dependencies[1].ImportPath)

	require.Len(t, config.Generate, 2)
	assert.Equal(t, "go", config.Generate[0].Name)
	assert.Equal(t, "gen/go", config.Generate[0].Output)
	require.Len(t, config.Generate[0].Options, 2)
	assert.Equal(t, "source_relative", config.Generate[0].Options["paths"])
	assert.Equal(t, "module=github.com/myorg/mymodule/gen/go", config.Generate[0].Options["go_opt"])
	require.Len(t, config.Generate[0].Plugins, 1)
	assert.Equal(t, "protoc-gen-go=path/to/go_plugin", config.Generate[0].Plugins[0])

	assert.Equal(t, "grpc-gateway", config.Generate[1].Name)
	assert.Equal(t, "gen/gw", config.Generate[1].Output)
	require.Len(t, config.Generate[1].Options, 2)
	assert.Equal(t, "true", config.Generate[1].Options["logtostderr"])
	assert.Equal(t, "source_relative", config.Generate[1].Options["paths"])
	assert.Empty(t, config.Generate[1].Plugins)
}

func TestParseConfigBytes_InvalidYAML(t *testing.T) {
	yamlData := `
version: v1
name: myorg/mymodule
  import_path: github.com/myorg/mymodule # Bad indentation
`
	_, err := ParseConfigBytes([]byte(yamlData))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal sproto config")
}

func TestParseConfig_FileNotFound(t *testing.T) {
	_, err := ParseConfig("nonexistent_sproto.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read sproto config file")
}

func TestParseConfig_ValidFile(t *testing.T) {
	yamlData := `
version: v1
name: test/module
import_path: example.com/test/module
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sproto.yaml")
	err := os.WriteFile(configPath, []byte(yamlData), 0644)
	require.NoError(t, err)

	config, err := ParseConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, "test/module", config.Name)
	assert.Equal(t, "example.com/test/module", config.ImportPath)
}

func TestSProtoConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    SProtoConfig
		expectErr bool
		errSubstr string
	}{
		{
			name: "Valid config",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule",
				Dependencies: []Dependency{
					{Namespace: "deporg", Name: "common", Version: "v1.0.0", ImportPath: "dep.com/common"},
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid version",
			config: SProtoConfig{
				Version:    "v2",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule",
			},
			expectErr: true,
			errSubstr: "unsupported configuration version",
		},
		{
			name: "Invalid name format (no slash)",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "mymodule",
				ImportPath: "github.com/myorg/mymodule",
			},
			expectErr: true,
			errSubstr: "invalid module name format",
		},
		{
			name: "Invalid name format (special chars)",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/my@module",
				ImportPath: "github.com/myorg/mymodule",
			},
			expectErr: true,
			errSubstr: "invalid module name format",
		},
		{
			name: "Invalid import path format",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/my@module", // Invalid char
			},
			expectErr: true,
			errSubstr: "invalid import path format",
		},
		{
			name: "Invalid dependency namespace",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule",
				Dependencies: []Dependency{
					{Namespace: "dep@org", Name: "common", Version: "v1.0.0", ImportPath: "dep.com/common"},
				},
			},
			expectErr: true,
			errSubstr: "invalid namespace format",
		},
		{
			name: "Invalid dependency name",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule",
				Dependencies: []Dependency{
					{Namespace: "deporg", Name: "com mon", Version: "v1.0.0", ImportPath: "dep.com/common"},
				},
			},
			expectErr: true,
			errSubstr: "invalid name format",
		},
		{
			name: "Invalid dependency version constraint",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule",
				Dependencies: []Dependency{
					{Namespace: "deporg", Name: "common", Version: "invalid-version", ImportPath: "dep.com/common"},
				},
			},
			expectErr: true,
			errSubstr: "invalid version constraint",
		},
		{
			name: "Invalid dependency import path",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule",
				Dependencies: []Dependency{
					{Namespace: "deporg", Name: "common", Version: "v1.0.0", ImportPath: "dep.com/com mon"}, // Space
				},
			},
			expectErr: true,
			errSubstr: "invalid import path format",
		},
		{
			name: "Duplicate dependency",
			config: SProtoConfig{
				Version:    "v1",
				Name:       "myorg/mymodule",
				ImportPath: "github.com/myorg/mymodule",
				Dependencies: []Dependency{
					{Namespace: "deporg", Name: "common", Version: "v1.0.0", ImportPath: "dep.com/common"},
					{Namespace: "deporg", Name: "common", Version: "v1.1.0", ImportPath: "dep.com/common/v1.1"}, // Same ns/name
				},
			},
			expectErr: true,
			errSubstr: "duplicate dependency detected",
		},
		{
			name: "Multiple errors",
			config: SProtoConfig{
				Version:    "v2",    // Error 1
				Name:       "myorg", // Error 2
				ImportPath: "github.com/myorg/mymodule",
				Dependencies: []Dependency{
					{Namespace: "deporg", Name: "common", Version: "invalid", ImportPath: "dep.com/common"}, // Error 3
				},
			},
			expectErr: true,
			errSubstr: "validation failed", // General error message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				// Check that multiple errors are joined
				if tt.name == "Multiple errors" {
					assert.Contains(t, err.Error(), "\n - ")
					assert.Greater(t, len(err.Error()), len(tt.errSubstr)+5) // Ensure more than just the main message
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetDependencyByName(t *testing.T) {
	config := &SProtoConfig{
		Dependencies: []Dependency{
			{Namespace: "org1", Name: "modA", Version: "v1"},
			{Namespace: "org2", Name: "modB", Version: "v2"},
		},
	}

	// Found
	dep, found := config.GetDependencyByName("org1/modA")
	assert.True(t, found)
	require.NotNil(t, dep)
	assert.Equal(t, "org1", dep.Namespace)
	assert.Equal(t, "modA", dep.Name)

	// Not found
	dep, found = config.GetDependencyByName("org1/modC")
	assert.False(t, found)
	assert.Nil(t, dep)

	// Invalid format
	dep, found = config.GetDependencyByName("org1-modA")
	assert.False(t, found)
	assert.Nil(t, dep)
}
