# SProto YAML Validation Rules

This document describes the validation rules applied to `sproto.yaml` configuration files and explains common error messages you might encounter.

## Table of Contents

1. [Introduction](#introduction)
2. [Field-Specific Requirements](#field-specific-requirements)
   - [Version Field](#version-field)
   - [Name Field](#name-field)
   - [Import Path Field](#import-path-field)
   - [Dependencies Field](#dependencies-field)
3. [Format Requirements](#format-requirements)
4. [Dependency Rules](#dependency-rules)
5. [Common Validation Errors](#common-validation-errors)
6. [How to Fix Common Issues](#how-to-fix-common-issues)

## Introduction

The `sproto.yaml` file is validated both when a module is published and when dependencies are resolved. Validation ensures that the configuration is correct, consistent, and can be processed by the SProto toolchain without errors.

## Field-Specific Requirements

### Version Field

The `version` field specifies the format version of the sproto.yaml configuration:

```yaml
version: v1
```

**Validation Rules**:
- **Required**: This field must be present in every sproto.yaml file.
- **Allowed Values**: Currently, only `"v1"` is supported.
- **Error Message**: `invalid configuration format version, expected "v1"`

### Name Field

The `name` field identifies your module in the SProto registry:

```yaml
name: mycompany/mymodule
```

**Validation Rules**:
- **Required**: This field must be present in every sproto.yaml file.
- **Format**: Must follow the pattern `namespace/module_name`.
- **Namespace Component**:
  - Must be at least 1 character and at most 63 characters long.
  - Can only contain lowercase letters (a-z), numbers (0-9), underscores (_), and hyphens (-).
  - Cannot start or end with underscore or hyphen.
- **Module Name Component**:
  - Same rules as the namespace component.
- **Error Messages**:
  - `name is required`
  - `invalid module name format, expected "namespace/name"`
  - `namespace component must be between 1 and 63 characters`
  - `module name component must be between 1 and 63 characters`
  - `module name components can only contain lowercase letters, numbers, underscores, and hyphens`
  - `module name components cannot start or end with an underscore or hyphen`

### Import Path Field

The `import_path` field defines the base import path prefix for the module:

```yaml
import_path: github.com/mycompany/mymodule/proto
```

**Validation Rules**:
- **Required**: This field must be present in every sproto.yaml file.
- **Format**: Must be a valid path string without leading or trailing slashes.
- **Characters**: Can contain letters, numbers, dots, underscores, hyphens, and forward slashes.
- **Error Messages**:
  - `import_path is required`
  - `import_path cannot be empty`
  - `import_path contains invalid characters`
  - `import_path cannot start or end with a slash`

### Dependencies Field

The `dependencies` field lists the modules that this module depends on:

```yaml
dependencies:
  - namespace: mycompany
    name: common
    version: ">=v1.0.0, <v2.0.0"
    import_path: github.com/mycompany/common/proto
```

**Validation Rules**:
- **Optional**: This field is only required if your module has dependencies.
- **Format**: Must be an array of dependency objects.
- **Each Dependency Object Must Contain**:
  - `namespace`: Subject to the same rules as the namespace component in the name field.
  - `name`: Subject to the same rules as the module name component in the name field.
  - `version`: Must be a valid version constraint (see [Version Constraints](#version-constraints)).
  - `import_path`: Must follow the same rules as the module's import_path field.
- **Error Messages**:
  - `invalid dependencies format, expected array`
  - `dependency must contain namespace, name, version, and import_path fields`
  - Various version constraint validation errors (see below)

## Format Requirements

### YAML Syntax

The sproto.yaml file must be a valid YAML document:

**Validation Rules**:
- Must be a valid YAML document (parseable by a standard YAML parser).
- Root element must be a map/object.
- No duplicate keys are allowed.
- **Error Messages**:
  - `failed to parse YAML content`
  - `invalid YAML structure, expected a map/object at root level`
  - `duplicate key "{key}" found in YAML document`

### Version Constraints

Version constraints in the `version` field of dependencies must follow SemVer notation:

**Validation Rules**:
- Must contain a valid SemVer constraint expression.
- Version numbers must be prefixed with `v` (e.g., `v1.0.0`).
- Valid operators: `=`, `>`, `>=`, `<`, `<=`, `~`, `^`.
- Combined constraints must be separated by commas.
- **Error Messages**:
  - `invalid version constraint: "{constraint}"`
  - `version constraint missing "v" prefix`
  - `unexpected token in version constraint`
  - `unsupported operator in version constraint`
  - `version part is not a valid number`

Refer to [Version Constraints](version-constraints.md) for detailed information about version constraint syntax.

## Dependency Rules

### Dependency Uniqueness

Each dependency must be unique within a module:

**Validation Rules**:
- No duplicate namespace/name pairs allowed in the dependencies list.
- **Error Message**: `duplicate dependency defined: {namespace}/{name}`

### Self-Dependency

A module cannot depend on itself:

**Validation Rules**:
- The namespace/name pair in a dependency cannot match the module's namespace/name.
- **Error Message**: `module cannot depend on itself`

### Import Path Conflicts

Import paths must not conflict with each other:

**Validation Rules**:
- Two dependencies cannot have the same import path.
- An import path cannot be a prefix of another import path.
- **Error Messages**:
  - `conflicting import path: {import_path} used by multiple dependencies`
  - `import path conflict: {import_path1} is a prefix of {import_path2}`

## Common Validation Errors

### File-Level Errors

| Error | Description | Solution |
|-------|-------------|----------|
| `file not found: sproto.yaml` | The sproto.yaml file is missing | Create a sproto.yaml file in the root of your module directory |
| `failed to parse YAML content` | The YAML syntax is invalid | Check your YAML syntax for errors like missing quotes, incorrect indentation, etc. |
| `unknown field: {field}` | There's an unexpected field in the configuration | Remove the unknown field or check for typos in field names |

### Version Field Errors

| Error | Description | Solution |
|-------|-------------|----------|
| `version field is required` | The version field is missing | Add `version: v1` to your sproto.yaml |
| `invalid configuration format version, expected "v1"` | The version field has an invalid value | Change the version field to `v1` |

### Name Field Errors

| Error | Description | Solution |
|-------|-------------|----------|
| `name field is required` | The name field is missing | Add a name field in the format `namespace/name` |
| `invalid module name format, expected "namespace/name"` | The name field doesn't follow the required format | Ensure the name follows the `namespace/name` format |
| `namespace component must be between 1 and 63 characters` | The namespace component is too long | Shorten the namespace component |
| `module name component must be between 1 and 63 characters` | The module name component is too long | Shorten the module name component |
| `module name components can only contain lowercase letters, numbers, underscores, and hyphens` | The name contains invalid characters | Remove invalid characters from the name |
| `module name components cannot start or end with an underscore or hyphen` | The name has leading or trailing special characters | Remove leading or trailing underscores and hyphens |

### Import Path Errors

| Error | Description | Solution |
|-------|-------------|----------|
| `import_path field is required` | The import_path field is missing | Add an import_path field specifying the base import path for your module |
| `import_path cannot be empty` | The import_path field is present but empty | Provide a valid import path for your module |
| `import_path contains invalid characters` | The import path contains characters not allowed | Ensure your import path only uses valid characters |
| `import_path cannot start or end with a slash` | The import path starts or ends with a slash | Remove leading or trailing slashes from the import path |

### Dependencies Errors

| Error | Description | Solution |
|-------|-------------|----------|
| `invalid dependencies format, expected array` | The dependencies field is not an array | Format the dependencies field as an array of dependency objects |
| `dependency must contain namespace, name, version, and import_path fields` | A dependency object is missing required fields | Ensure each dependency has all required fields |
| `duplicate dependency defined: {namespace}/{name}` | There are duplicate dependencies | Remove the duplicate dependency or use the most appropriate one |
| `module cannot depend on itself` | The module lists itself as a dependency | Remove the self-dependency |
| `conflicting import path: {import_path} used by multiple dependencies` | Multiple dependencies use the same import path | Ensure each dependency has a unique import path |

### Version Constraint Errors

| Error | Description | Solution |
|-------|-------------|----------|
| `invalid version constraint: "{constraint}"` | The version constraint syntax is invalid | Check the constraint syntax following SemVer rules |
| `version constraint missing "v" prefix` | A version number doesn't have the `v` prefix | Add the `v` prefix to version numbers (e.g., `v1.0.0`) |
| `unsupported operator in version constraint` | The constraint uses an invalid operator | Use only supported operators: `=`, `>`, `>=`, `<`, `<=`, `~`, `^` |
| `version part is not a valid number` | One of the version components is not a number | Ensure version numbers follow the format `vMAJOR.MINOR.PATCH` |
| `no version satisfies constraints` | Conflicting version constraints exist | Adjust your version constraints to be compatible with each other |

## How to Fix Common Issues

### Empty or Missing Configuration File

If SProto reports that it can't find a `sproto.yaml` file:

1. Create a `sproto.yaml` file in the root directory of your module
2. Include these minimal required fields:
   ```yaml
   version: v1
   name: yournamespace/yourmodule
   import_path: github.com/yournamespace/yourmodule
   ```

### Invalid Module Name

If your module name is invalid:

1. Ensure it follows the `namespace/name` format
2. Check that both the namespace and name:
   - Use only lowercase letters, numbers, underscores, and hyphens
   - Don't start or end with underscores or hyphens
   - Are no longer than 63 characters each

```yaml
# Correct
name: mycompany/user-service

# Incorrect - using uppercase
name: MyCompany/UserService

# Incorrect - starts with hyphen
name: mycompany/-userservice

# Incorrect - missing namespace or name
name: mycompany
```

### Invalid Version Constraints

If your version constraints are invalid:

1. Ensure all version numbers have the `v` prefix
2. Use only supported operators
3. Separate multiple constraints with commas
4. Verify that the constraints are not conflicting

```yaml
# Correct
version: ">=v1.0.0, <v2.0.0"

# Incorrect - missing v prefix
version: ">=1.0.0, <2.0.0"

# Incorrect - unsupported operator
version: "==v1.0.0"

# Incorrect - missing comma separator
version: ">=v1.0.0 <v2.0.0"

# Incorrect - conflicting constraints
version: "<v1.0.0, >=v2.0.0"
```

### Dependency Conflicts

If you have dependency conflicts:

1. Check for duplicate dependencies with the same namespace/name
2. Verify that no dependency has the same namespace/name as your module
3. Ensure import paths don't conflict (one being a prefix of another)

```yaml
# Incorrect - duplicate dependency
dependencies:
  - namespace: mycompany
    name: common
    version: "v1.0.0"
    import_path: github.com/mycompany/common
  - namespace: mycompany
    name: common  # Duplicate namespace/name 
    version: "v2.0.0"
    import_path: github.com/mycompany/common/v2

# Incorrect - import path prefix conflict
dependencies:
  - namespace: mycompany
    name: base
    version: "v1.0.0"
    import_path: github.com/mycompany/utils  # Prefix of the next one
  - namespace: mycompany
    name: extras
    version: "v1.0.0"
    import_path: github.com/mycompany/utils/extras
```

### Unresolvabie Dependencies

If SProto cannot resolve your dependencies:

1. Check that all referenced dependencies actually exist in the registry
2. Verify that the versions you're requiring exist
3. Ensure there are no conflicting version constraints from different dependencies

For example, if module A requires `common ^v1.0.0` and module B requires `common ^v2.0.0`, these constraints conflict and cannot be satisfied simultaneously.

### Import Path Resolution Issues

If imports are not resolving correctly:

1. Ensure the `import_path` in each dependency correctly maps to the actual import paths used in `.proto` files
2. Check that there are no path conflicts between different dependencies
3. Verify that your `.proto` files use the correct import paths

If your proto file imports `"github.com/mycompany/common/user.proto"` but the dependency has `import_path: "github.com/mycompany/common/types"`, the import won't resolve correctly.
