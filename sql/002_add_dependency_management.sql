-- Migration to add dependency management features to SProto

-- Step 1: Add import_path column to the modules table
-- Make it nullable initially for backward compatibility with existing rows.
ALTER TABLE modules ADD COLUMN import_path VARCHAR(255);

-- Step 2: Add an index on the new import_path column for faster lookups
-- Use IF NOT EXISTS for potential compatibility if run multiple times (though migrations should ideally run once)
CREATE INDEX IF NOT EXISTS idx_module_import_path ON modules(import_path);

-- Step 3: Create the module_dependencies table
-- This table stores the relationships between modules (which module depends on which).
CREATE TABLE IF NOT EXISTS module_dependencies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    dependent_module_id UUID NOT NULL, -- The module that has the dependency
    required_module_id UUID NOT NULL,  -- The module that is being depended upon
    version_constraint VARCHAR(100) NOT NULL, -- The SemVer constraint (e.g., ">=v1.0.0", "v1.2.3")
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Foreign key constraints to ensure data integrity
    CONSTRAINT fk_dependent_module FOREIGN KEY (dependent_module_id) REFERENCES modules(id) ON DELETE CASCADE,
    CONSTRAINT fk_required_module FOREIGN KEY (required_module_id) REFERENCES modules(id) ON DELETE CASCADE,

    -- Ensure a module cannot depend on the same required module multiple times
    CONSTRAINT uq_dependency UNIQUE (dependent_module_id, required_module_id)
);

-- Note: This migration assumes the uuid-ossp extension is already enabled (from 001_enable_uuid.sql).
-- Note: Backfilling existing modules' import_path (e.g., based on namespace/name)
-- would typically be done via a separate script or application logic after the migration.
