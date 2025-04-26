# SProto Configuration Examples

This document provides practical examples of `sproto.yaml` configuration files for various common scenarios.

## Table of Contents

1. [Basic Module (No Dependencies)](#basic-module-no-dependencies)
2. [Module with a Single Dependency](#module-with-a-single-dependency)
3. [Module with Multiple Dependencies](#module-with-multiple-dependencies)
4. [Using Different Version Constraints](#using-different-version-constraints)
5. [Advanced Configuration Patterns](#advanced-configuration-patterns)

## Basic Module (No Dependencies)

The simplest `sproto.yaml` configuration is for a module that doesn't depend on any other modules:

```yaml
# Basic module configuration
version: v1                        # Configuration format version
name: mycompany/logger             # Module identity in namespace/name format
import_path: github.com/mycompany/logger/proto  # Base import path
```

This configuration defines:
- The configuration format version (`v1`)
- The identity of the module (`mycompany/logger`)
- The import path prefix for all proto files in this module

When a proto file in this module is located at `proto/v1/logger.proto`, other modules would import it as:
```protobuf
import "github.com/mycompany/logger/proto/v1/logger.proto";
```

## Module with a Single Dependency

When your module depends on a single external module:

```yaml
# Configuration with a single dependency
version: v1
name: mycompany/user-service
import_path: github.com/mycompany/user-service/proto

# Dependencies section
dependencies:
  - namespace: mycompany     # The organization/owner of the dependency
    name: common             # The name of the dependency module
    version: "v1.2.0"        # Exact version requirement
    import_path: github.com/mycompany/common/proto  # Import path for the dependency
```

This configuration:
- Defines the module identity (`mycompany/user-service`)
- Specifies a single dependency on `mycompany/common` at exactly version `v1.2.0`
- Maps the import path `github.com/mycompany/common/proto/*` to files in the `mycompany/common` module

With this configuration, when your proto files import from the common module:
```protobuf
import "github.com/mycompany/common/proto/types/user.proto";
```

SProto will:
1. Recognize that this import belongs to the `mycompany/common` dependency
2. Fetch version `v1.2.0` of that module from the registry (if not already cached)
3. Provide the correct include path to protoc during compilation

## Module with Multiple Dependencies

For more complex projects that depend on multiple modules:

```yaml
# Configuration with multiple dependencies
version: v1
name: mycompany/api-gateway
import_path: github.com/mycompany/api-gateway/proto

# Multiple dependencies section
dependencies:
  - namespace: mycompany
    name: user-service
    version: ">=v1.0.0, <v2.0.0"  # Version range
    import_path: github.com/mycompany/user-service/proto
    
  - namespace: mycompany
    name: product-service
    version: "^v1.2.3"  # Caret constraint (>=v1.2.3, <v2.0.0)
    import_path: github.com/mycompany/product-service/proto
    
  - namespace: thirdparty
    name: common-protos
    version: "v0.5.1"  # Exact version
    import_path: github.com/thirdparty/common-protos
```

This configuration:
- Defines dependencies on three different modules
- Uses different version constraint styles for each dependency
- Maps three different import paths to their respective modules

When your proto files contain imports from these dependencies:

```protobuf
import "github.com/mycompany/user-service/proto/v1/user.proto";
import "github.com/mycompany/product-service/proto/v1/product.proto";
import "github.com/thirdparty/common-protos/type/money.proto";
```

SProto will resolve each import to the correct module version.

## Using Different Version Constraints

This example demonstrates various version constraint patterns:

```yaml
# Configuration showcasing different version constraints
version: v1
name: mycompany/demo
import_path: github.com/mycompany/demo/proto

dependencies:
  # Exact version match
  - namespace: mycompany
    name: exact-version
    version: "v1.2.3"  # Only v1.2.3 is acceptable
    import_path: github.com/mycompany/exact-version/proto
    
  # Greater than or equal constraint
  - namespace: mycompany
    name: minimum-version
    version: ">=v2.0.0"  # Any version from v2.0.0 onwards
    import_path: github.com/mycompany/minimum-version/proto
    
  # Version range
  - namespace: mycompany
    name: version-range
    version: ">=v1.0.0, <v2.0.0"  # Any v1.x.x version
    import_path: github.com/mycompany/version-range/proto
    
  # Tilde constraint (patch-level changes only)
  - namespace: mycompany
    name: patch-updates
    version: "~v1.2.0"  # Equivalent to >=v1.2.0, <v1.3.0
    import_path: github.com/mycompany/patch-updates/proto
    
  # Caret constraint (minor and patch-level changes)
  - namespace: mycompany
    name: compatible-updates
    version: "^v1.2.0"  # Equivalent to >=v1.2.0, <v2.0.0
    import_path: github.com/mycompany/compatible-updates/proto
    
  # Complex constraint
  - namespace: mycompany
    name: complex-constraint
    version: ">=v1.0.0, <v1.2.0, >=v1.3.0, <v2.0.0"  # v1.x.x except v1.2.x
    import_path: github.com/mycompany/complex-constraint/proto
```

This configuration demonstrates:
- Different ways to specify version constraints
- How to exclude specific versions
- How to use semantic versioning operators

## Advanced Configuration Patterns

### Microservice Architecture with Shared Base Modules

This example shows a configuration pattern common in microservice architectures where several base/common modules are shared across multiple services:

```yaml
# Gateway Service Configuration
version: v1
name: mycompany/gateway-service
import_path: github.com/mycompany/gateway-service/proto

dependencies:
  # Core infrastructure modules
  - namespace: mycompany
    name: core-types
    version: "^v1.0.0"
    import_path: github.com/mycompany/core-types
    
  - namespace: mycompany
    name: auth
    version: "^v2.0.0"
    import_path: github.com/mycompany/auth/proto
    
  # Dependent service APIs
  - namespace: mycompany
    name: user-service-api
    version: "^v1.0.0"
    import_path: github.com/mycompany/user-service/api
    
  - namespace: mycompany
    name: billing-service-api
    version: "^v1.0.0"
    import_path: github.com/mycompany/billing-service/api
    
  - namespace: mycompany
    name: product-service-api
    version: "^v1.0.0"
    import_path: github.com/mycompany/product-service/api
    
  # External dependencies
  - namespace: googleapis
    name: googleapis
    version: "v1.0.0"
    import_path: google/apis
```

### Monorepo with Multiple Service Modules

For organizations using a monorepo approach with multiple service modules:

```yaml
# Product Service Configuration in a Monorepo
version: v1
name: mycompany/product-service
import_path: github.com/mycompany/monorepo/product-service/proto

dependencies:
  # Internal shared modules (also in the monorepo)
  - namespace: mycompany
    name: shared-types
    version: "v1.0.0"  # Exact versions often used in monorepos
    import_path: github.com/mycompany/monorepo/shared/types
    
  - namespace: mycompany
    name: shared-auth
    version: "v1.0.0"
    import_path: github.com/mycompany/monorepo/shared/auth
    
  # External dependencies
  - namespace: googleapis
    name: googleapis
    version: "v1.0.0"
    import_path: google/apis
```

### Configuration with Comments

Adding comments to your configuration can help document your choices and provide context for other developers:

```yaml
# Main application API configuration
version: v1
name: mycompany/api  # Our public-facing API
import_path: github.com/mycompany/api/proto

dependencies:
  # Core types and utilities
  # These rarely change, so we use exact version to ensure stability
  - namespace: mycompany
    name: core
    version: "v2.3.1"  # Locked to this specific version for stability
    import_path: github.com/mycompany/core/proto
    
  # Business logic modules
  # These modules change more frequently but maintain backward compatibility
  - namespace: mycompany
    name: users
    version: "^v1.5.0"  # Use caret to get compatible updates automatically
    import_path: github.com/mycompany/users/proto
    
  # We're avoiding v1.2.0 due to a critical bug in that specific version
  - namespace: mycompany
    name: billing
    version: ">=v1.0.0, <v1.2.0, >=v1.3.0, <v2.0.0"
    import_path: github.com/mycompany/billing/proto
    
  # External dependencies
  # These are managed by third parties
  - namespace: thirdparty
    name: payment-types
    version: ">=v1.0.0, <v2.0.0"  # Compatible with any v1.x version
    import_path: github.com/thirdparty/payment-types
```

## Key Takeaways

When creating your `sproto.yaml` file, keep these principles in mind:

1. **Clarity**: Use meaningful module names and organize dependencies logically.
2. **Version Constraints**: Choose the right version constraint style for each dependency:
   - Exact versions (`v1.2.3`) for critical dependencies where stability is paramount
   - Range constraints (`>=v1.0.0, <v2.0.0`) for more flexible dependencies
   - Semantic operators (`^` or `~`) for dependencies that follow SemVer properly
3. **Import Paths**: Ensure import paths accurately reflect the structure of your proto files
4. **Comments**: Add descriptive comments to document your configuration choices
5. **Organization**: Group related dependencies together for better readability

By following these patterns, you can create clear, maintainable `sproto.yaml` files that accurately express your module's dependencies and make it easier for others to use your proto files.
