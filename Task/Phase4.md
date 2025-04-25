### Phase 4: Caching and Directory Structure

#### Task 4.1: Implement Cache Management
- **4.1.1**: Design cache directory structure
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Define a standard cache location, typically `~/.cache/sproto` (use `os.UserCacheDir()` for cross-platform compatibility).
    - Structure the cache to store downloaded module artifacts (zips) and potentially extracted files.
    - Proposed structure:
      ```
      ~/.cache/sproto/
        modules/
          <namespace>/
            <module_name>/
              <version>/
                artifact.zip  # The downloaded artifact
                sproto.yaml   # Extracted config (if present)
                extracted/    # (Optional) Extracted files for direct use
                  <import_path_structure>/...
        metadata/
          index.json # (Optional) Index of cached modules/versions
      ```
    - Document the cache layout clearly.
    - Consider adding a cache lock file mechanism to prevent concurrent access issues.

- **4.1.2**: Implement cache operations (get, put, invalidate)
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a new package `internal/cache`.
    - Implement core cache functions:
      ```go
      type Cache struct {
          rootDir string
      }
      
      // NewCache initializes the cache manager
      func NewCache() (*Cache, error) 
      
      // GetArtifactPath returns the path to a cached artifact zip, bool indicates if found
      func (c *Cache) GetArtifactPath(namespace, name, version string) (string, bool, error)
      
      // PutArtifact stores a downloaded artifact (from a reader) into the cache
      func (c *Cache) PutArtifact(namespace, name, version string, reader io.Reader) error
      
      // GetExtractedPath returns the path to the root of extracted files for a version
      func (c *Cache) GetExtractedPath(namespace, name, version string) (string, bool, error)
      
      // ExtractArtifact extracts a cached artifact zip into the 'extracted' directory
      func (c *Cache) ExtractArtifact(namespace, name, version string) error
      
      // Invalidate removes a specific module version or entire module from the cache
      func (c *Cache) Invalidate(namespace, name, version string) error // version is optional
      
      // Clean removes old or unused cache entries (based on policy TBD)
      func (c *Cache) Clean() error 
      ```
    - Use `github.com/mitchellh/go-homedir` or `os.UserCacheDir()` to determine the cache root directory.
    - Implement file locking for safe concurrent writes.
    - Add checksum verification for cached artifacts.

- **4.1.3**: Add cache status reporting
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a new CLI command `sproto cache <subcommand>`.
    - Implement subcommands:
      - `sproto cache list`: List all modules and versions currently in the cache.
      - `sproto cache path <module_ref@version>`: Print the filesystem path to the cached artifact or extracted files.
      - `sproto cache clean`: Trigger the cache cleaning process.
      - `sproto cache invalidate <module_ref[@version]>`: Remove specific entries from the cache.
      - `sproto cache size`: Report the total disk space used by the cache.
    - Integrate these commands with the `internal/cache` package.
    - Provide clear output formats for each command.

#### Task 4.2: Implement Import Path Directory Structure
- **4.2.1**: Create directory structure generator
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Implement logic within the `internal/cache` or a dedicated `internal/layout` package.
    - Create a function that takes a module's base `import_path` (e.g., "github.com/myorg/common") and a relative file path within the module (e.g., "types/money.proto") and generates the corresponding full path within the cache's `extracted` directory (e.g., `~/.cache/sproto/modules/myorg/common/v1.0.0/extracted/github.com/myorg/common/types/money.proto`).
    - Ensure correct handling of path separators across different operating systems (`path/filepath`).
    - This logic will be used by `ExtractArtifact` (Task 4.1.2) and potentially by the `fetch` command (Task 3.3.1).

- **4.2.2**: Implement file extraction preserving paths
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Enhance the `ExtractArtifact` function in `internal/cache`.
    - When extracting the `artifact.zip`:
      - Read the `sproto.yaml` from the zip (if it exists) to determine the module's base `import_path`. If not found, potentially query the registry or use a default based on `namespace/name`.
      - For each file in the zip:
        - Construct the target extraction path using the logic from Task 4.2.1 (base import path + relative path within zip).
        - Create necessary parent directories.
        - Extract the file to the target path.
      - Ensure file permissions are preserved during extraction.
      - Handle potential path traversal vulnerabilities securely.

- **4.2.3**: Add symlink support for complex structures (Optional/Advanced)
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Investigate scenarios where symlinks might be beneficial (e.g., mapping well-known types like Google protos without duplicating files).
    - If deemed necessary, add logic to `ExtractArtifact` to create symlinks instead of copying files under certain conditions.
    - Example: Link `~/.cache/sproto/modules/google/protobuf/vX.Y.Z/extracted/google/protobuf/timestamp.proto` to a central location for Google protos.
    - Ensure cross-platform compatibility for symlink creation (may require OS-specific code or checks).
    - Add configuration options to enable/disable symlink usage.

#### Task 4.3: Create Protoc Helper
- **4.3.1**: Implement proto_path generator
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a function or method, likely within the `internal/resolver` or a new `internal/compiler` package.
    - Input: A resolved dependency graph (map of module IDs to resolved versions).
    - Output: A list of directory paths suitable for use with `protoc --proto_path` (or `-I`).
    - Logic:
      - For each resolved module version in the dependency set:
        - Get the path to its extracted files in the cache (using `cache.GetExtractedPath`).
        - Add this path to the list of include paths.
      - Ensure the list contains unique paths.
      - Add the path to the current project's proto source directory.
      - Format the output as a single string with paths separated by the OS-specific list separator (e.g., `:` on Unix, `;` on Windows).
      ```go
      // GenerateProtoPath generates the --proto_path string for protoc
      func GenerateProtoPath(resolvedDeps map[string]string, cache *cache.Cache, projectProtoDir string) (string, error)
      ```

- **4.3.2**: Add protoc command wrapper
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a new CLI command, e.g., `sproto compile` or `sproto protoc`.
    - This command would:
      - Perform dependency resolution (similar to `sproto resolve`).
      - Generate the required `--proto_path` string using the logic from Task 4.3.1.
      - Identify the `.proto` files to be compiled (e.g., all files in the current module).
      - Construct the full `protoc` command line, including include paths, output directories, plugin options, and input files.
      - Execute the `protoc` command using `os/exec`.
      - Capture and display output/errors from `protoc`.
    - Add flags to pass through options to `protoc` (e.g., `--go_out`, `--grpc-gateway_out`, plugin paths).
    - Allow configuration of the `protoc` binary path.

- **4.3.3**: Create common generation configurations (Optional)
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Allow defining named generation templates within `sproto.yaml`.
    - Example:
      ```yaml
      generate:
        go:
          output: gen/go
          options:
            - paths=source_relative
        grpc-gateway:
          output: gen/gw
          options:
            - logtostderr=true
            - paths=source_relative
      ```
    - Enhance the `sproto compile [template_name]` command to use these predefined templates, simplifying common compilation tasks.
