package models

import (
	"time"

	"github.com/google/uuid"
	// Re-add for clarity and potential future use with hooks/methods
)

// Module represents a logical grouping of related .proto files.
type Module struct {
	ID        uuid.UUID       `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Namespace string          `gorm:"type:varchar(255);not null;uniqueIndex:idx_module_namespace_name"`
	Name      string          `gorm:"type:varchar(255);not null;uniqueIndex:idx_module_namespace_name"`
	CreatedAt time.Time       `gorm:"not null;default:current_timestamp"`
	UpdatedAt time.Time       `gorm:"not null;default:current_timestamp"`
	Versions  []ModuleVersion `gorm:"foreignKey:ModuleID"` // Has many relationship
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

// BeforeSave GORM hook for ModuleVersion to update the parent Module's UpdatedAt timestamp.
// Note: This requires fetching the Module first or handling it in the service layer,
// as GORM hooks don't automatically cascade updates like the SQL trigger did.
// A simpler approach might be to update the Module's timestamp explicitly after
// successfully creating a ModuleVersion in the service/handler logic.
// Let's omit the hook for now and handle the timestamp update manually in the handler.

// You might need to enable the uuid-ossp extension manually in your database
// if GORM's AutoMigrate doesn't handle it automatically.
// `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`
