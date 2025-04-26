# Converting Buf Configuration to SProto

This guide provides detailed instructions for converting Buf's `buf.yaml` configuration files to SProto's `sproto.yaml` format.

## Field-by-Field Mapping

| Buf Field (`buf.yaml`) | SProto Field (`sproto.yaml`) | Notes |
|------------------------|------------------------------|-------|
| `version` | `version` | Direct equivalent, both use "v1" |
| `name` | `name` | SProto uses `namespace/name` format without the `buf.build/` prefix |
| (Not present) | `import_path` | **Required in SProto** - Maps to the logical import path root |
| `deps` | `dependencies` | Different structure, see below |
| `build` | (Not supported) | No direct equivalent in SProto |
| `lint` | (Not supported) | No direct equivalent in SProto |
| `breaking` | (Not supported) | No direct equivalent in SProto |

## `deps` to `dependencies` Conversion

Buf's `deps` is a simple array of strings, while SProto's `dependencies` is an array of objects with specific fields:

**Buf Example:**
```yaml
deps:
  - buf.build/googleapis/googleapis
  - buf.build/organization/common
```

**SProto Equivalent:**
```yaml
dependencies:
  - namespace: googleapis
    name: googleapis
    version: "v1.0.0"  # Must specify a version or constraint
    import_path: google/apis  # Must specify an import path
  - namespace: organization
    name: common
    version: ">=v1.0.0, <v2.0.0"
    import_path: github.com/organization/common/proto
```

Key differences:
1. Each dependency requires explicit version information
2. Each dependency requires an import path prefix

## Complete Example

### Before (buf.yaml)

```yaml
version: v1
name: buf.build/acme/petstore
deps:
  - buf.build/googleapis/googleapis
  - buf.build/acme/common
build:
  excludes:
    - tmp/
    - vendor/
lint:
  use:
    - DEFAULT
  except:
    - ENUM_PASCAL_CASE
    - FIELD_LOWER_SNAKE_CASE
breaking:
  use:
    - FILE
```

### After (sproto.yaml)

```yaml
version: v1
name: acme/petstore
import_path: github.com/acme/petstore/proto
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

## Fields Without Direct Equivalents

### Buf-specific fields without SProto equivalents

1. **Build Configuration (`build`)**: 
   - SProto doesn't have equivalent functionality for build exclusions
   - Workaround: Use `.gitignore` or similar for excluding files from publishing

2. **Lint Rules (`lint`)**:
   - SProto doesn't include linting functionality
   - Workaround: Use external protoc plugins or tools for linting

3. **Breaking Change Detection (`breaking`)**:
   - SProto doesn't include breaking change detection
   - Workaround: Use semantic versioning and release notes to indicate breaking changes

### SProto-specific required fields

1. **Import Path (`import_path`)**:
   - Required in SProto, no direct equivalent in Buf
   - Must be specified for both the module and each dependency
   - Typically follows Go import path conventions (e.g., "github.com/organization/module/proto")

2. **Dependency Version Constraints**:
   - SProto requires explicit version constraints for all dependencies
   - Buf typically handles this differently (often via commits or tags)

## Conversion Strategies

### Manual Conversion

For a small number of files, manual conversion is straightforward:

1. Create a new `sproto.yaml` file
2. Copy the `version` field directly
3. Copy the `name` field, removing the "buf.build/" prefix
4. Add an appropriate `import_path` field based on your project's structure
5. Convert each dependency in `deps` to an object in `dependencies` with all required fields

### Automated Conversion

For larger projects, you can use a script (adapt as necessary):

```python
import yaml
import os
import sys

def convert_buf_to_sproto(buf_yaml_path, default_version="v1.0.0"):
    # Read buf.yaml
    with open(buf_yaml_path, 'r') as f:
        buf_config = yaml.safe_load(f)
    
    if not buf_config:
        print(f"Error: Empty or invalid YAML in {buf_yaml_path}")
        return False
    
    # Extract module name, removing the buf.build/ prefix
    module_name = buf_config.get('name', '')
    if module_name.startswith('buf.build/'):
        module_name = module_name[len('buf.build/'):]
    
    # Assume GitHub-style import path - modify as needed for your organization
    import_path_root = f"github.com/{module_name}/proto"
    
    # Build sproto.yaml structure
    sproto_config = {
        "version": buf_config.get("version", "v1"),
        "name": module_name,
        "import_path": import_path_root
    }
    
    # Convert dependencies if present
    if "deps" in buf_config and buf_config["deps"]:
        sproto_config["dependencies"] = []
        for dep in buf_config["deps"]:
            # Extract namespace/name from buf.build/namespace/name
            if dep.startswith('buf.build/'):
                dep = dep[len('buf.build/'):]
            
            parts = dep.split('/')
            if len(parts) == 2:
                namespace, name = parts
                # Create dependency object with required fields
                sproto_config["dependencies"].append({
                    "namespace": namespace,
                    "name": name,
                    "version": default_version,  # Use a default or prompt for specifics
                    "import_path": f"github.com/{namespace}/{name}/proto"  # Assume standard pattern
                })
            else:
                print(f"Warning: Could not parse dependency {dep}, skipping")
    
    # Write sproto.yaml
    sproto_yaml_path = os.path.join(os.path.dirname(buf_yaml_path), "sproto.yaml")
    with open(sproto_yaml_path, 'w') as f:
        yaml.dump(sproto_config, f, default_flow_style=False)
    
    print(f"Converted {buf_yaml_path} to {sproto_yaml_path}")
    return True

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python convert_config.py path/to/buf.yaml [default_version]")
        sys.exit(1)
    
    buf_path = sys.argv[1]
    default_version = sys.argv[2] if len(sys.argv) > 2 else "v1.0.0"
    
    if not os.path.exists(buf_path):
        print(f"Error: File {buf_path} not found")
        sys.exit(1)
    
    if convert_buf_to_sproto(buf_path, default_version):
        print("Conversion successful")
    else:
        print("Conversion failed")
        sys.exit(1)
```

## Important Notes

1. **Import Paths**: The most crucial part of the conversion is setting correct import paths. These must match how your `.proto` files import other files.

2. **Version Constraints**: You'll need to decide on appropriate version constraints for your dependencies. Start with specific versions and adjust as needed.

3. **Testing**: After conversion, test your setup with `protoreg-cli resolve` to ensure dependencies resolve correctly.

4. **Incremental Approach**: For large projects, consider converting and publishing base modules first, then working up to more complex modules with dependencies.
