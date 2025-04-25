### Phase 5: Testing and Documentation

#### Task 5.1: Unit Testing
- **5.1.1**: Write tests for configuration parsing
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create test files (`internal/config/config_test.go`).
    - Test `ParseConfig` and `ParseConfigBytes` with various valid and invalid `sproto.yaml` contents.
    - Cover edge cases: empty file, file not found, invalid YAML syntax, missing required fields, invalid field values (e.g., bad name format), extra unknown fields.
    - Test the `Validate` method thoroughly, ensuring each validation rule (Task 1.1.3) is covered by specific test cases (e.g., test invalid name, test invalid version constraint, test duplicate dependency).
    - Use table-driven tests for clarity and maintainability.
    - Mock filesystem interactions where necessary.

- **5.1.2**: Write tests for dependency resolution
  - **Assignee**: Cline
  - **Status**: TODO
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
  - **Status**: TODO
  - **Details**: 
    - Create test files (`internal/mapper/mapper_test.go`).
    - Test `AddMapping` and `ResolveImport` using the chosen prefix tree implementation.
    - Cover scenarios: exact match, longest-prefix match, no match, mapping root paths, mapping sub-paths.
    - Test `LoadMappingsFromConfig` and `LoadMappingsFromRegistry` (mocking registry client).
    - Test validation for conflicting mappings.
    - Test handling of well-known import paths.
    - Test relative path resolution logic if implemented.

#### Task 5.2: Integration Testing
- **5.2.1**: Create end-to-end test for publish workflow
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Set up a test environment using `docker-compose` with registry, DB, and storage.
    - Create test proto modules (directories with `.proto` files and `sproto.yaml`).
    - Write test scripts (e.g., Go tests using `os/exec` or shell scripts) that:
      - Run `protoreg-cli publish` on test modules.
      - Verify the command succeeds.
      - Query the registry API (`/api/v1/modules/...`) to confirm the module, version, import path, and dependencies were stored correctly in the database.
      - Check the artifact storage (MinIO/local) to ensure the zip file was uploaded.
      - Test publishing with and without `sproto.yaml`.
      - Test publishing with valid and invalid dependency declarations (expecting failures for invalid ones).

- **5.2.2**: Create end-to-end test for resolve workflow
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Use the test environment from 5.2.1, potentially pre-populated with test modules.
    - Create a test project with a `sproto.yaml` declaring dependencies on the pre-published modules.
    - Write test scripts that:
      - Run `protoreg-cli resolve`.
      - Verify the command succeeds and reports correct resolution.
      - Check the local cache (`~/.cache/sproto`) to ensure all required module artifacts (including transitive dependencies) were downloaded and potentially extracted correctly.
      - Test `protoreg-cli fetch --with-deps` similarly.
      - Test scenarios with version conflicts (expecting failures).
      - Test cache hits by running resolve/fetch multiple times.
      - Test `--update` and `--no-cache` flags.

- **5.2.3**: Test backward compatibility
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create test scenarios using the *old* CLI version (before these changes) against the *new* server version, and vice-versa.
    - Verify that existing commands (`publish` without `sproto.yaml`, `fetch` without `--with-deps`, `list`) continue to function as expected.
    - Publish a module using the old CLI and ensure it can be fetched/listed by the new CLI/API.
    - Publish a module using the new CLI (without dependency features) and ensure it can be fetched/listed by the old CLI.
    - Ensure database migrations (Task 1.2.3) handle existing data correctly without loss.

