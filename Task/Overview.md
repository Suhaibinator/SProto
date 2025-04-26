# SProto Enhancement - Dependency Management System

## Section 1: Goal

### Overview

Our goal is to enhance SProto to provide Buf-like dependency management capabilities for Protobuf files, while leveraging SProto's existing registry functionality. 

Currently, SProto functions as a registry for storing, versioning, and retrieving Protobuf modules, but lacks the dependency management features that Buf provides. This enhancement will bridge that gap, allowing users to:

1. Use Go module style imports (e.g., `import "github.com/myorg/common/proto/types.proto"`)
2. Declare dependencies in a configuration file (similar to buf.yaml)
3. Benefit from automatic dependency resolution and fetching
4. Avoid manual --proto_path configuration
5. Utilize a local cache for downloaded dependencies

### Approach

We will extend SProto in several ways:

1. **Configuration Format**: Introduce a `sproto.yaml` configuration file format (similar to buf.yaml) - [Phase 1, Task 1.1](Phase1.md#task-11-design-and-implement-configuration-format)
2. **Database Schema**: Extend the database schema to store import path mappings and dependency relationships - [Phase 1, Task 1.2](Phase1.md#task-12-update-database-schema)
3. **CLI Enhancements**: Add dependency resolution capabilities to the CLI - [Phase 3](Phase3.md)
4. **Caching Mechanism**: Implement a local cache for downloaded dependencies - [Phase 4](Phase4.md)
5. **Import Resolution**: Create a system for mapping import paths to modules - [Phase 2, Task 2.3](Phase2.md#task-23-implement-import-path-mapping-system)

We will leverage existing Go packages wherever possible to minimize reinventing the wheel, while ensuring backward compatibility with existing SProto installations.

## Implementation Phases

This enhancement is broken down into five major phases, each focusing on different aspects of the dependency management system:

1. [**Phase 1: Configuration System and Schema Updates**](Phase1.md) - Laying the foundation with configuration format and database changes
2. [**Phase 2: Dependency Resolution System**](Phase2.md) - Building the core dependency resolution logic
3. [**Phase 3: CLI Enhancements**](Phase3.md) - Improving the command-line interface to support dependency management
4. [**Phase 4: Caching and Directory Structure**](Phase4.md) - Implementing local caching for efficient dependency management
5. [**Phase 5: Testing and Documentation**](Phase5.md) - Ensuring quality and usability through tests and comprehensive documentation
