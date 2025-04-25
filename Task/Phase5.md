### Phase 5: Testing and Documentation

#### Task 5.1: Unit Testing
- **5.1.1**: Write tests for configuration parsing
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create test files (`internal/config/config_test.go`).
    - Test `ParseConfig` and `ParseConfigBytes` with various valid and invalid `sproto.yaml` contents.
    - Cover edge cases: empty file, file not found, invalid YAML syntax, missing required fields, invalid field values (e.g., bad name format), extra unknown fields.
    - Test the `Validate` method thoroughly, ensuring each validation rule (Task 1.1.3) is covered by specific test cases (e.g., test invalid name, test invalid version constraint, test duplicate dependency).
    - Use table-driven tests for clarity and maintainability.
    - Mock filesystem interactions where necessary.

- **5.1.2**: Write tests for dependency resolution
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create test files (`internal/resolver/resolver_test.go`).
    - Test `ResolveVersions` with pre-defined `DependencyGraph` structures (Task 2.2.4).
    - Cover scenarios: no dependencies, simple linear dependencies, diamond dependencies, complex graphs, version conflicts (multiple incompatible constraints), successful resolution with constraints.
    - Test `GetResolutionOrder` for correctness based on graph structure.
    - Test `CheckForCycles` with graphs containing various cycle patterns (direct, indirect, multiple).
    - Mock registry/API interactions if the resolver directly calls them (though ideally, graph building is separate).
    - Use test helpers (Task 2.2.4) to construct graphs easily.

- **5.1.3**: Write tests for import path mapping
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create test files (`internal/mapper/mapper_test.go`).
    - Test `AddMapping` and `ResolveImport` using the chosen prefix tree implementation.
    - Cover scenarios: exact match, longest-prefix match, no match, mapping root paths, mapping sub-paths.
    - Test `LoadMappingsFromConfig` and `LoadMappingsFromRegistry` (mocking registry client).
    - Test validation for conflicting mappings.
    - Test handling of well-known import paths.
    - Test relative path resolution logic if implemented.

#### Task 5.2: Integration Testing

##### Task 5.2.1: End-to-End Test for Publish Workflow
- **5.2.1.1**: Set up Docker Compose test environment
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a dedicated docker-compose.yaml for testing
    - Configure registry, PostgreSQL, and MinIO containers
    - Set up test-specific environment variables and volumes
    - Create a helper script to start/stop the environment

- **5.2.1.2**: Create test proto modules
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create sample proto files with imports and dependencies
    - Create sproto.yaml files with various configurations
    - Organize modules in a test-friendly directory structure
    - Include both valid and invalid test cases

- **5.2.1.3**: Write basic publish test script
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a Go test file for testing publish functionality
    - Implement test for publishing a simple module without dependencies
    - Verify success by checking API and storage
    - Include proper setup and teardown logic

- **5.2.1.4**: Implement tests for publish with dependencies
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Add tests for publishing modules with dependencies
    - Verify correct dependency metadata is stored
    - Test publishing with valid dependency declarations
    - Verify import path mappings are correctly stored

- **5.2.1.5**: Add tests for error cases
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Test publishing with invalid dependency declarations
    - Test publishing with missing dependencies
    - Test publishing with version conflicts
    - Verify appropriate error messages are returned

##### Task 5.2.2: End-to-End Test for Resolve Workflow
- **5.2.2.1**: Prepare test modules in registry
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a script to pre-populate the registry with test modules
    - Set up a dependency graph with multiple levels
    - Include modules with version constraints and import paths
    - Design the test dataset to cover key test scenarios

- **5.2.2.2**: Create test project with dependencies
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a test project with sproto.yaml declaring dependencies
    - Include direct and transitive dependency scenarios
    - Set up version constraints for testing resolution logic
    - Prepare proto files with imports from dependencies

- **5.2.2.3**: Write basic resolve test
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Implement test for basic dependency resolution
    - Verify correct modules are downloaded to cache
    - Check correct versions are selected based on constraints
    - Verify directory structure in cache is correct

- **5.2.2.4**: Implement fetch with dependencies test
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Test protoreg-cli fetch --with-deps functionality
    - Verify all dependencies are correctly downloaded
    - Test output directory structure with dependencies
    - Compare with fetch without dependencies

- **5.2.2.5**: Test cache behavior and flags
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Test cache hits by running resolve multiple times
    - Verify --update flag fetches latest versions
    - Test --no-cache forces re-fetching modules
    - Validate cache entries and timestamps

