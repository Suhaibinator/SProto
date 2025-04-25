# SProto Configuration File Specification (`sproto.yaml`) - v1

This document specifies the format for the `sproto.yaml` configuration file used by SProto for defining Protobuf modules and their dependencies.

## 1. Overview

The `sproto.yaml` file resides at the root of a Protobuf module directory. It defines metadata about the module, including its name, import path, and dependencies on other SProto modules. This file enables SProto's dependency management features.

## 2. Format Version

The configuration format is versioned to allow for future changes.

-   **`version`** (Required, String): Specifies the version of the `sproto.yaml` schema being used. The current version is `"v1"`.
    ```yaml
    version: v1
    ```

## 3. Module Definition

These fields define the identity and import path of the current module.

-   **`name`** (Required, String): A unique identifier for the module within the SProto registry, following the format `"namespace/name"`.
    -   `namespace`: Typically represents an organization, team, or user.
    -   `name`: The specific name of the module.
    -   Example: `"mycompany/user_service"`
    ```yaml
    name: mycompany/user_service
    ```

-   **`import_path`** (Required, String): The base Go-style import path prefix for the `.proto` files within this module. This allows SProto (and potentially other tools) to correctly resolve `import` statements in Protobuf files.
    -   Example: `"github.com/mycompany/user_service/proto"`
    ```yaml
    import_path: github.com/mycompany/user_service/proto
    ```
    If a `.proto` file within this module is located at `proto/v1/user.proto`, its full import path would be `github.com/mycompany/user_service/proto/v1/user.proto`.

## 4. Dependencies

This section lists other SProto modules that the current module depends on.

-   **`dependencies`** (Optional, Array): A list of module dependencies. If omitted or empty, the module has no declared dependencies.
    ```yaml
    dependencies:
      - namespace: mycompany
        name: common_types
        version: "v1.2.0" # Specific version
        import_path: github.com/mycompany/common_types/proto
      - namespace: external_org
        name: public_apis
        version: ">=v2.0.0, <v3.0.0" # Version constraint
        import_path: github.com/external_org/public_apis/proto
    ```

Each item in the `dependencies` list is an object with the following fields:

-   **`namespace`** (Required, String): The namespace of the dependency module.
-   **`name`** (Required, String): The name of the dependency module.
-   **`version`** (Required, String): The required version or version constraint for the dependency. This follows standard semantic versioning syntax (e.g., `"v1.0.0"`, `">=v1.1.0"`, `"~v1.2.0"`). SProto will use this to resolve the appropriate version from the registry.
-   **`import_path`** (Required, String): The base import path prefix for the dependency module. This is crucial for resolving imports pointing to files within the dependency.

## 5. Examples

### Example 1: Basic Module (No Dependencies)

```yaml
# sproto.yaml
version: v1
name: mycompany/billing_service
import_path: github.com/mycompany/billing_service/proto
```

### Example 2: Module with a Single, Specific Dependency

```yaml
# sproto.yaml
version: v1
name: mycompany/order_service
import_path: github.com/mycompany/order_service/proto
dependencies:
  - namespace: mycompany
    name: user_service # Depends on user_service
    version: "v1.0.0"   # Requires exactly v1.0.0
    import_path: github.com/mycompany/user_service/proto
```

### Example 3: Module with Multiple Dependencies and Constraints

```yaml
# sproto.yaml
version: v1
name: mycompany/api_gateway
import_path: github.com/mycompany/api_gateway/proto
dependencies:
  - namespace: mycompany
    name: user_service
    version: ">=v1.0.0, <v2.0.0" # Requires v1.x.x
    import_path: github.com/mycompany/user_service/proto
  - namespace: mycompany
    name: product_service
    version: "~v2.1.0" # Requires >=v2.1.0, <v2.2.0
    import_path: github.com/mycompany/product_service/proto
  - namespace: external_org
    name: common_protos
    version: "v0.5.1"
    import_path: github.com/external_org/common_protos/proto
```

## 6. Comparison with `buf.yaml`

This `sproto.yaml` format draws inspiration from `buf.yaml` but is simplified for SProto's specific goals:

-   **Similarities:**
    -   Versioned configuration (`version`).
    -   Module naming (`name`).
    -   Dependency declaration (`deps` in Buf, `dependencies` here).
-   **Differences:**
    -   **Import Path:** `sproto.yaml` explicitly requires `import_path` for both the module itself and its dependencies. Buf often infers this or relies on different mechanisms. SProto makes it explicit to simplify resolution.
    -   **Build/Lint/Breaking:** `sproto.yaml` currently focuses *only* on module identity and dependencies. It does *not* include build settings (`build`), lint rules (`lint`), or breaking change detection (`breaking`) like `buf.yaml`. These could potentially be added in future versions if needed.
    -   **Dependency Syntax:** The structure within `dependencies` is slightly different from Buf's `deps`, requiring `namespace`, `name`, `version`, and `import_path` explicitly for each dependency.

## 7. JSON Schema for Validation

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "SProto Configuration Schema v1",
  "description": "Schema for sproto.yaml configuration file (version v1)",
  "type": "object",
  "properties": {
    "version": {
      "description": "Schema version identifier",
      "type": "string",
      "const": "v1"
    },
    "name": {
      "description": "Module identifier (namespace/name)",
      "type": "string",
      "pattern": "^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$"
    },
    "import_path": {
      "description": "Base Go-style import path for the module's .proto files",
      "type": "string",
      "pattern": "^[a-zA-Z0-9_./-]+$"
      // A more robust regex might be needed depending on allowed chars in paths
    },
    "dependencies": {
      "description": "List of module dependencies",
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "namespace": {
            "description": "Namespace of the dependency module",
            "type": "string",
            "pattern": "^[a-zA-Z0-9_-]+$"
          },
          "name": {
            "description": "Name of the dependency module",
            "type": "string",
            "pattern": "^[a-zA-Z0-9_-]+$"
          },
          "version": {
            "description": "Required version or constraint (SemVer)",
            "type": "string",
            // Basic pattern, actual validation uses SemVer library
            "pattern": "^[><=~^]?v?[0-9]+(\\.[0-9]+){0,2}(-[a-zA-Z0-9-]+(\\.[a-zA-Z0-9-]+)*)?(\\+[a-zA-Z0-9-]+(\\.[a-zA-Z0-9-]+)*)?(,\\s*[><=~^]?v?[0-9]+(\\.[0-9]+){0,2}(-[a-zA-Z0-9-]+(\\.[a-zA-Z0-9-]+)*)?(\\+[a-zA-Z0-9-]+(\\.[a-zA-Z0-9-]+)*)?)*$"
          },
          "import_path": {
            "description": "Base Go-style import path for the dependency module",
            "type": "string",
            "pattern": "^[a-zA-Z0-9_./-]+$"
          }
        },
        "required": [
          "namespace",
          "name",
          "version",
          "import_path"
        ],
        "additionalProperties": false
      },
      "uniqueItems": true // Should ideally check uniqueness based on namespace/name combo
    }
  },
  "required": [
    "version",
    "name",
    "import_path"
  ],
  "additionalProperties": false
}
```
*(Note: The JSON schema provides basic structural validation. More complex validation, like SemVer constraint syntax and import path validity, will be handled by the Go parser implementation.)*
