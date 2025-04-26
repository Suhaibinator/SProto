### Phase 2: Dependency Resolution System

#### Task 2.1: Create Import Path Parser
- **2.1.1**: Implement Proto file import scanner
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create a new package `internal/proto` for Proto-specific utilities
    - Implement a scanner that extracts import statements from .proto files using `github.com/jhump/protoreflect/desc/protoparse`:
      ```go
      type ImportScanner struct {
          parser *protoparse.Parser
      }
      
      // ScanImports extracts all import statements from a .proto file
      func (s *ImportScanner) ScanImports(protoFilePath string) ([]string, error) {
          // Implementation logic here
      }
      
      // ScanDirectory recursively scans a directory and extracts imports from all .proto files
      func (s *ImportScanner) ScanDirectory(dirPath string) (map[string][]string, error) {
          // Implementation logic here
      }
      ```
    - Handle different Proto syntax versions (proto2, proto3)
    - Implement handling for different import styles:
      - Regular imports: `import "foo/bar.proto";`
      - Public imports: `import public "foo/bar.proto";`
      - Weak imports: `import weak "foo/bar.proto";`
    - Add proper error handling for:
      - Malformed .proto files
      - File access issues
      - Syntax errors
    - Create utility functions for common operations
    - Document performance considerations for large codebases with many .proto files

- **2.1.2**: Create import path normalization logic
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Implement a path normalizer that converts various import path formats to a standard canonical form:
      ```go
      // NormalizeImportPath converts various import path formats to a standard format
      func NormalizeImportPath(importPath string) string {
          // Implementation logic here
      }
      ```
    - Handle the following import path variations:
      - Absolute vs. relative paths
      - Platform-specific path separators (Windows backslash vs. Unix forward slash)
      - Import paths with or without leading slash
      - Import paths with different prefixes (like "google/protobuf/" vs "google.golang.org/protobuf/")
    - Create a mapping system for well-known Proto packages (e.g., mapping "google/protobuf/timestamp.proto" to appropriate import paths)
    - Implement logic to clean up paths:
      - Remove "../" and "./" segments
      - Collapse multiple slashes
      - Ensure consistent use of forward slashes
    - Add validation to ensure the normalized path is valid
    - Create a bidirectional mapping system to convert between:
      - Go import paths (e.g., "github.com/myorg/repo/proto/user/v1/user.proto")
      - Module-relative paths (e.g., "user/v1/user.proto")
    - Document the normalization rules with examples of before/after transformations

- **2.1.3**: Add tests for import path parsing
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create a comprehensive test suite in `internal/proto/scanner_test.go` with test cases for:
      - Valid .proto files with:
        - No imports
        - Single import
        - Multiple imports
        - Public imports
        - Weak imports
        - Mix of import types
      - Different syntax versions (proto2, proto3)
      - Files with comments near import statements
      - Files with syntax errors
      - Invalid or malformed .proto files
    - Add tests for the path normalization logic:
      - Windows-style paths
      - Unix-style paths
      - Relative paths
      - Absolute paths
      - Paths with ".." and "." segments
      - Paths with multiple consecutive slashes
      - Edge cases like empty paths or paths with only slashes
    - Create benchmark tests to measure performance with large sets of .proto files
    - Add integration tests that validate the parser against real-world Proto repositories
    - Create test helpers for generating test .proto files with specific characteristics

#### Task 2.2: Implement Dependency Graph Resolver
- **2.2.1**: Create dependency graph data structure
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create a new package `internal/resolver` for dependency resolution logic
    - Define a dependency graph struct using `github.com/heimdalr/dag`:
      ```go
      type DependencyGraph struct {
          dag *dag.DAG
          // Module metadata map
          modules map[string]ModuleMetadata
          // Version constraints between modules
          constraints map[string]map[string]string
      }
      
      type ModuleMetadata struct {
          Namespace   string
          Name        string
          ImportPath  string
          Versions    []string // Available versions, sorted semantically
      }
      ```
    - Implement methods for building the graph:
      ```go
      // NewDependencyGraph creates a new empty dependency graph
      func NewDependencyGraph() *DependencyGraph
      
      // AddModule adds a module to the graph
      func (g *DependencyGraph) AddModule(id string, metadata ModuleMetadata) error
      
      // AddDependency adds a dependency relationship between modules
      func (g *DependencyGraph) AddDependency(from, to, versionConstraint string) error
      ```
    - Add methods for querying the graph:
      ```go
      // GetModules returns all modules in the graph
      func (g *DependencyGraph) GetModules() []string
      
      // GetDependencies returns direct dependencies of a module
      func (g *DependencyGraph) GetDependencies(moduleID string) ([]string, error)
      
      // GetDependents returns modules that depend on the given module
      func (g *DependencyGraph) GetDependents(moduleID string) ([]string, error)
      ```
    - Implement functionality to load the graph from:
      - A configuration file (sproto.yaml)
      - The registry API
      - A local directory of .proto files
    - Add helper methods for common graph operations
    - Ensure efficient memory usage for large dependency graphs

- **2.2.2**: Implement graph traversal algorithm
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Implement dependency resolution algorithms:
      ```go
      // ResolveVersions finds compatible versions for all modules in the graph
      func (g *DependencyGraph) ResolveVersions(rootModuleID string) (map[string]string, error)
      ```
    - Use topological sort to determine dependency resolution order:
      ```go
      // GetResolutionOrder returns modules in dependency-first order
      func (g *DependencyGraph) GetResolutionOrder() ([]string, error)
      ```
    - Implement version selection logic that:
      - Respects all version constraints
      - Follows semver precedence rules
      - Prefers higher versions when constraints allow
      - Handles complex constraints like ">=v1.2.0 <v2.0.0"
    - Add conflict resolution for competing version requirements:
      - Detect when different modules require incompatible versions of the same dependency
      - Provide detailed error messages explaining the conflict
      - Suggest possible resolutions when feasible
    - Implement efficient caching for previously resolved dependency trees
    - Document performance considerations and optimization strategies
    - Add detailed logging to trace the resolution process

