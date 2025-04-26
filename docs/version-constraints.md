# SProto Version Constraints

This document provides a comprehensive guide to version constraints in SProto's dependency management system.

## Table of Contents

1. [Introduction](#introduction)
2. [Basic Syntax](#basic-syntax)
3. [Operators](#operators)
4. [Combining Constraints](#combining-constraints)
5. [Semantics and Resolution Rules](#semantics-and-resolution-rules)
6. [Best Practices](#best-practices)
7. [Examples](#examples)

## Introduction

Version constraints allow you to specify which versions of a dependency your module is compatible with. SProto uses semantic versioning (SemVer) for all version constraints, providing flexibility and precision in dependency management.

In your `sproto.yaml` file, version constraints are specified in the `version` field of each dependency:

```yaml
dependencies:
  - namespace: mycompany
    name: common
    version: ">=v1.0.0, <v2.0.0"
    import_path: github.com/mycompany/common/proto
```

## Basic Syntax

Version constraints in SProto follow these rules:

- All versions should be prefixed with `v` (e.g., `v1.0.0`)
- Version numbers follow the SemVer format: `vMAJOR.MINOR.PATCH`
- Pre-release versions are supported with `-` suffix (e.g., `v1.0.0-alpha.1`)
- Build metadata is supported with `+` suffix (e.g., `v1.0.0+20230401`)
- Version constraints can use various operators to specify ranges

## Operators

SProto supports the following version constraint operators:

| Operator | Example         | Description                                      |
|----------|-----------------|--------------------------------------------------|
| `=`      | `=v1.0.0`       | Exact version match                              |
| `>`      | `>v1.0.0`       | Greater than the specified version               |
| `>=`     | `>=v1.0.0`      | Greater than or equal to the specified version   |
| `<`      | `<v2.0.0`       | Less than the specified version                  |
| `<=`     | `<=v1.9.9`      | Less than or equal to the specified version      |
| `~`      | `~v1.2.0`       | Patch-level changes allowed (`>=v1.2.0, <v1.3.0`) |
| `^`      | `^v1.2.0`       | Minor and patch-level changes allowed (`>=v1.2.0, <v2.0.0`) |

### Operator Details

- **Exact Match (`=`)**: Requires exactly the specified version. This is the most restrictive constraint.
  - Example: `=v1.2.3` allows only version `v1.2.3`
  - Note: The `=` sign is optional for exact matches, so `v1.2.3` is equivalent to `=v1.2.3`

- **Greater Than (`>`)**: Requires a version higher than the specified version.
  - Example: `>v1.2.3` allows any version above `v1.2.3` (`v1.2.4`, `v1.3.0`, `v2.0.0`, etc.)

- **Greater Than or Equal To (`>=`)**: Requires a version equal to or higher than the specified version.
  - Example: `>=v1.2.3` allows `v1.2.3` and any higher version

- **Less Than (`<`)**: Requires a version lower than the specified version.
  - Example: `<v2.0.0` allows any version below `v2.0.0` (`v1.9.9`, `v1.2.3`, etc.)

- **Less Than or Equal To (`<=`)**: Requires a version equal to or lower than the specified version.
  - Example: `<=v1.9.9` allows `v1.9.9` and any lower version

- **Tilde (`~`)**: Allows patch-level changes but requires the same minor version.
  - Example: `~v1.2.3` is equivalent to `>=v1.2.3, <v1.3.0`
  - This is useful when you want to accept bug fixes but not new features

- **Caret (`^`)**: Allows changes that don't change the leftmost non-zero digit.
  - Example: `^v1.2.3` is equivalent to `>=v1.2.3, <v2.0.0`
  - For `v0.x.y` versions: `^v0.2.3` is equivalent to `>=v0.2.3, <v0.3.0`
  - For `v0.0.x` versions: `^v0.0.3` is equivalent to `>=v0.0.3, <v0.0.4`
  - This is useful when following SemVer, as it allows all updates that should be backwards compatible

## Combining Constraints

Multiple constraints can be combined using commas to further restrict the allowed versions:

```yaml
version: ">=v1.0.0, <v2.0.0"
```

When combining constraints:
- All constraints must be satisfied for a version to be considered compatible
- Separate each constraint with a comma
- Whitespace around commas is optional but recommended for readability

### Common Combinations

- **Allowing a range of versions**: `>=v1.0.0, <v2.0.0` (any version from `v1.0.0` up to but not including `v2.0.0`)
- **Excluding specific versions**: `>=v1.0.0, <v1.2.0, >v1.3.0, <v2.0.0` (any version from `v1.0.0` up to but not including `v2.0.0`, except for versions `v1.2.0` through `v1.3.0`)
- **Combining operators for precise control**: `>v1.2.0, <=v1.5.0` (any version greater than `v1.2.0` and less than or equal to `v1.5.0`)

## Semantics and Resolution Rules

When SProto resolves dependencies, it follows these rules:

1. **Satisfaction**: A version satisfies a constraint if it meets all the conditions in the constraint.
2. **Version Selection**: If multiple versions satisfy the constraints, SProto selects the highest version.
3. **Pre-releases**: Pre-release versions are only selected if explicitly requested in the constraint.
4. **Conflict Resolution**: If conflicting constraints are found (no version satisfies all constraints), SProto reports an error.
5. **Transitive Dependencies**: Constraints from all modules in the dependency graph are considered when resolving a specific dependency.

### Constraint Evaluation Order

1. The exact constraints are evaluated first (`=v1.2.3`)
2. Range constraints are evaluated next (`>`, `>=`, `<`, `<=`)
3. Semantic operators (`~`, `^`) are expanded to their equivalent ranges and then evaluated

## Best Practices

1. **Use version ranges wisely**:
   - Prefer `^v1.0.0` for most dependencies (allows compatible updates)
   - Use `~v1.2.0` when you want only patch updates
   - Use explicit constraints (`=v1.2.3`) for critical dependencies where you need exact control

2. **Avoid overly restrictive constraints**:
   - `=v1.2.3` might lead to dependency conflicts in large projects
   - Consider the impact on transitive dependencies

3. **Follow SemVer conventions**:
   - Major version changes (`v1.0.0` to `v2.0.0`) indicate breaking changes
   - Minor version changes (`v1.1.0` to `v1.2.0`) indicate new features without breaking changes
   - Patch version changes (`v1.1.1` to `v1.1.2`) indicate bug fixes

4. **Use narrow constraint ranges for unstable dependencies**:
   - For modules that don't strictly follow SemVer, use narrower constraints

## Examples

### Basic Dependencies

```yaml
dependencies:
  # Exact version
  - namespace: mycompany
    name: common
    version: "v1.0.0"
    import_path: github.com/mycompany/common/proto

  # Version range with caret (allowing minor and patch updates)
  - namespace: mycompany
    name: auth
    version: "^v1.2.0"
    import_path: github.com/mycompany/auth/proto

  # Version range with tilde (allowing only patch updates)
  - namespace: mycompany
    name: logging
    version: "~v1.3.0"
    import_path: github.com/mycompany/logging/proto
```

### Complex Constraints

```yaml
dependencies:
  # Multiple version constraints
  - namespace: mycompany
    name: api
    version: ">=v1.5.0, <v2.0.0"
    import_path: github.com/mycompany/api/proto

  # Exclude problematic versions
  - namespace: mycompany
    name: database
    version: ">=v1.0.0, !=v1.3.0, <v2.0.0"
    import_path: github.com/mycompany/database/proto

  # Complex range
  - namespace: thirdparty
    name: utils
    version: ">=v2.0.0, <v2.2.0, >=v2.3.1, <v3.0.0"
    import_path: github.com/thirdparty/utils/proto
```

### Pre-release Versions

```yaml
dependencies:
  # Specific pre-release version
  - namespace: mycompany
    name: experimental
    version: "v1.0.0-alpha.2"
    import_path: github.com/mycompany/experimental/proto

  # Range including pre-releases
  - namespace: mycompany
    name: beta
    version: ">=v1.0.0-beta.1, <v1.0.0"
    import_path: github.com/mycompany/beta/proto