#### Task 5.3: Documentation
- **5.3.1**: Update README.md with new features
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Add a new "Dependency Management" section to `README.md` right after the existing "Features" section:
      ```markdown
      ## Dependency Management
      
      SProto now supports Buf-like dependency management for Protobuf files, allowing you to:
      
      * Declare dependencies in a `sproto.yaml` configuration file
      * Use import paths like `import "github.com/myorg/common/proto/types.proto"`
      * Automatically fetch and cache dependencies
      * Generate correct `--proto_path` arguments for protoc
      ```
    - Explain the concept of `sproto.yaml` with a clear example:
      ```markdown
      ### Configuration File Format
      
      Dependencies are managed through a `sproto.yaml` file in the root of your proto directory:
      
      ```yaml
      version: v1
      name: mycompany/myapp
      import_path: github.com/mycompany/myapp
      dependencies:
        - namespace: mycompany
          name: common
          version: ">=v1.0.0 <v2.0.0"
          import_path: github.com/mycompany/common
      ```
      ```
    - Update the Architecture diagram to include the dependency resolution flow:
      ```mermaid
      graph LR
        Dev["Developer Machine
      (Proto Files)"] --> CLI["Registry Client
      protoreg-cli"];
        CLI --> Server["Registry Server
      Go App in Docker"];
        Server --> DB[("PostgreSQL
      Metadata")];
        Server --> S3[("MinIO / S3
      Artifacts")];
        CLI -- "1. Resolve deps" --> CLI;
        CLI -- "2. Pull artifacts" --> CLI;
        CLI -- "3. Cache locally" --> Cache[("Local Cache
      ~/.cache/sproto")];
      ```
    - Expand the CLI Usage section with the new commands:
      ```markdown
      ### Dependency Resolution Commands
      
      1. **`resolve`**: Resolves and fetches all dependencies for a module.
         ```bash
         # Resolve dependencies for the current directory (using sproto.yaml)
         protoreg-cli resolve
         
         # Resolve dependencies for a specific module version
         protoreg-cli resolve mycompany/common@v1.0.0
         
         # Force re-fetching even if cached
         protoreg-cli resolve --update
         ```
      
      2. **`fetch`** (with enhanced dependency options):
         ```bash
         # Fetch a module and all its dependencies
         protoreg-cli fetch mycompany/auth v1.0.0 --output ./protos --with-deps
         ```
      
      3. **`compile`**: Simplifies running protoc with the correct include paths:
         ```bash
         # Compile with resolved dependencies
         protoreg-cli compile --go_out=./gen
         ```
      
      4. **`cache`**: Manages the local module cache:
         ```bash
         # List cached modules
         protoreg-cli cache list
         
         # Clean the cache
         protoreg-cli cache clean
         ```
      ```
    - Update the main feature list at the top of the README to include:
      ```markdown
      *   **Dependency Management:** Declare, resolve, and fetch module dependencies automatically.
      *   **Import Path Mapping:** Use logical import paths in your .proto files that map to registry modules.
      *   **Local Caching:** Store and reuse downloaded dependencies to improve build performance.
      ```
    - Add a new section linking to detailed documentation:
      ```markdown
      ### Dependency Management Documentation
      
      * [Configuration File Format](docs/sproto-yaml-spec.md) - Full specification for sproto.yaml
      * [Usage Examples](docs/usage-examples.md) - Examples of common workflows
      * [Migrating from Buf](docs/migrating-from-buf.md) - Guide for existing Buf users
      ```

- **5.3.2**: Create guide for migrating from Buf
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a new documentation file (e.g., `docs/migrating-from-buf.md`).
    - Provide a comparison table of Buf concepts/commands vs. SProto equivalents:
      ```markdown
      | Buf Concept/Command | SProto Equivalent | Notes |
      | ------------------ | ----------------- | ----- |
      | `buf.yaml` | `sproto.yaml` | Similar structure, different field names |
      | `buf.build/org/repo` | Registry URL + namespace/name | SProto uses `namespace/name` conventions |
      | `buf push` | `protoreg-cli publish` | Similar workflow, different options |
      | `buf build` | `protoreg-cli resolve` | SProto separates resolution from compilation |
      | `buf generate` | `sproto compile` | SProto focuses on proto_path generation |
      | `buf mod update` | `protoreg-cli resolve --update` | Similar functionality |
      | `buf mod init` | Manual creation of `sproto.yaml` | No direct init command yet |
      ```
    - Explain how to convert a `buf.yaml` to `sproto.yaml`:
      - Map `name` field in buf.yaml to `import_path` in sproto.yaml
      - Convert `deps` array to `dependencies` array with explicit namespace/name
      - Transform version requirements to semver constraints
      - Example conversion code in the guide
    - Detail differences in workflow with code examples:
      - Buf: `buf push` → SProto: `protoreg-cli publish ./protos --module myorg/mymodule`
      - Buf: `buf build` → SProto: `protoreg-cli resolve && protoreg-cli compile`
      - Buf: `buf generate` → SProto: `protoreg-cli compile --go_out=./gen/go`
    - Address potential pain points:
      - BSR's hosted service vs. SProto's self-hosted approach
      - Authentication differences
      - Migration path for existing modules
      - Performance considerations for large repositories
    - Provide a step-by-step migration checklist for teams
    - Include success stories/case studies if available

