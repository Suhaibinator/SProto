# Workflow Differences: Buf vs. SProto

This guide compares common workflows between Buf and SProto, providing command examples and highlighting key differences to help teams transition between the systems.

## Table of Contents
1. [Module Initialization](#module-initialization)
2. [Dependency Management](#dependency-management)
3. [Publishing Modules](#publishing-modules)
4. [Fetching Modules](#fetching-modules)
5. [Code Generation](#code-generation)
6. [Local Cache Management](#local-cache-management)
7. [Significant Differences](#significant-differences)

## Module Initialization

### Buf
```bash
# Create a new module in the current directory
buf mod init

# Create a new module with specific name
buf mod init --module-name buf.build/acme/petstore
```

### SProto
```bash
# SProto doesn't have a command for initialization
# Instead, manually create a sproto.yaml file:

cat > sproto.yaml << EOF
version: v1
name: acme/petstore
import_path: github.com/acme/petstore/proto
EOF
```

**Key Differences:**
- Buf provides a CLI command for module initialization
- SProto requires manual creation of the configuration file
- SProto requires an explicit `import_path` field

## Dependency Management

### Buf
```bash
# Add a dependency
buf mod update

# Update dependencies to latest versions
buf mod update

# List dependencies
buf mod ls-deps
```

### SProto
```bash
# Resolve dependencies (after manually adding them to sproto.yaml)
protoreg-cli resolve

# Update dependencies to latest versions matching constraints
protoreg-cli resolve --update

# List dependencies without fetching
protoreg-cli resolve --dry-run
```

**Key Differences:**
- Buf can add dependencies via CLI, SProto requires manual edits to `sproto.yaml`
- SProto requires explicit version constraints for each dependency
- SProto requires explicit import paths for each dependency

## Publishing Modules

### Buf
```bash
# Push current module to registry
buf push

# Push with specific tag
buf push --tag v1.0.0
```

### SProto
```bash
# Publish current directory as a module
protoreg-cli publish . --module acme/petstore --version v1.0.0

# Specify source directory
protoreg-cli publish ./proto --module acme/petstore --version v1.0.0
```

**Key Differences:**
- SProto requires explicit module name and version on publish command
- Buf derives this information from the `buf.yaml` file
- SProto doesn't have a concept of "draft" versions - all versions are immutable
- Authentication mechanisms differ (Buf uses user accounts, SProto uses a static token)

## Fetching Modules

### Buf
```bash
# Export a module to a directory
buf export buf.build/acme/petstore:v1.0.0 --output ./proto

# Use as part of build process (implicit fetch)
buf build
```

### SProto
```bash
# Fetch a specific module version
protoreg-cli fetch acme/petstore v1.0.0 --output ./proto

# Fetch with dependencies
protoreg-cli fetch acme/petstore v1.0.0 --output ./proto --with-deps

# Resolve dependencies from sproto.yaml (similar to Buf's implicit fetch)
protoreg-cli resolve
```

**Key Differences:**
- SProto separates fetching modules from resolving dependencies
- SProto requires explicit version specification
- SProto has explicit flag for including dependencies

## Code Generation

### Buf
```bash
# Generate code using buf.gen.yaml configuration
buf generate

# Generate with specific template
buf generate --template buf.gen.yaml
```

### SProto
```bash
# Compile with protoc plugins (after resolving dependencies)
protoreg-cli compile --go_out=./gen --go-grpc_out=./gen

# With additional options
protoreg-cli compile --go_out=./gen --go-grpc_out=./gen --proto_path=./extra-protos
```

**Key Differences:**
- Buf uses a configuration file for code generation
- SProto passes options directly to protoc via CLI flags
- SProto doesn't have managed plugin support - relies on locally installed protoc plugins
- Buf has advanced template features, SProto offers simpler direct protoc integration

## Local Cache Management

### Buf
```bash
# Clear Buf's module cache
buf mod prune

# Clear per-module cached artifacts
buf build --cache-disable
```

### SProto
```bash
# List cached modules
protoreg-cli cache list

# Clean specific module from cache
protoreg-cli cache clean acme/petstore

# Clean entire cache
protoreg-cli cache clean

# Bypass cache for a resolve operation
protoreg-cli resolve --update
```

**Key Differences:**
- SProto provides more granular cache management commands
- SProto caches are module-specific and can be individually managed
- Buf's cache system is more integrated with its build process

## Significant Differences

### 1. Configuration Structure

**Buf** centralizes multiple concerns in `buf.yaml`:
- Module identity
- Dependencies
- Build configuration
- Lint rules
- Breaking change rules

**SProto** uses `sproto.yaml` for a more focused set of concerns:
- Module identity
- Import path specification
- Dependencies with explicit version constraints

### 2. Import Path Handling

**Buf** uses:
- Module identity based on registry paths (buf.build/org/repo)
- Imports often use well-known import paths (google/protobuf/timestamp.proto)

**SProto** uses:
- Explicit mapping between import paths and modules
- Import paths typically follow Go-style conventions (github.com/org/repo/proto/...)
- Each dependency must declare its import_path

### 3. Authentication Models

**Buf**:
- User accounts with API tokens
- Organization-level permissioning
- Role-based access control

**SProto**:
- Simple static token authentication
- Single token for the entire registry
- No built-in user management

### 4. Code Generation

**Buf**:
- Plugin management system
- Template-based configuration
- Remote plugin execution

**SProto**:
- Direct protoc integration
- Relies on locally installed plugins
- Simpler, more direct approach

### 5. Missing Features in SProto

Several Buf features have no direct equivalent in SProto:
- Linting and breaking change detection
- Managed remote plugins
- Workspaces for multi-module development
- Advanced build configuration (excludes, includes)
- Template-based code generation

## Command Mapping Summary

| Task | Buf Command | SProto Command |
|------|-------------|----------------|
| Initialize module | `buf mod init` | *(manual creation)* |
| Resolve dependencies | `buf mod update` | `protoreg-cli resolve` |
| Update dependencies | `buf mod update` | `protoreg-cli resolve --update` |
| List dependencies | `buf mod ls-deps` | `protoreg-cli resolve --dry-run` |
| Publish module | `buf push` | `protoreg-cli publish . --module x/y --version v1.0.0` |
| Download module | `buf export` | `protoreg-cli fetch x/y v1.0.0 --output ./dir` |
| Generate code | `buf generate` | `protoreg-cli compile --<plugin>_out=./dir` |
| Clear cache | `buf mod prune` | `protoreg-cli cache clean` |
| Lint code | `buf lint` | *(not supported)* |
| Breaking changes | `buf breaking` | *(not supported)* |
