# Migrating from Buf to SProto

This guide helps Buf users transition to SProto for Protobuf module management, explaining key differences and providing migration strategies.

## Table of Contents

1. [Introduction](#introduction)
2. [Core Concept Comparison](#core-concept-comparison)
3. [Configuration File Comparison](#configuration-file-comparison)
4. [Command Comparison](#command-comparison)
5. [Migration Strategy](#migration-strategy)
6. [Advanced Topics](#advanced-topics)
7. [Troubleshooting](#troubleshooting)

## Introduction

SProto is inspired by the Buf Schema Registry (BSR) but offers a simpler, self-hosted alternative with core versioning and dependency management capabilities. This guide is designed to help teams familiar with Buf migrate to SProto with minimal disruption.

### Key Differences at a Glance

- **Self-hosted vs. SaaS**: Unlike Buf's SaaS offering, SProto is entirely self-hosted
- **Simplified Scope**: SProto focuses on registry and dependency management, omitting some Buf features like linting and breaking change detection
- **Command Structure**: Different command organization and options, though with conceptual similarities
- **Configuration Format**: Different file structure, though with similar concepts

## Core Concept Comparison

| Buf Concept | SProto Equivalent | Notes |
|-------------|-------------------|-------|
| Module | Module | Both represent a collection of .proto files with a unique identity |
| Module Name (`buf.build/owner/repo`) | Module name (`namespace/name`) | SProto uses simpler `namespace/name` format without a registry prefix |
| Module Version | Module Version | Both use semver (v1.2.3) for versioning |
| BSR | SProto Registry | Self-hosted alternative to BSR |
| Dependencies | Dependencies | Both allow declaring dependencies on other modules |
| Workspaces | (Not supported) | SProto doesn't currently have a workspace concept |
| Module Cache (`~/.cache/buf`) | Module Cache (`~/.cache/sproto`) | Similar local caching mechanisms |
| Breaking Change Detection | (Not supported) | SProto focuses on core dependency management |
| Linting | (Not supported) | SProto doesn't include built-in lint capabilities |
| Code Generation | Basic support via `compile` | SProto has simpler code generation capabilities |

## Configuration File Comparison

### Basic Module Definition

**Buf (`buf.yaml`)**:
```yaml
version: v1
name: buf.build/acme/petapis
```

**SProto (`sproto.yaml`)**:
```yaml
version: v1
name: acme/petapis
import_path: github.com/acme/petapis/proto
```

Key differences:
- SProto requires an explicit `import_path` field
- Buf prefixes names with `buf.build/`, while SProto uses just `namespace/name`

### Dependencies

**Buf (`buf.yaml`)**:
```yaml
version: v1
name: buf.build/acme/petapis
deps:
  - buf.build/googleapis/googleapis
  - buf.build/acme/common
```

**SProto (`sproto.yaml`)**:
```yaml
version: v1
name: acme/petapis
import_path: github.com/acme/petapis/proto
dependencies:
  - namespace: googleapis
    name: googleapis
    version: "v1.0.0"
    import_path: google/apis
  - namespace: acme
    name: common
    version: ">=v1.0.0, <v2.0.0"
    import_path: github.com/acme/common/proto
```

Key differences:
- SProto requires explicit version constraints for each dependency
- SProto requires explicit import paths for each dependency
- SProto uses a more structured format for dependency declarations

### Buf Features Not in SProto

Buf's `buf.yaml` supports additional features that aren't currently in SProto:

```yaml
version: v1
name: buf.build/acme/petapis
deps:
  - buf.build/googleapis/googleapis
build:
  excludes:
    - foo/bar
lint:
  use:
    - DEFAULT
  except:
    - FIELD_LOWER_SNAKE_CASE
breaking:
  use:
    - FILE
```

SProto doesn't currently support:
- Build configuration (`build` section)
- Lint rules (`lint` section)
- Breaking change detection (`breaking` section)

## Command Comparison

| Buf Command | SProto Equivalent | Notes |
|-------------|-------------------|-------|
| `buf mod init` | (Manual creation) | Manually create `sproto.yaml` |
| `buf mod update` | `protoreg-cli resolve --update` | Update dependencies to latest versions |
| `buf push` | `protoreg-cli publish` | Push module to registry |
| `buf build` | `protoreg-cli resolve` | Resolve dependencies |
| `buf generate` | `protoreg-cli compile` | Generate code from protos with dependencies |
| `buf export` | `protoreg-cli fetch` | Download a module |
| `buf mod ls-deps` | `protoreg-cli resolve --dry-run` | List dependencies |
| `buf mod prune` | `protoreg-cli cache clean` | Clean up cache |
| `buf lint` | (Not supported) | No built-in lint support |
| `buf breaking` | (Not supported) | No breaking change detection |
| `buf ls-files` | (Not directly supported) | Use standard file system commands |

### Command Examples

#### Publishing a Module

**Buf**:
```bash
buf push
```

**SProto**:
```bash
protoreg-cli publish . --module acme/petapis --version v1.0.0
```

#### Resolving Dependencies

**Buf**:
```bash
buf mod update
```

**SProto**:
```bash
protoreg-cli resolve
```

#### Generating Code

**Buf**:
```bash
buf generate
```

**SProto**:
```bash
protoreg-cli compile --go_out=./gen --go-grpc_out=./gen
```

## Migration Strategy

### Step 1: Set Up SProto Registry

1. Deploy the SProto registry using Docker Compose
2. Configure authentication and security settings
3. Test connectivity with `protoreg-cli configure`

### Step 2: Convert Configuration Files

1. Create `sproto.yaml` files based on existing `buf.yaml` files
2. Add the required `import_path` fields
3. Expand dependencies with explicit version constraints

Example script for basic conversion (starting point, will need adjustments):

```python
import yaml
import os

def convert_buf_to_sproto(buf_yaml_path, default_version="v1.0.0"):
    with open(buf_yaml_path, 'r') as f:
        buf_config = yaml.safe_load(f)
    
    sproto_config = {
        "version": buf_config.get("version", "v1"),
        "name": buf_config.get("name", "").replace("buf.build/", ""),
        # You'll need to determine appropriate import_path values
        "import_path": f"github.com/{buf_config.get('name', '').replace('buf.build/', '')}/proto"
    }
    
    if "deps" in buf_config:
        sproto_config["dependencies"] = []
        for dep in buf_config["deps"]:
            # Parse dep which is in format "buf.build/namespace/name"
            parts = dep.replace("buf.build/", "").split("/")
            if len(parts) == 2:
                namespace, name = parts
                sproto_config["dependencies"].append({
                    "namespace": namespace,
                    "name": name,
                    "version": default_version,
                    # You'll need to determine appropriate import_path values
                    "import_path": f"github.com/{namespace}/{name}/proto"
                })
    
    sproto_yaml_path = os.path.join(os.path.dirname(buf_yaml_path), "sproto.yaml")
    with open(sproto_yaml_path, 'w') as f:
        yaml.dump(sproto_config, f, default_flow_style=False)
    
    print(f"Converted {buf_yaml_path} to {sproto_yaml_path}")

# Example usage
convert_buf_to_sproto("./path/to/buf.yaml")
```

### Step 3: Publish Modules to SProto Registry

1. Start with base modules that have no dependencies
2. Progress to modules with dependencies
3. Verify modules are properly registered

```bash
for module in base_modules/*; do
  protoreg-cli publish "$module" --module "$(basename $module)" --version v1.0.0
done

for module in dependent_modules/*; do
  protoreg-cli publish "$module" --module "$(basename $module)" --version v1.0.0
done
```

### Step 4: Update CI/CD Pipelines

1. Replace `buf` commands with equivalent `protoreg-cli` commands
2. Update authentication mechanisms
3. Adjust any custom scripts that interact with Buf

### Step 5: Transition Developers

1. Install `protoreg-cli` on development machines
2. Configure client to point to your SProto registry
3. Update documentation and onboarding processes

## Advanced Topics

### Authentication Differences

- **Buf**: Uses API keys tied to user accounts
- **SProto**: Uses a simpler static token system that can be configured in environment variables or client config

### Import Path Resolution

SProto's approach to import path resolution:

1. When a `.proto` file imports `"github.com/acme/common/proto/user.proto"`:
2. SProto looks at all dependencies' `import_path` fields
3. It identifies that this path starts with the `import_path` of a dependency
4. It fetches the correct module based on this mapping and version constraint

### Offline Usage

SProto maintains a local cache at `~/.cache/sproto` to allow offline work once dependencies are downloaded.

### Managing Multiple Environments

For teams that need to work with both Buf and SProto during transition:

```bash
# Create aliases to avoid confusion
alias buf-push="buf push"
alias sproto-push="protoreg-cli publish"

# Script to push to both systems
push_to_both() {
  buf-push
  sproto-push . --module "$(grep name buf.yaml | cut -d':' -f2 | tr -d ' ' | sed 's/buf.build\///')" --version "$1"
}
```

## Troubleshooting

### Common Issues

1. **Import path not found**
   - Ensure dependencies' `import_path` fields correctly match the imports in your `.proto` files
   - Check that all dependencies are declared in your `sproto.yaml`

2. **Version conflicts**
   - SProto requires explicit version constraints; check for incompatible constraints

3. **Missing features**
   - If you rely heavily on Buf linting or breaking change detection, you may need to add additional tools to your workflow

4. **Import path confusion**
   - Buf and SProto handle import paths differently; ensure your imports are consistent

### Getting Help

- Visit the SProto GitHub repository for issues and discussions
- Refer to the SProto documentation for detailed configuration options