- **5.3.3**: Document configuration file format
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a dedicated documentation file (e.g., `docs/sproto-yaml-spec.md`) with the following sections:
      - **Introduction**: Explain the purpose of `sproto.yaml` and how it fits into the SProto ecosystem
      - **File Location**: Describe where the file should be placed (root of proto directory)
      - **Schema Version**: Document the versioning scheme for the configuration format
    - Include a complete formal specification in a table format:
      ```markdown
      | Field | Type | Required | Description |
      |-------|------|----------|-------------|
      | `version` | string | Yes | Schema version (currently only "v1") |
      | `name` | string | Yes | Module identifier in "namespace/name" format (e.g., "myorg/common") |
      | `import_path` | string | Yes | Base Go-style import path for this module (e.g., "github.com/myorg/common") |
      | `dependencies` | array | No | List of modules this module depends on |
      | `dependencies[].namespace` | string | Yes (if dependencies present) | Organization or user namespace |
      | `dependencies[].name` | string | Yes (if dependencies present) | Module name |
      | `dependencies[].version` | string | Yes (if dependencies present) | Version constraint (e.g., "v1.0.0", ">=v1.2.0") |
      | `dependencies[].import_path` | string | No | Import path prefix for this dependency |
      ```
    - Provide a detailed explanation of version constraint syntax based on SemVer:
      ```markdown
      ## Version Constraints
      
      SProto supports the following version constraint operators:
      
      - Exact version: `v1.2.3`
      - Greater than: `>v1.2.3`
      - Greater than or equal: `>=v1.2.3`
      - Less than: `<v2.0.0`
      - Less than or equal: `<=v1.9.0`
      - Tilde range: `~v1.2.3` (equivalent to `>=v1.2.3 <v1.3.0`)
      - Caret range: `^v1.2.3` (equivalent to `>=v1.2.3 <v2.0.0`)
      
      Multiple constraints can be combined with spaces or commas:
      
      ```yaml
      version: ">=v1.2.0 <v2.0.0"  # Greater than or equal to v1.2.0 and less than v2.0.0
      ```
      ```
    - Include complete examples for different use cases:
      - Basic module without dependencies:
        ```yaml
        version: v1
        name: myorg/utils
        import_path: github.com/myorg/utils
        ```
      - Module with a single dependency:
        ```yaml
        version: v1
        name: myorg/api
        import_path: github.com/myorg/api
        dependencies:
          - namespace: myorg
            name: common
            version: v1.2.0
            import_path: github.com/myorg/common
        ```
      - Module with multiple dependencies with version constraints:
        ```yaml
        version: v1
        name: myorg/service
        import_path: github.com/myorg/service
        dependencies:
          - namespace: myorg
            name: common
            version: ">=v1.0.0 <v2.0.0"
            import_path: github.com/myorg/common
          - namespace: myorg
            name: api
            version: "^v1.2.3"
            import_path: github.com/myorg/api
          - namespace: google
            name: protobuf
            version: "v1.28.0"
            import_path: google/protobuf
        ```
      - Advanced example with Generate templates:
        ```yaml
        version: v1
        name: myorg/advanced
        import_path: github.com/myorg/advanced
        dependencies:
          - namespace: myorg
            name: common
            version: ">=v1.0.0"
          - namespace: grpc
            name: ecosystem
            version: "v1.0.0"
        
        # Generation templates (future feature)
        generate:
          go:
            output: gen/go
            options:
              - paths=source_relative
          grpc-gateway:
            output: gen/gw
            options:
              - logtostderr=true
        ```
    - Add an appendix with validation rules:
      - Namespace and name must be valid identifiers (allowed characters, length limits)
      - Import paths must be valid (format rules, restricted characters)
      - Version constraints must follow the SemVer specification
      - No duplicate dependencies (same namespace/name)
      - No circular dependencies
    - Include a JSON Schema link (reference to the schema created in Task 1.1.1)
    - Add migration notes for users coming from Buf with examples of equivalent files
    - Include troubleshooting section for common configuration errors