##### Task 5.2.3: Backward Compatibility Testing
- **5.2.3.1**: Test old CLI with new server
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Build old CLI version before dependency features
    - Test basic commands against new server version
    - Verify publish, fetch, and list still work
    - Document any compatibility issues

- **5.2.3.2**: Test new CLI with backward compatibility mode
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Test new CLI against a server without dependency features
    - Verify graceful fallback for dependency-related operations
    - Test publish without dependency features
    - Test fetch without dependency resolution

- **5.2.3.3**: Verify database migration with existing data
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a database with old schema and sample data
    - Run migrations to upgrade to new schema
    - Verify existing data integrity
    - Test operations on modules published before migration

#### Task 5.3: Documentation

##### Task 5.3.1: Update README.md
- **5.3.1.1**: Add dependency features to main README features list
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Update the main feature list at the top of the README
    - Add entries for dependency management, import path mapping, and caching
    - Ensure consistency with existing feature descriptions

- **5.3.1.2**: Create dependency management section
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Add a new "Dependency Management" section after "Features"
    - Explain key concepts and capabilities
    - Include a basic sproto.yaml example
    - Highlight advantages over manual dependency management

- **5.3.1.3**: Update architecture diagram
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Add dependency resolution flow to the existing diagram
    - Show local cache interactions
    - Include dependency resolution steps
    - Update the mermaid code and ensure it renders correctly

- **5.3.1.4**: Add CLI dependency commands documentation
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Expand CLI Usage section with new commands
    - Document resolve, fetch --with-deps, compile, and cache commands
    - Include command examples with common options
    - Show example output for key commands

- **5.3.1.5**: Add documentation links section
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create section for additional documentation
    - Add links to all new documentation files
    - Provide brief descriptions of each linked document
    - Ensure paths are correct relative to repository structure

##### Task 5.3.2: Migrating from Buf Guide
- **5.3.2.1**: Create comparison table
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create docs/migrating-from-buf.md file
    - Build comparison table of Buf vs SProto concepts
    - Include commands, file formats, and terminology
    - Add detailed notes column for important differences

- **5.3.2.2**: Document configuration conversion
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Document buf.yaml to sproto.yaml conversion process
    - Provide field-by-field mapping instructions
    - Include example of before/after files
    - Note any fields without direct equivalents

- **5.3.2.3**: Document workflow differences
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Compare common workflows between Buf and SProto
    - Include command examples for each platform
    - Detail how tasks map between the two systems
    - Highlight areas where SProto differs significantly

- **5.3.2.4**: Create migration checklist
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Develop step-by-step migration guide
    - Address potential pain points and solutions
    - Include sections for different types of Buf usage
    - Add troubleshooting section for common issues

##### Task 5.3.3: Configuration Format Documentation
- **5.3.3.1**: Document basic schema
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create docs/sproto-yaml-spec.md file
    - Document file purpose and location
    - Create formal specification table of all fields
    - Include required/optional status for each field

- **5.3.3.2**: Document version constraints
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Provide detailed explanation of version constraint syntax
    - Document all supported operators (exact, ranges, etc.)
    - Include examples of combining constraints
    - Explain semantics of different constraints

- **5.3.3.3**: Create configuration examples
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create examples for different use cases
    - Include basic module, single dependency, multiple dependencies
    - Show advanced configuration options
    - Add comments explaining key aspects of each example

- **5.3.3.4**: Document validation rules
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Document all validation rules for sproto.yaml
    - Include format requirements for each field
    - Document rules for valid dependencies
    - Add common validation error messages and how to fix them

##### Task 5.3.4: Usage Examples Documentation
- **5.3.4.1**: Basic module publishing example
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create docs/usage-examples.md file
    - Document step-by-step process of creating and publishing a module
    - Include proto file and sproto.yaml examples
    - Show complete command examples with expected output

- **5.3.4.2**: Module with dependencies example
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Document creating a module with dependencies
    - Show how to reference types from dependencies
    - Include complete proto files and sproto.yaml
    - Explain key concepts related to dependencies

- **5.3.4.3**: Dependency resolution workflow
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Document using resolve command
    - Explain resolution process and output
    - Show how to use --update and other flags
    - Include examples of resolving with different constraints

- **5.3.4.4**: Compile workflow with dependencies
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Document using compile command with dependencies
    - Show how compile automatically handles include paths
    - Include examples for different output formats
    - Explain integration with protoc plugins

- **5.3.4.5**: Cache management examples
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Document cache commands and operations
    - Show when and how to use cache clean/invalidate
    - Include example output of cache operations
    - Explain cache location and structure
