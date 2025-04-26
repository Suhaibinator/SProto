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

The configuration formats between Buf and SProto have important differences that must be addressed during migration.

**Key Differences:**
- SProto requires an explicit `import_path` field
- SProto uses `namespace/name` format without the `buf.build/` prefix
- Dependencies in SProto require explicit version constraints
- Dependencies in SProto require explicit import paths
- SProto doesn't support build configuration, linting, or breaking change detection

For detailed examples and a complete field-by-field mapping guide, see:
- [**Detailed Configuration Conversion Guide**](migrating-from-buf/config-conversion.md)

## Command Comparison

The command-line interfaces for Buf and SProto serve similar purposes but with different structures and options.

**Key Command Mappings:**
- Module initialization: `buf mod init` → Manual creation of `sproto.yaml`
- Dependency resolution: `buf mod update` → `protoreg-cli resolve`
- Publishing modules: `buf push` → `protoreg-cli publish`
- Downloading modules: `buf export` → `protoreg-cli fetch`
- Code generation: `buf generate` → `protoreg-cli compile`
- Cache management: `buf mod prune` → `protoreg-cli cache clean`

For comprehensive command examples and workflow comparisons, see:
- [**Detailed Workflow Differences Guide**](migrating-from-buf/workflow-differences.md)

## Migration Strategy

> For a comprehensive and detailed migration checklist, see our [**Migration Checklist**](migrating-from-buf/migration-checklist.md) document which provides step-by-step instructions for different team sizes and project types.

### Step 1: Set Up SProto Registry

1. Deploy the SProto registry using Docker Compose
2. Configure authentication and security settings
3. Test connectivity with `protoreg-cli configure`

### Step 2: Convert Configuration Files

1. Create `sproto.yaml` files based on existing `buf.yaml` files
2. Add the required `import_path` fields
3. Expand dependencies with explicit version constraints

For this critical step:
- Use the detailed [Configuration Conversion Guide](migrating-from-buf/config-conversion.md) for field-by-field mapping
- Follow the examples showing before/after configuration files
- Use either the manual conversion approach or the provided script template

The configuration conversion is the most important step in the migration process, as it defines how your modules will be identified and how dependencies will be resolved.

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
4. Provide training on workflow differences

Share the [Workflow Differences Guide](migrating-from-buf/workflow-differences.md) with your development team to help them understand how SProto's commands differ from Buf's. This guide will serve as a quick reference for developers as they adjust to the new system and workflows.

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

For a comprehensive list of common issues and their solutions, refer to the [Common Issues and Solutions](migrating-from-buf/migration-checklist.md#common-issues-and-solutions) section in our Migration Checklist.

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