- **5.3.4**: Create usage examples
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a new documentation file (e.g., `docs/usage-examples.md`) with clear, comprehensive examples
    - Structure the document with these major sections:
      - **Introduction**: Overview of what the examples cover and how they build upon each other
      - **Prerequisites**: Required tools (protoreg-cli, protoc, etc.), sample repositories used in examples
      - **Getting Started**: Installation and basic setup instructions
    - Provide detailed step-by-step examples for these common workflows, each with its own section:
      1. **Creating and Publishing a New Module**:
         ```markdown
         ## Creating and Publishing a New Module
         
         This example demonstrates creating a simple "common" module with basic message types and publishing it to the registry.
         
         ### Step 1: Create the module structure
         
         ```bash
         mkdir -p common/types
         ```
         
         ### Step 2: Create a basic message type
         
         Create `common/types/user.proto`:
         
         ```protobuf
         syntax = "proto3";
         
         package myorg.common.types;
         
         message User {
           string id = 1;
           string name = 2;
           string email = 3;
         }
         ```
         
         ### Step 3: Create the sproto.yaml configuration
         
         Create `common/sproto.yaml`:
         
         ```yaml
         version: v1
         name: myorg/common
         import_path: github.com/myorg/common
         ```
         
         ### Step 4: Publish the module
         
         ```bash
         cd common
         protoreg-cli publish . --version v1.0.0
         ```
         ```
      2. **Creating a Module with Dependencies**:
         - Example with a service that depends on the common module
         - Show both proto files and sproto.yaml with dependencies
         - Demonstrate how to reference types from dependencies in your proto files
      3. **Publishing a Module with Dependencies**:
         - Show validation process, warnings, and success output
         - Example of fixing issues that might occur during publish
      4. **Resolving Dependencies for a Project**:
         - How to use `protoreg-cli resolve` command to fetch all dependencies
         - Explain the resolution process and output
         - Show how to use `--update` flag to refresh dependencies
      5. **Fetching Specific Modules with Dependencies**:
         - Using `protoreg-cli fetch` with `--with-deps` option
         - Demonstrate correct directory structure after fetch
         - Comparison between fetching with and without dependencies
      6. **Compiling Protos with Resolved Dependencies**:
         - Using `protoreg-cli compile` to automatically handle includes
         - Example for generating Go code using the correct proto paths
         - Example with multiple output formats (Go, gRPC, etc.)
      7. **Managing the Cache**:
         - Show cache list, inspect, clean and invalidate commands
         - Real-world examples of when to clean or invalidate cache
         - Example output of cache operations
    - For each workflow example, include:
      - Complete sample code (proto files, sproto.yaml) with syntax highlighting
      - Full command-line examples with expected output
      - Explanation of key concepts and how they relate to SProto's dependency system
      - Troubleshooting tips for common issues
    - Add advanced examples section:
      - Working with complex dependency graphs with diamond dependencies
      - Handling version conflicts
      - Migration example from Buf to SProto
      - Integration with CI/CD pipelines
    - Create a cheat sheet at the end with common commands and options
    - Include downloadable example repository with all the example code ready to run
