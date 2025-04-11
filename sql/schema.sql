-- Enable UUID generation if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Table to store module definitions
CREATE TABLE modules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    namespace VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Ensure unique combination of namespace and name
    CONSTRAINT uq_module_namespace_name UNIQUE (namespace, name)
);

-- Index for efficient lookup by namespace and name
CREATE INDEX idx_module_namespace_name ON modules (namespace, name);

-- Table to store specific versions of modules and links to their artifacts
CREATE TABLE module_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    -- Store version string directly (e.g., "v1.2.3")
    version VARCHAR(100) NOT NULL CHECK (version ~ '^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$'), -- Basic SemVer check constraint
    -- SHA256 digest of the artifact zip file for integrity
    artifact_digest VARCHAR(64) NOT NULL, -- SHA256 hex string length
    -- The key (path) within the MinIO bucket where the artifact is stored
    artifact_storage_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Ensure unique combination of module and version
    CONSTRAINT uq_module_version UNIQUE (module_id, version)
);

-- Index for efficient lookup by module and version
CREATE INDEX idx_module_version ON module_versions (module_id, version);
-- Index for finding all versions of a module
CREATE INDEX idx_module_versions_module_id ON module_versions (module_id);

-- Trigger function to update 'updated_at' timestamp on module table
CREATE OR REPLACE FUNCTION update_module_updated_at()
RETURNS TRIGGER AS $$
BEGIN
   UPDATE modules
   SET updated_at = CURRENT_TIMESTAMP
   WHERE id = NEW.module_id;
   RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to execute the function after insert on module_versions
CREATE TRIGGER trigger_update_module_timestamp
AFTER INSERT ON module_versions
FOR EACH ROW EXECUTE FUNCTION update_module_updated_at();
