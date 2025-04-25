package models

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
	// Re-add for clarity and potential future use with hooks/methods
)

// Module represents a logical grouping of related .proto files.
type Module struct {
	ID           uuid.UUID          `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Namespace    string             `gorm:"type:varchar(255);not null;uniqueIndex:idx_module_namespace_name"`
	Name         string             `gorm:"type:varchar(255);not null;uniqueIndex:idx_module_namespace_name"`
	ImportPath   *string            `gorm:"type:varchar(255);index:idx_module_import_path"` // Base Go-style import path for this module's protos. Nullable for backward compatibility.
	CreatedAt    time.Time          `gorm:"not null;default:current_timestamp"`
	UpdatedAt    time.Time          `gorm:"not null;default:current_timestamp"`
	Versions     []ModuleVersion    `gorm:"foreignKey:ModuleID"`          // Has many relationship
	Dependencies []ModuleDependency `gorm:"foreignKey:DependentModuleID"` // Module dependencies (this module depends on others)
}

// FullImportPath returns the module's import path, or an empty string if not set.
func (m *Module) FullImportPath() string {
	if m.ImportPath == nil {
		return ""
	}
	return *m.ImportPath
}

// ModuleVersion represents a specific version of a module.
type ModuleVersion struct {
	ID                 uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	ModuleID           uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_module_version"`         // Foreign key
	Version            string    `gorm:"type:varchar(100);not null;uniqueIndex:idx_module_version"` // SemVer string
	ArtifactDigest     string    `gorm:"type:varchar(64);not null"`                                 // SHA256 hex string
	ArtifactStorageKey string    `gorm:"type:text;not null"`                                        // Key in MinIO
	CreatedAt          time.Time `gorm:"not null;default:current_timestamp"`
	// Module             Module    `gorm:"foreignKey:ModuleID"` // Belongs to relationship (optional, can use ModuleID directly)
}

// ModuleDependency represents a dependency relationship between two modules.
type ModuleDependency struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	DependentModuleID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_dependency_modules"` // FK to the module that depends on another
	RequiredModuleID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_dependency_modules"` // FK to the module that is required
	VersionConstraint string    `gorm:"type:varchar(100);not null"`                            // SemVer constraint expression (e.g., ">=v1.0.0")
	CreatedAt         time.Time `gorm:"not null;default:current_timestamp"`
	UpdatedAt         time.Time `gorm:"not null;default:current_timestamp"`
	// Relationships (optional, useful for eager loading if needed)
	// DependentModule   Module    `gorm:"foreignKey:DependentModuleID"`
	// RequiredModule    Module    `gorm:"foreignKey:RequiredModuleID"`
}

// SatisfiedBy checks if a given version string satisfies the dependency's version constraint.
func (d *ModuleDependency) SatisfiedBy(versionStr string) (bool, error) {
	constraint, err := semver.NewConstraint(d.VersionConstraint)
	if err != nil {
		// This should ideally not happen if constraints are validated on save.
		return false, fmt.Errorf("invalid version constraint '%s' in database: %w", d.VersionConstraint, err)
	}

	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return false, fmt.Errorf("invalid version string '%s' provided for check: %w", versionStr, err)
	}

	return constraint.Check(version), nil
}

// GetDependenciesForModule retrieves all dependencies for a given module ID.
// Note: Consider moving database interaction logic like this to a dedicated db package/layer.
func GetDependenciesForModule(db *gorm.DB, moduleID uuid.UUID) ([]ModuleDependency, error) {
	var dependencies []ModuleDependency
	// Eager load RequiredModule details if needed often, e.g.: .Preload("RequiredModule")
	result := db.Where("dependent_module_id = ?", moduleID).Find(&dependencies)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get dependencies for module %s: %w", moduleID, result.Error)
	}
	return dependencies, nil
}

// Note on Circular Dependencies: The current schema allows circular dependencies
// (e.g., Module A depends on B, Module B depends on A). Detecting and preventing
// these cycles typically requires graph traversal logic during the publish/validation phase,
// rather than just at the database schema level.

// BeforeSave GORM hook for ModuleVersion to update the parent Module's UpdatedAt timestamp.
// Note: This requires fetching the Module first or handling it in the service layer,
// as GORM hooks don't automatically cascade updates like the SQL trigger did.
// A simpler approach might be to update the Module's timestamp explicitly after
// successfully creating a ModuleVersion in the service/handler logic.
// Let's omit the hook for now and handle the timestamp update manually in the handler.

// You might need to enable the uuid-ossp extension manually in your database
// if GORM's AutoMigrate doesn't handle it automatically.
// `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`
