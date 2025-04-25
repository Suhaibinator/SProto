### Phase 3: CLI Enhancements

#### Task 3.1: Update Publish Command
- **3.1.1**: Extend publish command to process sproto.yaml
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Modify `internal/cli/publish.go` to automatically detect and parse `sproto.yaml` in the root of the directory being published.
    - If `sproto.yaml` is found:
      - Use the `name` and `import_path` fields from the config instead of requiring `--module` flag (make `--module` optional if config exists).
      - Validate the parsed configuration using the logic from Task 1.1.3.
      - Pass the parsed `import_path` and `dependencies` to the registry API during the publish request.
    - If `sproto.yaml` is not found, maintain existing behavior (require `--module` flag).
    - Add a new flag `--config-path` to specify an alternative location for `sproto.yaml`.
    - Update command help text and examples to reflect the new behavior.
    - Add logging to indicate whether a config file was found and used.

- **3.1.2**: Implement import path extraction during publish
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Integrate the `ImportScanner` (from Task 2.1.1) into the publish command.
    - Before zipping, scan all `.proto` files in the target directory to extract their import paths.
    - Compare the extracted import paths against the `import_path` declared in `sproto.yaml` (if present) and the `import_path` of declared dependencies.
    - Add validation to ensure all imports can be resolved either within the module itself or via a declared dependency's import path.
    - Report errors for unresolved imports.
    - Optionally, add a flag `--skip-import-validation` to bypass this check.
    - Consider adding extracted import information to the metadata sent to the registry (potentially for future analysis or validation).

- **3.1.3**: Add dependency declaration validation
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Before uploading the artifact, if `sproto.yaml` contains dependencies:
      - For each dependency listed:
        - Query the registry API (using the registry client) to verify that the specified module (`namespace/name`) exists.
        - Optionally (controlled by a flag like `--validate-dependency-versions`), check if the specified version constraint can be satisfied by at least one version available in the registry.
      - Report clear errors if any declared dependencies are not found in the registry or if version constraints cannot be met.
      - Provide informative messages about which dependencies were validated.
      - Ensure this validation step happens *before* the actual artifact upload to avoid partial publishes.

#### Task 3.2: Implement Dependency Resolution Command
- **3.2.1**: Create new `resolve` command structure
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Create a new file `internal/cli/resolve.go`.
    - Define a new Cobra command `resolveCmd`:
      ```go
      var resolveCmd = &cobra.Command{
          Use:   "resolve [module_ref]",
          Short: "Resolve and fetch dependencies for a module",
          Long: `Resolves the dependency tree for a given module (or the module in the current directory if sproto.yaml exists) 
                 and fetches the required artifacts into the local cache.`,
          Run: func(cmd *cobra.Command, args []string) { /* ... */ },
      }
      ```
    - Add flags:
      - `--output <dir>`: (Optional) Directory to write resolved dependency information (e.g., a lock file).
      - `--update`: Force re-fetching dependencies even if they exist in the cache.
      - `--no-cache`: Disable using the cache entirely.
    - The command should:
      - Identify the root module (either from `sproto.yaml` or `module_ref` argument).
      - Build the dependency graph (using logic from Task 2.2).
      - Resolve compatible versions (using logic from Task 2.2.2).
      - Fetch required artifacts (Task 3.2.2).

- **3.2.2**: Implement recursive dependency fetching
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Integrate the `DependencyGraph` resolver (Task 2.2) into the `resolve` command.
    - After resolving the versions, iterate through the required modules and versions.
    - For each required module version:
      - Check if it exists in the local cache (Task 4.1).
      - If not cached or `--update` flag is used:
        - Use the registry client's `FetchArtifact` method (similar to the existing `fetch` command but adapted for caching).
        - Download the artifact zip.
        - Store the artifact in the cache directory structure (Task 4.1.1).
        - Verify artifact integrity (e.g., using SHA256 digest if provided by API).
      - Handle download errors gracefully (retries, network issues).
      - Update cache metadata.

- **3.2.3**: Add progress reporting for resolution process
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Implement user-friendly progress indicators during the `resolve` command execution.
    - Use a library like `github.com/vbauerster/mpb` or simple log messages.
    - Show progress for:
      - Building the initial dependency graph.
      - Querying the registry for module versions.
      - Resolving version constraints.
      - Downloading artifacts.
      - Extracting files (if applicable).
    - Provide a summary at the end:
      - Number of modules resolved.
      - Number of artifacts downloaded vs. cache hits.
      - Total time taken.
    - Add different verbosity levels using `--log-level` flag.

#### Task 3.3: Enhance Fetch Command
- **3.3.1**: Update fetch to handle import paths
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Modify the existing `fetch` command in `internal/cli/fetch.go`.
    - When extracting the downloaded zip artifact:
      - Determine the module's base `import_path` (query registry API if needed).
      - Extract files into a directory structure that mirrors the Go import path convention relative to the `--output` directory.
      - Example: Fetching `myorg/common@v1.0.0` with `import_path: github.com/myorg/common` into `--output ./protos` should place files like `./protos/github.com/myorg/common/types.proto`.
      - Use the `path/filepath` package for constructing paths correctly across OSes.
      - Update documentation and examples for the `fetch` command.

- **3.3.2**: Implement dependency resolution in fetch
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Add a new flag `--with-deps` to the `fetch` command.
    - If `--with-deps` is specified:
      - After fetching the primary module artifact, trigger the dependency resolution logic (similar to the `resolve` command, Task 3.2).
      - Resolve the full dependency tree for the fetched module.
      - Fetch all required dependency artifacts into the *local cache* (not necessarily the `--output` directory of the primary fetch).
      - This ensures dependencies are available for subsequent compilation steps that use the cache.
    - Clearly document the difference between fetching a single module vs. fetching with dependencies.

- **3.3.3**: Add caching integration
  - **Assignee**: Cline
  - **Status**: TODO
  - **Details**: 
    - Modify the `fetch` command to check the local cache *before* attempting to download an artifact from the registry.
    - If the requested module version exists in the cache (and `--update` is not specified):
      - Use the cached artifact directly for extraction.
      - Report a cache hit in the logs.
    - If downloading from the registry, store the downloaded artifact in the cache after successful download and verification.
    - Ensure cache interaction uses the cache management logic defined in Phase 4.
