# Migration Checklist: Buf to SProto

This checklist provides a step-by-step guide for migrating your Protobuf workflow from Buf to SProto. Follow these steps in order to ensure a smooth transition.

## Table of Contents

1. [Preparation Phase](#preparation-phase)
2. [Setup Phase](#setup-phase)
3. [Module Migration Phase](#module-migration-phase)
4. [Integration Phase](#integration-phase)
5. [Testing Phase](#testing-phase)
6. [Migration Types](#migration-types)
   - [For Small Teams](#for-small-teams)
   - [For Large Organizations](#for-large-organizations)
   - [For CI/CD-Heavy Workflows](#for-cicd-heavy-workflows)
7. [Common Issues and Solutions](#common-issues-and-solutions)

## Preparation Phase

Before you begin the migration process, complete these preparation steps:

- [ ] **Inventory all Protobuf modules**
  - List all modules managed by Buf
  - Identify dependencies between modules
  - Document module owners/maintainers

- [ ] **Analyze current Buf usage patterns**
  - Identify which Buf features are used (BSR, lint, breaking change detection, code generation)
  - Document current workflows and automation around Buf
  - List any custom scripts or tools that interact with Buf

- [ ] **Evaluate SProto limitations**
  - Review the [Configuration Conversion Guide](config-conversion.md) to identify Buf features without direct SProto equivalents
  - Plan for alternative solutions where needed (e.g., external linting tools)
  - Determine if any workflows need redesigning

- [ ] **Create a dependency graph**
  - Map out the dependencies between your modules
  - Identify base modules (those with no dependencies)
  - Identify "leaf" modules (those with many dependencies)

- [ ] **Plan rollout strategy**
  - Decide between all-at-once or phased migration
  - For phased approaches, start with base modules and work up the dependency chain
  - Establish a timeframe for the migration

## Setup Phase

Set up the SProto infrastructure and tools:

- [ ] **Deploy SProto Registry server**
  - Follow the installation instructions in the main README
  - Set up the database (Postgres/SQLite)
  - Configure the storage backend (MinIO/S3/local)
  - Secure the deployment with proper authentication

- [ ] **Install SProto CLI tools**
  - Install the `protoreg-cli` on all development machines
  - Configure the CLI to connect to your registry
  - Update build scripts to use SProto instead of Buf

- [ ] **Set up a test environment (optional)**
  - Create a parallel SProto registry for testing
  - Convert a few sample modules as a trial run
  - Test the entire workflow before full migration

- [ ] **Create conversion tools**
  - Adapt the [conversion script](config-conversion.md#automated-conversion) for your specific needs
  - Test the conversion tool on sample buf.yaml files
  - Create any additional scripts needed for your specific workflow

## Module Migration Phase

Migrate modules from Buf to SProto:

- [ ] **Start with base modules**
  - Convert `buf.yaml` to `sproto.yaml` for modules with no dependencies
  - Set appropriate version constraints
  - Define import paths correctly (most critical step!)

- [ ] **Publish base modules to SProto Registry**
  - Use `protoreg-cli publish` to publish each converted module
  - Verify successful publishing via the registry API
  - Document the published modules and their versions

- [ ] **Migrate dependent modules**
  - Convert the next level of modules in the dependency hierarchy
  - Update dependency references to point to the published base modules
  - Verify that import paths are correctly mapped

- [ ] **Repeat for all modules**
  - Continue the process, working up the dependency chain
  - Maintain a list of migrated vs. pending modules
  - Regularly test that everything works as expected

## Integration Phase

Update your broader development workflow to use SProto:

- [ ] **Update CI/CD pipelines**
  - Modify build scripts to use `protoreg-cli` instead of `buf`
  - Update artifact publishing steps
  - Update dependency resolution steps
  - See the [Workflow Differences Guide](workflow-differences.md) for command mappings

- [ ] **Update code generation workflows**
  - Replace `buf generate` with `protoreg-cli compile`
  - Modify any template-based generation
  - Test that generated code compiles and works correctly

- [ ] **Update developer documentation**
  - Document the new workflow for developers
  - Provide examples of common tasks with SProto
  - Create guides for new team members

- [ ] **Migration validation**
  - Verify that all modules are correctly published
  - Check that dependencies resolve properly
  - Test that import paths work correctly

## Testing Phase

Thoroughly test the migrated system:

- [ ] **Test module resolution**
  - Create test projects that depend on various modules
  - Verify that `protoreg-cli resolve` correctly resolves all dependencies
  - Test with various version constraints

- [ ] **Test code generation**
  - Ensure that generated code is identical (or functionally equivalent) to what Buf produced
  - Verify that all necessary plugins work correctly
  - Test integration with other build tools

- [ ] **Test in production-like environment**
  - Deploy to a staging environment
  - Run full system tests
  - Verify that all services communicate correctly

- [ ] **Perform regression testing**
  - Compare results with previous Buf-based workflows
  - Check for any regressions or issues
  - Address any discrepancies

## Migration Types

### For Small Teams

For teams with a limited number of Protobuf modules:

1. **Single-Phase Migration**
   - Convert all modules at once
   - Manually update configuration files
   - Focus on thorough testing
   - Timeline: 1-2 days

2. **Key Considerations**
   - Backup all existing modules before migration
   - Schedule migration during a low-activity period
   - Have all team members available during migration

### For Large Organizations

For organizations with many modules and teams:

1. **Module-Group Migration**
   - Group modules by team or functionality
   - Migrate one group at a time
   - Use automation for configuration conversion
   - Timeline: 1-2 weeks

2. **Key Considerations**
   - Establish clear ownership for each module
   - Coordinate between teams for dependent modules
   - Create centralized documentation for the migration
   - Consider running both systems in parallel during transition

### For CI/CD-Heavy Workflows

For teams with extensive CI/CD automation:

1. **Pipeline-Focused Migration**
   - Inventory all pipelines using Buf
   - Create parallel pipelines using SProto
   - Test thoroughly before switching
   - Timeline: 1-2 weeks

2. **Key Considerations**
   - Update deployment scripts and hooks
   - Modify any custom tools that interact with Buf
   - Pay special attention to versioning and tag management
   - Consider canary deployments of the new pipelines

## Common Issues and Solutions

### Import Path Resolution Issues

**Problem**: Proto imports fail to resolve correctly after migration.

**Solution**:
- Verify that the `import_path` field in `sproto.yaml` matches the actual import paths used in `.proto` files
- Check that all dependencies are correctly declared
- Run `protoreg-cli resolve --verbose` to debug resolution problems
- Ensure the import prefix in `.proto` files matches the `import_path` in the dependency's `sproto.yaml`

### Version Constraint Conflicts

**Problem**: Dependency resolution fails due to conflicting version constraints.

**Solution**:
- Review all version constraints in your `sproto.yaml` files
- Use broader version ranges where possible (e.g., `>=v1.0.0, <v2.0.0` instead of exact versions)
- Analyze the dependency graph to identify which modules are causing conflicts
- Consider updating some modules to newer versions to resolve conflicts

### Missing Linting Functionality

**Problem**: SProto doesn't include Buf's linting capabilities.

**Solution**:
- Integrate standalone Protobuf linters like `protolint`
- Create a separate lint step in your CI/CD pipeline
- Consider using `protoc` plugins for linting
- Document linting standards for developers to follow manually

### Breaking Change Detection Absence

**Problem**: SProto doesn't include breaking change detection.

**Solution**:
- Implement stricter semantic versioning practices
- Create pre-publish hooks to manually review changes
- Consider integrating a standalone breaking change detector tool
- Add more comprehensive testing for API compatibility

### Cache Management Issues

**Problem**: SProto cache behaves differently than Buf's cache.

**Solution**:
- Review the `protoreg-cli cache` commands and understand the differences
- Regularly clean the cache during development with `protoreg-cli cache clean`
- Add cache invalidation steps to CI/CD pipelines when needed
- Use `protoreg-cli resolve --update` to force dependency updates

### Code Generation Differences

**Problem**: Code generation produces different results than with Buf.

**Solution**:
- Review the `protoreg-cli compile` command and how it differs from `buf generate`
- Verify that all protoc plugins are correctly installed and configured
- Compare generated outputs to identify specific differences
- Modify templates or scripts as needed to match the previous output

### Authentication Failures

**Problem**: Unable to publish modules due to authentication issues.

**Solution**:
- Verify that the API token is correctly configured in the CLI or environment variables
- Check server logs for authentication errors
- Ensure the token has not expired or been revoked
- Confirm that the registry URL is correct in the configuration
