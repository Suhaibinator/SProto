# SProto Usage Examples

This document provides practical examples of common SProto workflows, including module creation, dependency management, and compilation.

## Table of Contents

1. [Basic Module Publishing](#basic-module-publishing)
2. [Working with Dependencies](#working-with-dependencies)
3. [Dependency Resolution Workflow](#dependency-resolution-workflow)
4. [Compilation with Dependencies](#compilation-with-dependencies)
5. [Cache Management](#cache-management)

## Basic Module Publishing

This section walks through the process of creating and publishing a simple Protobuf module to SProto.

### Step 1: Create Your Proto Files

First, create a directory structure for your proto files. For a basic module without dependencies, structure it like this:

```
my-module/
├── proto/
│   ├── v1/
│   │   ├── user.proto
│   │   └── service.proto
│   └── common/
│       └── types.proto
└── sproto.yaml
```

Create a simple proto file, for example `proto/v1/user.proto`:

```protobuf
syntax = "proto3";

package mycompany.user.v1;

// Import internal type from the same module
import "github.com/mycompany/user/proto/common/types.proto";

option go_package = "github.com/mycompany/user/proto/gen/go/user/v1;userv1";

// User represents a user in the system
message User {
  string id = 1;
  string email = 2;
  string name = 3;
  UserType type = 4;
  int64 created_at = 5;
  int64 updated_at = 6;
}

// UserService provides operations for managing users
service UserService {
  // GetUser retrieves a user by ID
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  
  // ListUsers retrieves a list of users
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
}

// GetUserRequest is the request for GetUser
message GetUserRequest {
  string id = 1;
}

// GetUserResponse is the response from GetUser
message GetUserResponse {
  User user = 1;
}

// ListUsersRequest is the request for ListUsers
message ListUsersRequest {
  int32 page_size = 1;
  string page_token = 2;
}

// ListUsersResponse is the response from ListUsers
message ListUsersResponse {
  repeated User users = 1;
  string next_page_token = 2;
}
```

Create the referenced `proto/common/types.proto` file:

```protobuf
syntax = "proto3";

package mycompany.user.common;

option go_package = "github.com/mycompany/user/proto/gen/go/common;common";

// UserType represents the type of user in the system
enum UserType {
  USER_TYPE_UNSPECIFIED = 0;
  USER_TYPE_ADMIN = 1;
  USER_TYPE_REGULAR = 2;
  USER_TYPE_GUEST = 3;
}
```

### Step 2: Create Configuration File

Create a `sproto.yaml` file in the module's root directory:

```yaml
version: v1
name: mycompany/user
import_path: github.com/mycompany/user/proto
```

This basic configuration specifies:
- The configuration format version (`v1`)
- The module's identity (`mycompany/user`)
- The base import path for proto files in this module

### Step 3: Configure the CLI

Ensure your CLI is configured to communicate with your SProto registry:

```bash
# Configure the CLI to use your registry
protoreg-cli configure --registry-url http://your-registry.example.com:8080 --api-token your-api-token
```

### Step 4: Publish the Module

Use the `publish` command to upload your module to the registry:

```bash
# Navigate to your module directory
cd my-module

# Publish the module with a version
protoreg-cli publish . --module mycompany/user --version v1.0.0
```

Upon successful publishing, you'll see output similar to:

```
Successfully published module mycompany/user version v1.0.0
Artifact digest: sha256:a1b2c3d4e5f6...
```

### Step 5: Verify the Publishing

You can verify that your module was published by listing the available versions:

```bash
# List all versions of the module
protoreg-cli list mycompany/user
```

Expected output:

```
Module: mycompany/user
Available versions:
- v1.0.0 (latest)
```

### Complete Publishing Workflow Example

Here's the complete sequence of commands for publishing a module:

```bash
# Create module directory
mkdir -p my-module/proto/v1 my-module/proto/common

# Create proto files (using your favorite editor)
# ...

# Create sproto.yaml (using your favorite editor)
# ...

# Configure the CLI (if not already configured)
protoreg-cli configure --registry-url http://localhost:8080 --api-token your-api-token

# Publish the module
cd my-module
protoreg-cli publish . --module mycompany/user --version v1.0.0

# Verify the module was published
protoreg-cli list mycompany/user
```

### Common Issues with Publishing

**1. Authentication Errors**

```
Error: Unauthorized: invalid or missing API token
```

Solution: Ensure you've configured your CLI with the correct API token using the `configure` command.

**2. Module Already Exists**

```
Error: Module version already exists: mycompany/user v1.0.0
```

Solution: Each version is immutable. You need to use a new version number if you want to publish an updated version.

**3. Invalid Module Name**

```
Error: Invalid module name format, expected "namespace/name"
```

Solution: Ensure your module name follows the required format with a namespace and name separated by a slash.

**4. Proto Import Issues**

If your proto files use imports that don't follow your `import_path` structure, you might encounter issues when others use your module. Make sure all imports are consistent with the `import_path` specified in your `sproto.yaml`.

## Working with Dependencies

This section demonstrates how to create a module that depends on other modules and how to reference types from those dependencies.

### Step 1: Identify Your Dependencies

Before creating a module with dependencies, identify which modules you'll need to depend on. For this example, we'll create a `mycompany/order-service` module that depends on two existing modules:

1. `mycompany/user` - Contains user-related message types (from the previous example)
2. `mycompany/common` - Contains common types and utilities

### Step 2: Create Module Structure

Create a directory structure for your module with dependencies:

```
order-service/
├── proto/
│   └── v1/
│       ├── order.proto
│       └── service.proto
└── sproto.yaml
```

### Step 3: Define Dependencies in sproto.yaml

Create a `sproto.yaml` file that includes your dependencies:

```yaml
version: v1
name: mycompany/order-service
import_path: github.com/mycompany/order-service/proto

dependencies:
  - namespace: mycompany
    name: user
    version: "^v1.0.0"  # Accept any v1.x.x version
    import_path: github.com/mycompany/user/proto
    
  - namespace: mycompany
    name: common
    version: "v2.1.0"  # Require exactly this version
    import_path: github.com/mycompany/common/proto
```

This configuration:
- Specifies the identity of your module (`mycompany/order-service`)
- Defines the base import path for your proto files
- Declares dependencies on two other modules with their import paths and version constraints

### Step 4: Create Proto Files with References to Dependencies

Now create your proto file that references types from your dependencies:

**proto/v1/order.proto**:

```protobuf
syntax = "proto3";

package mycompany.order.v1;

// Import from dependencies
import "github.com/mycompany/user/proto/v1/user.proto";
import "github.com/mycompany/common/proto/types/money.proto";

option go_package = "github.com/mycompany/order-service/proto/gen/go/order/v1;orderv1";

// Order represents a customer order
message Order {
  string id = 1;
  mycompany.user.v1.User customer = 2;  // Reference User from user dependency
  repeated OrderItem items = 3;
  mycompany.common.types.Money total = 4;  // Reference Money from common dependency
  int64 created_at = 5;
  OrderStatus status = 6;
}

message OrderItem {
  string product_id = 1;
  string name = 2;
  int32 quantity = 3;
  mycompany.common.types.Money unit_price = 4;  // Reference Money again
}

enum OrderStatus {
  ORDER_STATUS_UNSPECIFIED = 0;
  ORDER_STATUS_PENDING = 1;
  ORDER_STATUS_PROCESSING = 2;
  ORDER_STATUS_SHIPPED = 3;
  ORDER_STATUS_DELIVERED = 4;
  ORDER_STATUS_CANCELLED = 5;
}
```

**proto/v1/service.proto**:

```protobuf
syntax = "proto3";

package mycompany.order.v1;

import "github.com/mycompany/order-service/proto/v1/order.proto";
import "github.com/mycompany/user/proto/v1/user.proto";

option go_package = "github.com/mycompany/order-service/proto/gen/go/order/v1;orderv1";

// OrderService provides operations for managing orders
service OrderService {
  // CreateOrder creates a new order for a user
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
  
  // GetOrder retrieves an order by ID
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
  
  // ListUserOrders lists all orders for a user
  rpc ListUserOrders(ListUserOrdersRequest) returns (ListUserOrdersResponse);
}

message CreateOrderRequest {
  mycompany.user.v1.User user = 1;  // Use User from dependency
  repeated OrderItem items = 2;
}

message CreateOrderResponse {
  Order order = 1;
}

message GetOrderRequest {
  string order_id = 1;
}

message GetOrderResponse {
  Order order = 1;
}

message ListUserOrdersRequest {
  string user_id = 1;
  int32 page_size = 2;
  string page_token = 3;
}

message ListUserOrdersResponse {
  repeated Order orders = 1;
  string next_page_token = 2;
}
```

### Step 5: Resolve Dependencies Before Publishing

Before publishing, resolve your dependencies to ensure they exist and are compatible:

```bash
cd order-service
protoreg-cli resolve
```

This command:
1. Reads your `sproto.yaml` file
2. Contacts the registry to find compatible versions of your dependencies
3. Downloads the dependencies to your local cache
4. Validates that import paths can be resolved correctly

If all dependencies resolve successfully, you'll see output similar to:

```
Resolving dependencies...
✅ mycompany/user@v1.2.1 (resolved from ^v1.0.0)
✅ mycompany/common@v2.1.0 (exact version)
All dependencies resolved successfully.
```

### Step 6: Publish Module with Dependencies

Now publish your module with its dependency information:

```bash
protoreg-cli publish . --module mycompany/order-service --version v1.0.0
```

Upon successful publishing, the registry will store not just your proto files but also the dependency information, so others can automatically resolve the same dependencies when they use your module.

### Key Points About Working with Dependencies

1. **Import Paths**: The import paths in your proto files must match the `import_path` fields defined in your `sproto.yaml` file.

2. **Version Constraints**: Choose appropriate version constraints based on your needs:
   - `v1.2.3` (exact version) - When you need exactly this version
   - `^v1.0.0` (caret constraint) - When you want compatible updates (any v1.x.x)
   - `~v1.2.0` (tilde constraint) - When you want only patch updates (any v1.2.x)
   - `>=v1.0.0, <v2.0.0` (range constraint) - For more precise control

3. **Resolution Order**: When multiple modules depend on the same module with different version constraints, SProto will attempt to find a version that satisfies all constraints.

4. **Transitive Dependencies**: If your dependencies have their own dependencies, those will be resolved automatically when you run `protoreg-cli resolve` or `protoreg-cli fetch` with the `--with-deps` flag.

5. **Dependency Verification**: Always run `protoreg-cli resolve` before publishing to verify that your dependencies can be resolved correctly.

### Common Issues with Dependencies

**1. Dependency Not Found**

```
Error: Unable to resolve dependency mycompany/user@^v1.0.0: module not found
```

Solution: Ensure the dependency has been published to the registry and that you've specified the correct namespace and name.

**2. No Compatible Version**

```
Error: No version of mycompany/common satisfies the constraint ^v3.0.0
```

Solution: Check that a compatible version exists in the registry or adjust your constraint.

**3. Conflicting Constraints**

```
Error: Conflicting version constraints for mycompany/common:
- ^v1.0.0 (from mycompany/order-service)
- ^v2.0.0 (from mycompany/user)
```

Solution: Adjust your constraints to be compatible or update your dependencies to versions that have compatible constraints.

**4. Import Path Resolution Issues**

```
Error: Could not resolve import "github.com/mycompany/user/proto/user.proto"
```

Solution: Verify that the import path matches the `import_path` in your dependency's configuration and that the file exists in the dependency module.

## Dependency Resolution Workflow

This section covers the process of resolving dependencies using the `resolve` command and various flags to control the resolution behavior.

### Understanding Dependency Resolution

Dependency resolution is the process of:
1. Reading your module's `sproto.yaml` to identify dependencies
2. Contacting the registry to find module versions that satisfy your constraints
3. Building a dependency graph including transitive dependencies
4. Resolving any version conflicts
5. Downloading and caching the selected modules
6. Setting up the correct import path mappings

### Basic Resolution Workflow

The simplest form of dependency resolution is run from a directory containing a `sproto.yaml` file:

```bash
cd my-project
protoreg-cli resolve
```

This command:
- Reads dependencies from the `sproto.yaml` file
- Resolves and downloads all dependencies (direct and transitive)
- Stores them in the local cache (`~/.cache/sproto` by default)
- Creates import path mappings for use with protoc

Sample output:

```
Resolving dependencies...
✅ mycompany/common@v2.1.0 (exact version)
✅ mycompany/user@v1.2.3 (resolved from ^v1.0.0)
  ⮑ mycompany/auth@v1.0.1 (transitive dependency from mycompany/user)
All dependencies resolved successfully.
```

### Using the --dry-run Flag

If you want to see which dependencies would be resolved without actually downloading them:

```bash
protoreg-cli resolve --dry-run
```

This outputs the resolution plan without modifying the cache:

```
Dependency resolution plan (dry run):
✅ mycompany/common@v2.1.0 (exact version)
✅ mycompany/user@v1.2.3 (resolved from ^v1.0.0)
  ⮑ mycompany/auth@v1.0.1 (transitive dependency from mycompany/user)
No modules were downloaded (dry run).
```

### Using the --update Flag

By default, if a module is already cached, SProto will use the cached version. To force SProto to check for newer versions that still satisfy your constraints:

```bash
protoreg-cli resolve --update
```

This will:
- Check the registry for the newest versions that satisfy your constraints
- Update your local cache with these versions, even if you already have a version cached
- Useful when dependencies have released new patch or minor versions

Sample output when updates are available:

```
Resolving dependencies with update flag...
🔄 mycompany/common@v2.1.0 → v2.1.2 (updated)
🔄 mycompany/user@v1.2.3 → v1.3.0 (updated)
  ⮑ mycompany/auth@v1.0.1 → v1.0.3 (transitive dependency updated)
All dependencies resolved successfully.
```

### Resolving with Specific Modules

You can also resolve dependencies for a specific module without a local `sproto.yaml`:

```bash
protoreg-cli resolve mycompany/order-service@v1.0.0
```

This is useful for:
- Exploring a module's dependencies without having it in your project
- Pre-caching dependencies before using them
- Verifying version compatibility

### Resolving with Custom Cache Location

By default, resolved modules are stored in `~/.cache/sproto`, but you can specify a custom location:

```bash
protoreg-cli resolve --cache-dir /path/to/custom/cache
```

This is useful for:
- Project-specific caches
- CI/CD environments where `~/.cache` might not be accessible
- Sharing a cache between multiple projects

### Verbose Resolution Output

For more detailed information about the resolution process:

```bash
protoreg-cli resolve --verbose
```

This produces detailed output about each step of the resolution process:

```
Verbose resolution activated...
Loading module configuration from sproto.yaml...
Building dependency graph...
- Adding mycompany/common@v2.1.0 to graph
- Adding mycompany/user@^v1.0.0 to graph
- Fetching available versions for mycompany/user...
  - Found versions: v1.0.0, v1.1.0, v1.2.0, v1.2.1, v1.2.3, v1.3.0
  - Selected v1.3.0 (latest matching ^v1.0.0)
- Resolving transitive dependencies for mycompany/user@v1.3.0...
  - Found dependency: mycompany/auth@v1.0.3
- Adding mycompany/auth@v1.0.3 to graph
- Checking for cycles in dependency graph... None found.
- Determining resolution order: common → auth → user
Downloading modules...
- Downloading mycompany/common@v2.1.0...
- Downloading mycompany/auth@v1.0.3...
- Downloading mycompany/user@v1.3.0...
Setting up import path mappings...
- github.com/mycompany/common/proto → ~/.cache/sproto/mycompany/common/v2.1.0
- github.com/mycompany/auth/proto → ~/.cache/sproto/mycompany/auth/v1.0.3
- github.com/mycompany/user/proto → ~/.cache/sproto/mycompany/user/v1.3.0
Resolution completed successfully.
```

### Resolving with Different Version Constraints

Let's see examples of resolving dependencies with different types of version constraints:

#### Exact Versions

```yaml
dependencies:
  - namespace: mycompany
    name: common
    version: "v2.1.0"  # Exact version
    import_path: github.com/mycompany/common/proto
```

```
Resolving dependencies...
✅ mycompany/common@v2.1.0 (exact version)
```

#### Caret Constraint (^)

```yaml
dependencies:
  - namespace: mycompany
    name: user
    version: "^v1.2.0"  # v1.2.0 or later, but less than v2.0.0
    import_path: github.com/mycompany/user/proto
```

```
Resolving dependencies...
✅ mycompany/user@v1.3.5 (resolved from ^v1.2.0)
```

#### Tilde Constraint (~)

```yaml
dependencies:
  - namespace: mycompany
    name: logging
    version: "~v2.1.0"  # v2.1.0 or later, but less than v2.2.0
    import_path: github.com/mycompany/logging/proto
```

```
Resolving dependencies...
✅ mycompany/logging@v2.1.3 (resolved from ~v2.1.0)
```

#### Range Constraint

```yaml
dependencies:
  - namespace: mycompany
    name: config
    version: ">=v1.0.0, <v2.0.0"  # v1.x.x
    import_path: github.com/mycompany/config/proto
```

```
Resolving dependencies...
✅ mycompany/config@v1.5.2 (resolved from >=v1.0.0, <v2.0.0)
```

### Handling Resolution Conflicts

When dependencies have conflicting version constraints, SProto will attempt to find a version that satisfies all constraints. If no such version exists, it will report an error:

```
Resolving dependencies...
❌ Error: Unable to resolve dependencies due to version conflicts:
  - mycompany/common:
    - ^v1.0.0 (from mycompany/order-service)
    - ^v2.0.0 (from mycompany/payment)
  No version can satisfy both constraints.
```

To resolve this, you have several options:

1. **Adjust Your Constraints**: Change your direct dependency constraints to be compatible with transitive dependencies.

2. **Update Dependencies**: Contact the owners of your dependencies to update their constraints.

3. **Fork Dependencies**: In extreme cases, you might need to fork a dependency to make it compatible with your constraints.

### Best Practices for Dependency Resolution

1. **Use Flexible Constraints**: Prefer `^` or range constraints over exact versions to allow for compatible updates.

2. **Regular Updates**: Run `resolve --update` periodically to get bug fixes and security updates.

3. **Consistent Constraints**: Use consistent constraint patterns across projects to avoid conflicts.

4. **Pre-Resolution in CI/CD**: Run `resolve --dry-run` in CI to catch dependency conflicts early.

5. **Version Pinning**: For production releases, consider documenting the exact versions that were resolved to ensure reproducibility.

## Compilation with Dependencies

This section explains how to use SProto's `compile` command, which simplifies the process of compiling Protobuf files with properly configured include paths for dependencies.

### Understanding the Compile Command

The `compile` command is a wrapper around `protoc` (the Protobuf compiler) that automatically:

1. Resolves dependencies from your `sproto.yaml` file
2. Sets up the correct `--proto_path` arguments based on import mappings
3. Passes through other arguments to `protoc`
4. Handles compilation with all necessary dependencies available

This eliminates the error-prone process of manually configuring include paths when working with Protobuf dependencies.

### Basic Compilation

The simplest form of compilation uses the `compile` command in a directory containing a `sproto.yaml` file:

```bash
cd my-project
protoreg-cli compile
```

This command:
- Resolves all dependencies defined in `sproto.yaml`
- Sets up the correct include paths
- Runs `protoc` on all `.proto` files in the current directory

### Specifying Output Format

More commonly, you'll want to generate code in a specific language format:

```bash
# Generate Go code
protoreg-cli compile --go_out=paths=source_relative:./gen

# Generate gRPC Go code
protoreg-cli compile --go_out=paths=source_relative:./gen --go-grpc_out=paths=source_relative:./gen
```

The `compile` command passes these output arguments directly to `protoc`. It supports any output plugins that you have installed for protoc.

### Specifying Input Files

By default, `compile` processes all `.proto` files in the current directory. You can specify certain files instead:

```bash
# Compile specific proto files
protoreg-cli compile api.proto service.proto --go_out=./gen
```

### Additional Include Paths

While SProto automatically configures include paths for your dependencies, you might need to add additional include paths:

```bash
# Add extra include path
protoreg-cli compile --proto_path=./extra-protos --go_out=./gen
```

### Compilation Process

Here's what happens when you run `protoreg-cli compile`:

1. **Dependency Resolution**: SProto first resolves and downloads all dependencies defined in `sproto.yaml` (if they're not already cached).

2. **Import Path Mapping**: It creates mappings from import paths (e.g., `github.com/mycompany/common/proto`) to local filesystem paths (e.g., `~/.cache/sproto/mycompany/common/v1.2.3`).

3. **Protoc Command Construction**: It builds a `protoc` command with the correct `--proto_path` arguments for:
   - The current directory
   - Each dependency's location in the cache
   - Any additional include paths you've specified

4. **Command Execution**: It executes the constructed `protoc` command with your specified output arguments.

Here's an example of the equivalent `protoc` command that might be generated:

```bash
protoc --proto_path=. \
  --proto_path=/home/user/.cache/sproto/mycompany/common/v1.2.3 \
  --proto_path=/home/user/.cache/sproto/mycompany/user/v2.1.0 \
  --go_out=paths=source_relative:./gen \
  --go-grpc_out=paths=source_relative:./gen \
  api.proto service.proto
```

### Compilation Examples

Let's look at some examples for different output formats:

#### Go Code Generation

```bash
# Generate Go code
protoreg-cli compile --go_out=paths=source_relative:./gen

# With Go gRPC service generation
protoreg-cli compile --go_out=paths=source_relative:./gen --go-grpc_out=paths=source_relative:./gen
```

#### Java Code Generation

```bash
# Generate Java code
protoreg-cli compile --java_out=./gen-java
```

#### Python Code Generation

```bash
# Generate Python code
protoreg-cli compile --python_out=./gen-python
```

#### TypeScript Code Generation

```bash
# Using protobuf-ts plugin
protoreg-cli compile --ts_out=./gen-ts
```

#### Multiple Output Formats

```bash
# Generate code for multiple languages
protoreg-cli compile \
  --go_out=./gen-go \
  --java_out=./gen-java \
  --python_out=./gen-python
```

### Viewing the Generated Command

If you want to see the `protoc` command that would be executed without actually running it:

```bash
protoreg-cli compile --dry-run --go_out=./gen
```

This outputs the full command that would be executed:

```
The following command would be executed (dry run):
protoc --proto_path=. --proto_path=/home/user/.cache/sproto/mycompany/common/v1.2.3 --proto_path=/home/user/.cache/sproto/mycompany/user/v2.1.0 --go_out=./gen api.proto service.proto
```

### Integration with Protoc Plugins

SProto's `compile` command works with any protoc plugins that are installed on your system:

1. **Standard Plugins**: The built-in language generators (go, java, python, etc.) work automatically.

2. **Third-Party Plugins**: Any plugin available in your PATH will work with the same syntax as raw protoc:

   ```bash
   # Using grpc-gateway plugin
   protoreg-cli compile \
     --go_out=paths=source_relative:./gen \
     --go-grpc_out=paths=source_relative:./gen \
     --grpc-gateway_out=paths=source_relative:./gen
   ```

3. **Custom Plugin Options**: Plugin-specific options work the same as with raw protoc:

   ```bash
   # With custom plugin options
   protoreg-cli compile \
     --go_out=paths=source_relative,plugins=grpc:./gen \
     --validate_out=lang=go,paths=source_relative:./gen
   ```

### Handling Well-Known Types

SProto automatically includes the well-known types that come with your protoc installation (like `google/protobuf/timestamp.proto`). You don't need to specify include paths for these.

### Compile Command in CI/CD

Using the `compile` command in CI/CD pipelines ensures consistent compilation across environments:

```bash
# Example CI/CD script
set -e
protoreg-cli resolve  # Ensure dependencies are downloaded
protoreg-cli compile \
  --go_out=paths=source_relative:./gen \
  --go-grpc_out=paths=source_relative:./gen
```

### Best Practices for Compilation

1. **Provide Output Directories**: Always specify output directories that are separate from your source files.

2. **Use `paths=source_relative`**: For Go and similar language plugins, using source-relative paths often leads to a cleaner output structure.

3. **Run `resolve` Before Compilation**: In scripts or CI/CD pipelines, explicitly run `resolve` before `compile` to ensure all dependencies are available.

4. **Keep Your Plugins Updated**: Ensure your protoc plugins are compatible with the protoc version you're using.

5. **Create Compilation Scripts**: For complex projects, consider creating a script with your compilation command for consistency.

## Cache Management

This section explores SProto's caching system, which stores resolved dependencies locally to improve performance and enable offline work.

### Understanding the Cache System

SProto stores downloaded module artifacts in a local cache directory, which:

1. **Improves Performance**: Avoids repeatedly downloading the same modules
2. **Enables Offline Work**: Allows you to work without registry access once dependencies are cached
3. **Reduces Registry Load**: Minimizes unnecessary requests to the registry server 

The cache is used automatically when you run `resolve`, `compile`, or `fetch --with-deps` commands. By default, resolved dependencies are stored in `~/.cache/sproto`.

### Cache Commands

SProto provides several commands to manage the cache:

#### Listing Cached Modules

To see what modules are currently cached:

```bash
protoreg-cli cache list
```

Sample output:

```
Cached Modules:
- mycompany/common@v2.1.0
- mycompany/user@v1.3.0
- mycompany/auth@v1.0.3
- googleapis/googleapis@v1.0.0
```

Adding the `--verbose` flag provides more details:

```bash
protoreg-cli cache list --verbose
```

```
Cached Modules:
- mycompany/common@v2.1.0
  Location: /home/user/.cache/sproto/mycompany/common/v2.1.0
  Size: 45.2 KB
  Downloaded: 2025-04-20 14:30:45

- mycompany/user@v1.3.0
  Location: /home/user/.cache/sproto/mycompany/user/v1.3.0
  Size: 128.7 KB
  Downloaded: 2025-04-22 09:15:32
  
- mycompany/auth@v1.0.3
  Location: /home/user/.cache/sproto/mycompany/auth/v1.0.3
  Size: 37.8 KB
  Downloaded: 2025-04-22 09:15:33
  
- googleapis/googleapis@v1.0.0
  Location: /home/user/.cache/sproto/googleapis/googleapis/v1.0.0
  Size: 2.4 MB
  Downloaded: 2025-04-19 16:45:21
```

#### Cleaning the Cache

To remove modules from the cache, use the `clean` command. You can clean specific modules or the entire cache:

```bash
# Clean the entire cache
protoreg-cli cache clean

# Clean a specific module (all versions)
protoreg-cli cache clean mycompany/user

# Clean a specific module version
protoreg-cli cache clean mycompany/user@v1.3.0
```

When cleaning the entire cache, you'll be prompted for confirmation:

```
This will remove all cached modules from /home/user/.cache/sproto
Are you sure you want to continue? [y/N]: y
Cache cleaned successfully.
```

#### Checking Cache Location

To check where the cache is located:

```bash
protoreg-cli cache location
```

```
Cache location: /home/user/.cache/sproto
```

#### Using a Custom Cache Location

You can specify a different cache location:

```bash
# For a single command
protoreg-cli resolve --cache-dir /path/to/custom/cache

# For all commands in a session
export PROTOREG_CACHE_DIR=/path/to/custom/cache
protoreg-cli resolve
```

### Cache Directory Structure

The cache follows a structured directory format:

```
~/.cache/sproto/
├── <namespace>/
│   └── <module name>/
│       └── <version>/
│           ├── module.zip      # Original zip artifact
│           └── extract/        # Extracted content
│               └── <files>
├── mappings/
│   └── import_mappings.json    # Import path mappings
└── metadata/
    └── module_versions.json    # Cache metadata
```

For example:

```
~/.cache/sproto/
├── mycompany/
│   ├── common/
│   │   └── v2.1.0/
│   │       ├── module.zip
│   │       └── extract/
│   │           └── proto/
│   │               └── types/
│   │                   ├── primitive.proto
│   │                   └── money.proto
│   └── user/
│       └── v1.3.0/
│           ├── module.zip
│           └── extract/
│               └── proto/
│                   └── v1/
│                       └── user.proto
└── mappings/
    └── import_mappings.json
```

### Common Cache Operations

Let's look at some common cache workflows:

#### Priming the Cache

Before working offline, you can prime the cache with all necessary dependencies:

```bash
# Using sproto.yaml in current directory
protoreg-cli resolve

# For a specific module
protoreg-cli fetch mycompany/service@v1.0.0 --with-deps --output /tmp/temp-extract
rm -rf /tmp/temp-extract  # Optionally remove the extracted files, keeping just the cache
```

#### Force Updating Cached Modules

When you want to refresh cached modules with the latest versions:

```bash
protoreg-cli resolve --update
```

When you run `resolve` with the `--update` flag, SProto will:
1. Check the registry for available versions
2. Download newer versions that match your constraints
3. Replace the existing cached versions

#### Dealing with Corrupt Cache

If your cache becomes corrupted:

```bash
# Remove the problematic module
protoreg-cli cache clean mycompany/problem-module

# Or clean the entire cache
protoreg-cli cache clean

# Then resolve again
protoreg-cli resolve
```

#### Sharing Cache Between Projects

You can use the same cache for multiple projects to save disk space:

```bash
# Create a shared cache directory
mkdir -p /path/to/shared/sproto-cache

# Use it across projects
cd project1
protoreg-cli resolve --cache-dir /path/to/shared/sproto-cache

cd ../project2
protoreg-cli resolve --cache-dir /path/to/shared/sproto-cache
```

#### Cache in CI/CD Environments

For CI/CD pipelines, you might want to cache dependencies between builds:

```bash
# Example CI script
CACHE_DIR="/ci/cache/sproto"
mkdir -p "$CACHE_DIR"

# Use the cache
protoreg-cli resolve --cache-dir "$CACHE_DIR"
protoreg-cli compile --cache-dir "$CACHE_DIR" --go_out=./gen
```

### Best Practices for Cache Management

1. **Regular Maintenance**: Periodically clean unused modules with `cache clean` to free up disk space.

2. **Offline Workflows**: Before going offline, run `resolve` to ensure all dependencies are cached.

3. **Version Control**: Don't commit the cache to version control; let each developer maintain their own cache.

4. **Project-Specific Cache**: For projects with conflicting requirements, use project-specific cache directories.

5. **CI/CD Cache**: Consider persisting the cache in CI/CD systems to speed up builds.
