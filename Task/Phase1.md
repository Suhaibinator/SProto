
### Phase 1: Configuration System and Schema Updates

#### Task 1.1: Design and Implement Configuration Format
- **1.1.1**: Design `sproto.yaml` format specification
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create a detailed specification for the `sproto.yaml` file format that will be backward compatible with existing SProto functionality
    - Define required fields:
      - `version`: Schema version (e.g., "v1")
      - `name`: Module identifier in the format "namespace/name" (e.g., "myorg/common")
      - `import_path`: Base Go-style import path for this module (e.g., "github.com/myorg/common")
    - Define optional fields:
      - `dependencies`: List of dependent modules with their version requirements
        - Each dependency must specify:
          - `namespace`: Organization or user namespace
          - `name`: Module name
          - `version`: Specific version or version constraint (e.g., "v1.0.0", ">=v1.2.0")
          - `import_path`: Import path prefix for this dependency
    - Document complete example configurations for different use cases:
      - Basic module with no dependencies
      - Module with single dependency
      - Module with multiple dependencies
      - Module with complex version constraints
    - Compare with Buf's format to ensure feature parity for essential functionality
    - Create a JSON schema for validation purposes
  
- **1.1.2**: Implement configuration parser
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create Go structs that map to the YAML configuration format:
      ```go
      type SProtoConfig struct {
          Version      string       `yaml:"version"`
          Name         string       `yaml:"name"`
          ImportPath   string       `yaml:"import_path"`
          Dependencies []Dependency `yaml:"dependencies,omitempty"`
      }
      
      type Dependency struct {
          Namespace  string `yaml:"namespace"`
          Name       string `yaml:"name"`
          Version    string `yaml:"version"`
          ImportPath string `yaml:"import_path"`
      }
      ```
    - Implement parser functions in a new `internal/config` package:
      - `func ParseConfig(configPath string) (*SProtoConfig, error)`
      - `func ParseConfigBytes(data []byte) (*SProtoConfig, error)`
    - Add helper methods for common operations:
      - `func (c *SProtoConfig) FullName() string` (returns "namespace/name")
      - `func (c *SProtoConfig) GetDependencyByName(name string) (*Dependency, bool)`
    - Use `gopkg.in/yaml.v3` for YAML parsing
    - Handle file I/O errors gracefully
    - Implement proper logging using the existing logger system
    - Add comments and documentation for functions and types
  
- **1.1.3**: Write validation logic for configuration files
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create a validation function `func (c *SProtoConfig) Validate() error` that checks:
      - Version is specified and supported (e.g., "v1")
      - Name follows the "namespace/name" pattern and contains valid characters
      - Import path is a valid Go import path format
      - Dependencies (if any):
        - Have valid namespace and name values
        - Have syntactically valid version strings (using Masterminds/semver)
        - Have valid import paths
        - Do not contain circular references
      - No duplicate dependencies exist
    - Implement specific, descriptive error messages for each validation failure
    - Create a standalone validation command for the CLI:
      ```go
      func ValidateConfigCmd() *cobra.Command {
          // Implementation that validates a config file and reports issues
      }
      ```
    - Add unit tests for validation logic covering:
      - Valid configurations
      - Invalid version format
      - Invalid module name format
      - Invalid import path
      - Invalid dependency references
      - Circular dependencies
      - Duplicate dependencies