- **2.2.3**: Handle circular dependency detection
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Leverage `github.com/heimdalr/dag`'s built-in cycle detection:
      ```go
      // CheckForCycles detects circular dependencies in the graph
      func (g *DependencyGraph) CheckForCycles() ([][]string, error)
      ```
    - Implement advanced cycle detection that:
      - Identifies all cycles in the dependency graph
      - Reports the exact modules and relationships creating each cycle
      - Provides detailed, user-friendly error messages
    - Create visualization helpers for cycles:
      ```go
      // CycleToString converts a cycle to a readable string representation
      func CycleToString(cycle []string) string
      ```
    - Add suggestions for resolving circular dependencies:
      - Identify which dependency could be moved to break the cycle
      - Suggest restructuring options
    - Implement special handling for "dev dependencies" vs. "runtime dependencies"
    - Add detailed logging during cycle detection
    - Create documentation explaining circular dependency concepts and best practices
    - Document common patterns that lead to circular dependencies and how to avoid them

- **2.2.4**: Write tests for dependency resolution
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create a comprehensive test suite in `internal/resolver/resolver_test.go` with:
      - Simple dependency trees with clear resolution order
      - Complex graphs with many modules and dependencies
      - Graphs with multiple valid resolution orders
      - Cases with version conflicts
      - Edge cases like:
        - Single-node graphs
        - Disconnected graphs
        - Nearly-circular dependencies
      - Real-world inspired dependency patterns
    - Implement test helpers for creating test dependency graphs:
      ```go
      // BuildTestGraph creates a dependency graph from a structured description
      func BuildTestGraph(modules []ModuleSpec, dependencies []DependencySpec) *DependencyGraph
      ```
    - Add specific tests for cycle detection:
      - Simple direct cycles (A → B → A)
      - Complex indirect cycles (A → B → C → D → A)
      - Multiple distinct cycles in the same graph
      - Self-dependencies (A → A)
    - Create tests for version resolution:
      - Compatible version constraints
      - Conflicting version constraints
      - Complex version constraint patterns
    - Add benchmark tests for performance analysis
    - Implement property-based testing for complex resolution logic
    - Create visual output for test failures to help with debugging

#### Task 2.3: Implement Import Path Mapping System
- **2.3.1**: Create import path to module mapper
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create a new package `internal/mapper` for import path mapping logic
    - Define a data structure to store the mapping between import path prefixes and module identifiers:
      ```go
      type ImportMapper struct {
          // Use a prefix tree (Trie) for efficient longest-prefix matching
          prefixTree *trie.Trie 
          // Store module info associated with each prefix
          moduleInfo map[string]ModuleIdentifier 
      }
      
      type ModuleIdentifier struct {
          Namespace string
          Name      string
      }
      ```
    - Implement methods for building the mapping:
      ```go
      // NewImportMapper creates a new empty mapper
      func NewImportMapper() *ImportMapper
      
      // AddMapping registers a mapping from an import path prefix to a module
      func (m *ImportMapper) AddMapping(importPathPrefix string, module ModuleIdentifier) error
      
      // LoadMappingsFromConfig loads mappings from a SProtoConfig struct
      func (m *ImportMapper) LoadMappingsFromConfig(config *config.SProtoConfig) error
      
      // LoadMappingsFromRegistry loads mappings by querying the registry API
      func (m *ImportMapper) LoadMappingsFromRegistry(registryClient *api.Client) error
      ```
    - Add validation to prevent conflicting mappings (e.g., two modules claiming the same import path prefix)
    - Implement logic to handle well-known import paths (e.g., Google Protobuf types)
    - Ensure the mapper can handle a large number of mappings efficiently
    - Add documentation explaining the mapping strategy and potential conflicts

- **2.3.2**: Implement best-match algorithm for import paths
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Implement the core mapping function using the prefix tree:
      ```go
      // ResolveImport finds the module corresponding to a given import path
      // It uses longest-prefix matching to find the most specific mapping
      func (m *ImportMapper) ResolveImport(importPath string) (ModuleIdentifier, bool) {
          // Implementation using the prefix tree
      }
      ```
    - Handle cases where an import path does not match any registered prefix
    - Implement logic to resolve relative import paths based on the context of the importing file
    - Add support for mapping specific files within a module (e.g., `github.com/myorg/common/types.proto` maps to `myorg/common`)
    - Optimize the matching algorithm for performance
    - Create helper functions for common lookup patterns
    - Document the matching logic, especially how conflicts are resolved (longest prefix wins)

- **2.3.3**: Add registry lookup integration
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Integrate the mapper with the registry client (`internal/cli/client.go` or similar)
    - Implement functions to fetch module information (including import paths) from the registry:
      ```go
      // FetchModuleMetadata retrieves module details from the registry
      func (c *RegistryClient) FetchModuleMetadata(namespace, name string) (*models.Module, error)
      
      // FetchAllModules retrieves metadata for all modules
      func (c *RegistryClient) FetchAllModules() ([]models.Module, error)
      ```
    - Update the `LoadMappingsFromRegistry` function in the mapper to use these client methods
    - Implement caching for registry lookups to reduce API calls
    - Handle API errors gracefully (e.g., network issues, module not found)
    - Add authentication handling for registry access
    - Ensure the integration works with both the main registry and potentially local caches
    - Add tests for the registry integration, mocking the API client