#### Task 1.2: Update Database Schema
- **1.2.1**: Extend Module model to include import path
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Update the Module struct in `internal/models/models.go` to include an import path field:
      ```go
      type Module struct {
          ID          uuid.UUID       `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
          Namespace   string          `gorm:"type:varchar(255);not null;uniqueIndex:idx_module_namespace_name"`
          Name        string          `gorm:"type:varchar(255);not null;uniqueIndex:idx_module_namespace_name"`
          ImportPath  string          `gorm:"type:varchar(255);index:idx_module_import_path"` // New field
          CreatedAt   time.Time       `gorm:"not null;default:current_timestamp"`
          UpdatedAt   time.Time       `gorm:"not null;default:current_timestamp"`
          Versions    []ModuleVersion `gorm:"foreignKey:ModuleID"` 
          // Relationships
          Dependencies []ModuleDependency `gorm:"foreignKey:DependentModuleID"` // New relationship
      }
      ```
    - Add data validation tags if needed
    - Add Go documentation comments explaining the purpose of the ImportPath field
    - Ensure the field is properly indexed for efficient lookups
    - Make the import path nullable for backward compatibility with existing modules
    - Add helper methods for working with import paths:
      ```go
      func (m *Module) FullImportPath() string {
          if m.ImportPath == "" {
              return ""
          }
          return m.ImportPath
      }
      ```

- **1.2.2**: Create ModuleDependency model
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Define a new ModuleDependency struct in `internal/models/models.go` to model dependency relationships:
      ```go
      type ModuleDependency struct {
          ID                 uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
          DependentModuleID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_dependency_modules"` // FK to the module that depends on another
          RequiredModuleID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_dependency_modules"` // FK to the module that is required
          VersionConstraint  string    `gorm:"type:varchar(100);not null"` // SemVer constraint expression (e.g., ">=v1.0.0")
          CreatedAt          time.Time `gorm:"not null;default:current_timestamp"`
          UpdatedAt          time.Time `gorm:"not null;default:current_timestamp"`
          // Relationships (optional, depending on usage patterns)
          DependentModule    Module    `gorm:"foreignKey:DependentModuleID"`
          RequiredModule     Module    `gorm:"foreignKey:RequiredModuleID"`
      }
      ```
    - Set up unique constraints to prevent duplicate dependencies
    - Add GORM tags for proper table creation and foreign key relationships
    - Add validation methods to check version constraint syntax
    - Create helper functions for common operations:
      ```go
      // Checks if a specific version satisfies the constraint
      func (d *ModuleDependency) SatisfiedBy(version string) (bool, error) {
          // Implementation using Masterminds/semver
      }
      
      // Gets all dependencies for a module
      func GetDependenciesForModule(db *gorm.DB, moduleID uuid.UUID) ([]ModuleDependency, error) {
          // Implementation
      }
      ```
    - Document potential circular dependency concerns and how they're addressed

- **1.2.3**: Implement database migration
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create a new SQL migration file in `sql/002_add_dependency_management.sql` with:
      ```sql
      -- Add import path to modules table
      ALTER TABLE modules ADD COLUMN import_path VARCHAR(255);
      -- Create index on import_path for faster lookups
      CREATE INDEX idx_module_import_path ON modules(import_path);
      
      -- Create module dependencies table
      CREATE TABLE module_dependencies (
          id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
          dependent_module_id UUID NOT NULL,
          required_module_id UUID NOT NULL,
          version_constraint VARCHAR(100) NOT NULL,
          created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
          updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
          CONSTRAINT fk_dependent_module FOREIGN KEY (dependent_module_id) REFERENCES modules(id) ON DELETE CASCADE,
          CONSTRAINT fk_required_module FOREIGN KEY (required_module_id) REFERENCES modules(id) ON DELETE CASCADE,
          CONSTRAINT uq_dependency UNIQUE (dependent_module_id, required_module_id)
      );
      ```
    - Implement migration application mechanism in the server startup logic:
      ```go
      // In server initialization
      if err := db.AutoMigrate(&models.Module{}, &models.ModuleVersion{}, &models.ModuleDependency{}); err != nil {
          log.Fatalf("Failed to migrate database: %v", err)
      }
      ```
    - Add fallback logic for SQLite database schema:
      ```go
      // SQLite-specific schema adjustments
      if config.DB.Type == "sqlite" {
          // Handle SQLite-specific variations
      }
      ```
    - Create a data backfill plan for existing modules:
      - Default ImportPath can be derived from namespace/name for existing modules
      - No dependencies would be created for existing modules initially
    - Add database migration tests to ensure schema changes apply correctly
    - Document how to roll back migrations if needed

#### Task 1.3: Update API Handlers for New Schema
- **1.3.1**: Update module creation handler
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Modify the publish module version handler in `internal/api/handlers.go` to accept and process import path data:
      ```go
      // Update the PublishModuleVersionRequest struct to include import path
      type PublishModuleVersionRequest struct {
          ImportPath string `json:"import_path,omitempty"`
          // Existing fields...
      }
      ```
    - Update `HandlePublishModuleVersion` function to:
      - Extract import path information from the multipart form or from the `sproto.yaml` in the uploaded zip
      - Store the import path in the module record
      - Add validation for import path format
      - Maintain backward compatibility for clients not providing import path
    - Implement logic to automatically extract import path from the first matching `sproto.yaml` file in the zip artifact:
      ```go
      // Code snippet to extract config from zip
      func extractConfigFromZip(zipReader *zip.Reader) (*config.SProtoConfig, error) {
          for _, file := range zipReader.File {
              if filepath.Base(file.Name) == "sproto.yaml" {
                  // Extract and parse the file
                  // ...
              }
          }
          return nil, errors.New("sproto.yaml not found in artifact")
      }
      ```
    - Add handling for the module's import path during initial module creation
    - Add proper error handling for invalid import paths
    - Update documentation and example API requests

- **1.3.2**: Implement dependency relationship handling
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Create new API endpoints for dependency management:
      - `GET /api/v1/modules/{namespace}/{module_name}/dependencies` - List dependencies
      - `GET /api/v1/modules/{namespace}/{module_name}/{version}/dependencies` - List dependencies for specific version
    - Update the publish endpoint to extract and store dependency relationships:
      ```go
      // Code to store dependencies
      func storeDependencies(db *gorm.DB, moduleID uuid.UUID, dependencies []config.Dependency) error {
          for _, dep := range dependencies {
              // Find required module by namespace/name
              requiredModule := &models.Module{}
              result := db.Where("namespace = ? AND name = ?", dep.Namespace, dep.Name).First(requiredModule)
              if result.Error != nil {
                  return fmt.Errorf("dependency %s/%s not found: %w", dep.Namespace, dep.Name, result.Error)
              }
              
              // Create dependency relationship
              moduleDep := &models.ModuleDependency{
                  DependentModuleID: moduleID,
                  RequiredModuleID:  requiredModule.ID,
                  VersionConstraint: dep.Version,
              }
              
              if err := db.Create(moduleDep).Error; err != nil {
                  return fmt.Errorf("failed to save dependency: %w", err)
              }
          }
          return nil
      }
      ```
    - Add dependency validation during publish to ensure all declared dependencies exist in the registry
    - Implement dependency resolution endpoint for clients:
      ```go
      // Handler for dependency resolution
      func HandleResolveDependencies(w http.ResponseWriter, r *http.Request) {
          // Extract module coordinates
          // Recursively resolve dependencies
          // Return full dependency tree with version resolution
      }
      ```
    - Add proper error responses for missing dependencies or resolution conflicts
    - Create utility functions for dependency graph traversal
    - Implement caching of frequently resolved dependency trees for performance

- **1.3.3**: Update API response structs to include new fields
  - **Assignee**: Cline
  - **Status**: DONE
  - **Details**: 
    - Modify response structs in `internal/api/response/response.go` to include new fields:
      ```go
      // Update ModuleResponse struct
      type ModuleResponse struct {
          Namespace   string `json:"namespace"`
          Name        string `json:"name"`
          ImportPath  string `json:"import_path,omitempty"` // New field
          LatestVersion string `json:"latest_version,omitempty"`
      }
      
      // Update PublishModuleVersionResponse
      type PublishModuleVersionResponse struct {
          Namespace      string    `json:"namespace"`
          ModuleName     string    `json:"module_name"`
          ImportPath     string    `json:"import_path,omitempty"` // New field
          Version        string    `json:"version"`
          ArtifactDigest string    `json:"artifact_digest"`
          CreatedAt      time.Time `json:"created_at"`
          Dependencies   []DependencyResponse `json:"dependencies,omitempty"` // New field
      }
      
      // New response struct for dependencies
      type DependencyResponse struct {
          Namespace   string `json:"namespace"`
          Name        string `json:"name"`
          ImportPath  string `json:"import_path,omitempty"`
          Constraint  string `json:"version_constraint"`
      }
      ```
    - Create new response types for dependency-specific endpoints:
      ```go
      // Dependency resolution response
      type DependencyResolutionResponse struct {
          Root         ModuleVersionInfo   `json:"root"`
          Dependencies []ModuleVersionInfo `json:"dependencies"`
          ImportPaths  map[string]string   `json:"import_paths"` // Maps import paths to module versions
      }
      
      type ModuleVersionInfo struct {
          Namespace   string `json:"namespace"`
          Name        string `json:"name"`
          Version     string `json:"version"`
          ImportPath  string `json:"import_path,omitempty"`
      }
      ```
    - Update all handler functions to use the new response structs
    - Ensure backward compatibility by making new fields optional
    - Add proper documentation for response formats
    - Update API tests to verify correct response structures
