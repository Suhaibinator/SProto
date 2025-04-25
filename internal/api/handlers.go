package api

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/Suhaibinator/SProto/internal/api/response"
	"github.com/Suhaibinator/SProto/internal/config" // Added for sproto.yaml parsing
	"github.com/Suhaibinator/SProto/internal/db"
	"github.com/Suhaibinator/SProto/internal/models"
	"github.com/Suhaibinator/SProto/internal/storage"
	"github.com/google/uuid" // Added for storeDependencies
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// --- Helper Functions ---

// extractConfigFromZip searches for sproto.yaml in the root of a zip archive
// and parses it into an SProtoConfig struct.
func extractConfigFromZip(zipFilePath string) (*config.SProtoConfig, error) {
	zipReader, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file '%s': %w", zipFilePath, err)
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		// Check if the file is exactly 'sproto.yaml' at the root level
		// Normalize path separators just in case
		cleanedName := filepath.ToSlash(file.Name)
		if cleanedName == "sproto.yaml" {
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open sproto.yaml within zip: %w", err)
			}
			defer rc.Close()

			configBytes, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("failed to read sproto.yaml within zip: %w", err)
			}

			sprotoConfig, err := config.ParseConfigBytes(configBytes)
			if err != nil {
				// Log the parsing error but don't necessarily fail the whole publish yet
				log.Printf("Warning: Found sproto.yaml but failed to parse: %v", err)
				// Return the parsing error so the caller knows validation failed
				return nil, fmt.Errorf("failed to parse sproto.yaml: %w", err)
			}
			log.Printf("Successfully parsed sproto.yaml from artifact")
			return sprotoConfig, nil // Found and parsed successfully
		}
	}

	return nil, errors.New("sproto.yaml not found in the root of the artifact zip") // Not found
}

// storeDependencies saves the dependency relationships defined in sproto.yaml to the database.
// It operates within the provided database transaction.
func storeDependencies(tx *gorm.DB, dependentModuleID uuid.UUID, dependencies []config.Dependency) error {
	if len(dependencies) == 0 {
		return nil // Nothing to store
	}
	log.Printf("Storing %d dependencies for module ID %s", len(dependencies), dependentModuleID)

	for i, dep := range dependencies {
		// 1. Find the required module by namespace and name
		var requiredModule models.Module
		result := tx.Where("namespace = ? AND name = ?", dep.Namespace, dep.Name).First(&requiredModule)

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Dependency module not found in the registry
			return fmt.Errorf("dependency %d (%s/%s) not found in registry", i+1, dep.Namespace, dep.Name)
		} else if result.Error != nil {
			// Other database error finding the dependency
			log.Printf("Error finding dependency module %s/%s: %v", dep.Namespace, dep.Name, result.Error)
			return fmt.Errorf("database error finding dependency %d (%s/%s): %w", i+1, dep.Namespace, dep.Name, result.Error)
		}

		// 2. Validate version constraint syntax (already done in config parsing, but double-check)
		_, err := semver.NewConstraint(dep.Version)
		if err != nil {
			return fmt.Errorf("dependency %d (%s/%s) has invalid version constraint '%s': %w", i+1, dep.Namespace, dep.Name, dep.Version, err)
		}

		// 3. Create the ModuleDependency record
		moduleDep := models.ModuleDependency{
			DependentModuleID: dependentModuleID,
			RequiredModuleID:  requiredModule.ID,
			VersionConstraint: dep.Version,
			// CreatedAt/UpdatedAt set by default
		}

		err = tx.Create(&moduleDep).Error
		// Handle potential unique constraint violation (uq_dependency) gracefully if needed,
		// though duplicates should ideally be caught by config validation first.
		if err != nil {
			log.Printf("Error creating dependency record (%s/%s -> %s/%s): %v",
				moduleDep.DependentModuleID, // Assuming we can get namespace/name easily later
				moduleDep.RequiredModuleID,
				dep.Namespace, dep.Name, err)
			return fmt.Errorf("failed to save dependency %d (%s/%s): %w", i+1, dep.Namespace, dep.Name, err)
		}
		log.Printf("Successfully stored dependency: %s -> %s/%s (%s)", dependentModuleID, dep.Namespace, dep.Name, dep.Version)
	}
	return nil
}

// ListModulesResponse defines the structure for the list modules endpoint.
type ListModulesResponse struct {
	Modules []ModuleInfo `json:"modules"`
}

// ModuleInfo contains details for a single module in the list response.
type ModuleInfo struct {
	Namespace     string  `json:"namespace"`
	Name          string  `json:"name"`
	ImportPath    *string `json:"import_path,omitempty"` // Added import path
	LatestVersion string  `json:"latest_version"`        // Based on creation time for now
}

// ListModulesHandler handles requests to list all registered modules.
// GET /api/v1/modules
func ListModulesHandler(w http.ResponseWriter, r *http.Request) {
	gormDB := db.GetDB() // Get the initialized GORM DB instance

	// Use Raw SQL to execute the query similar to the one defined for sqlc,
	// as replicating the CTE and window function logic purely with GORM methods can be complex.
	query := `
		WITH LatestVersions AS (
			SELECT
				module_id,
				version,
				ROW_NUMBER() OVER(PARTITION BY module_id ORDER BY created_at DESC) as rn
			FROM module_versions
		)
		SELECT
			m.namespace,
			m.name,
			m.import_path, -- Added import path
			COALESCE(lv.version, '') AS latest_version
		FROM modules m
		LEFT JOIN LatestVersions lv ON m.id = lv.module_id AND lv.rn = 1
		ORDER BY m.namespace, m.name;
	`

	var results []ModuleInfo
	if err := gormDB.Raw(query).Scan(&results).Error; err != nil {
		log.Printf("Error listing modules: %v", err)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve modules")
		return
	}

	// Although the SQL query gets the latest by creation date,
	// true semantic version sorting might be desired here if versions
	// could be published out of order. We'll skip that complexity for now.

	respData := ListModulesResponse{Modules: results}
	if results == nil {
		// Ensure we return an empty array instead of null if no modules exist
		respData.Modules = []ModuleInfo{}
	}

	response.JSON(w, http.StatusOK, respData)
}

// --- Placeholder for other handlers ---
// ListModuleVersionsResponse defines the structure for listing versions of a module.
type ListModuleVersionsResponse struct {
	Namespace  string   `json:"namespace"`
	ModuleName string   `json:"module_name"`
	Versions   []string `json:"versions"`
}

// ListModuleVersionsHandler handles requests to list versions for a specific module.
// GET /api/v1/modules/{namespace}/{module_name}
func ListModuleVersionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	moduleName := vars["module_name"]

	if namespace == "" || moduleName == "" {
		response.Error(w, http.StatusBadRequest, "Namespace and module name are required")
		return
	}

	gormDB := db.GetDB()
	var module models.Module

	// Find the module first
	err := gormDB.Where("namespace = ? AND name = ?", namespace, moduleName).First(&module).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Module not found: %s/%s", namespace, moduleName)
			response.Error(w, http.StatusNotFound, "Module not found")
		} else {
			log.Printf("Error finding module %s/%s: %v", namespace, moduleName, err)
			response.Error(w, http.StatusInternalServerError, "Failed to retrieve module")
		}
		return
	}

	// Find the versions for this module
	var versions []string
	err = gormDB.Model(&models.ModuleVersion{}).Where("module_id = ?", module.ID).Order("created_at DESC").Pluck("version", &versions).Error
	if err != nil {
		log.Printf("Error listing versions for module %s/%s (ID: %s): %v", namespace, moduleName, module.ID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve module versions")
		return
	}

	// Sort versions semantically descending
	sortVersionsDesc(versions) // Use the helper function

	respData := ListModuleVersionsResponse{
		Namespace:  namespace,
		ModuleName: moduleName,
		Versions:   versions,
	}
	if versions == nil {
		respData.Versions = []string{} // Ensure empty array, not null
	}

	response.JSON(w, http.StatusOK, respData)
}

// FetchModuleVersionArtifactHandler handles requests to download a module version's artifact.
// GET /api/v1/modules/{namespace}/{module_name}/{version}/artifact
func FetchModuleVersionArtifactHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	moduleName := vars["module_name"]
	version := vars["version"] // Already URL decoded by mux

	if namespace == "" || moduleName == "" || version == "" {
		response.Error(w, http.StatusBadRequest, "Namespace, module name, and version are required")
		return
	}

	// Validate version format (basic check)
	if !strings.HasPrefix(version, "v") {
		response.Error(w, http.StatusBadRequest, "Invalid version format: must start with 'v'")
		return
	}
	// More robust SemVer validation could be added here if needed

	gormDB := db.GetDB()
	var moduleVersion models.ModuleVersion

	// Find the specific module version, joining with modules to filter by namespace/name
	err := gormDB.Joins("JOIN modules ON modules.id = module_versions.module_id").
		Where("modules.namespace = ? AND modules.name = ? AND module_versions.version = ?", namespace, moduleName, version).
		First(&moduleVersion).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Module version not found: %s/%s@%s", namespace, moduleName, version)
			response.Error(w, http.StatusNotFound, "Module version not found")
		} else {
			log.Printf("Error finding module version %s/%s@%s: %v", namespace, moduleName, version, err)
			response.Error(w, http.StatusInternalServerError, "Failed to retrieve module version details")
		}
		return
	}

	// Get the storage provider
	storageProvider := storage.GetStorageProvider()

	// Get the artifact stream from the storage provider
	artifactStream, err := storageProvider.DownloadFile(r.Context(), moduleVersion.ArtifactStorageKey)
	if err != nil {
		// Check if it's a 'not found' error specifically if possible (depends on provider impl)
		// Note: Need to import "os" for os.ErrNotExist
		if errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "no such key") {
			log.Printf("Artifact not found in storage: key=%s, error=%v", moduleVersion.ArtifactStorageKey, err)
			response.Error(w, http.StatusNotFound, "Artifact not found in storage")
		} else {
			log.Printf("Error downloading artifact from storage: key=%s, error=%v", moduleVersion.ArtifactStorageKey, err)
			response.Error(w, http.StatusInternalServerError, "Failed to retrieve artifact from storage")
		}
		return
	}
	defer artifactStream.Close() // Ensure the stream is closed

	// Set headers
	w.Header().Set("Content-Type", "application/zip") // Assuming all artifacts are zip
	// Encode filename according to RFC 5987 for broader compatibility
	encodedFilename := url.PathEscape(fmt.Sprintf("%s.zip", version))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, version+".zip", encodedFilename))
	if moduleVersion.ArtifactDigest != "" {
		// Use the stored digest as ETag.
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, moduleVersion.ArtifactDigest))
	}
	// Content-Length is harder to determine reliably beforehand with the abstraction, removed for now.
	// If needed later, the StorageProvider interface could be extended with a StatFile method.

	// Stream the artifact content to the response writer
	_, err = io.Copy(w, artifactStream)
	if err != nil {
		// This error might happen if the client disconnects mid-stream
		log.Printf("Error streaming artifact %s/%s@%s to client: %v", namespace, moduleName, version, err)
		// Can't send an error response here as headers/body might be partially written
		return
	}
}

// PublishModuleVersionRequest defines the expected path parameters (implicitly handled by mux).
// The request body is multipart/form-data with a file field named "artifact".

// PublishModuleVersionResponse defines the successful response structure.
type PublishModuleVersionResponse struct {
	Namespace      string               `json:"namespace"`
	ModuleName     string               `json:"module_name"`
	Version        string               `json:"version"`
	ImportPath     *string              `json:"import_path,omitempty"` // Added import path
	ArtifactDigest string               `json:"artifact_digest"`       // sha256:<hex_digest>
	CreatedAt      time.Time            `json:"created_at"`
	Dependencies   []DependencyResponse `json:"dependencies,omitempty"` // Added dependencies list
}

// PublishModuleVersionHandler handles requests to publish a new module version.
// POST /api/v1/modules/{namespace}/{module_name}/{version}
// Requires Authentication.
func PublishModuleVersionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	moduleName := vars["module_name"]
	versionStr := vars["version"] // Already URL decoded

	// --- Input Validation ---
	if namespace == "" || moduleName == "" || versionStr == "" {
		response.Error(w, http.StatusBadRequest, "Namespace, module name, and version are required")
		return
	}

	// Validate SemVer format
	semVer, err := semver.NewVersion(versionStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, fmt.Sprintf("Invalid semantic version format: %v", err))
		return
	}
	// Re-assign versionStr to ensure it includes the 'v' prefix consistently if the library stripped it
	versionStr = "v" + semVer.String()

	// --- File Handling & Digest Calculation ---
	// Limit upload size (e.g., 32 MB)
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20) // 32 MB
	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		log.Printf("Error parsing multipart form: %v", err)
		if errors.Is(err, http.ErrMissingBoundary) || strings.Contains(err.Error(), "no multipart boundary param") {
			response.Error(w, http.StatusBadRequest, "Invalid request: Missing or malformed multipart boundary")
		} else if strings.Contains(err.Error(), "request body too large") {
			response.Error(w, http.StatusRequestEntityTooLarge, "Artifact file size exceeds limit (32MB)")
		} else {
			response.Error(w, http.StatusBadRequest, "Could not parse multipart form")
		}
		return
	}

	file, header, err := r.FormFile("artifact")
	if err != nil {
		log.Printf("Error retrieving artifact file from form: %v", err)
		if errors.Is(err, http.ErrMissingFile) {
			response.Error(w, http.StatusBadRequest, "Missing 'artifact' file in form data")
		} else {
			response.Error(w, http.StatusBadRequest, "Could not retrieve artifact file")
		}
		return
	}
	defer file.Close()
	log.Printf("Received artifact file: %s, Size: %d", header.Filename, header.Size)

	// --- File Handling: Temp File, Hashing, Zip Inspection ---
	// Create a temporary file to store the upload
	tempFile, err := os.CreateTemp("", "sproto-artifact-*.zip")
	if err != nil {
		log.Printf("Error creating temporary file: %v", err)
		response.Error(w, http.StatusInternalServerError, "Failed to process artifact file")
		return
	}
	defer os.Remove(tempFile.Name()) // Clean up the temp file afterwards
	defer tempFile.Close()           // Close the file handle

	// Calculate SHA256 digest while copying to the temporary file
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)
	writtenBytes, err := io.Copy(multiWriter, file)
	if err != nil {
		log.Printf("Error writing artifact to temporary file: %v", err)
		response.Error(w, http.StatusInternalServerError, "Failed to process artifact file")
		return
	}
	if writtenBytes != header.Size {
		log.Printf("Warning: Size mismatch writing artifact (expected %d, wrote %d)", header.Size, writtenBytes)
		// Potentially return an error here depending on strictness
	}
	artifactDigestHex := hex.EncodeToString(hasher.Sum(nil))
	tempFilePath := tempFile.Name()
	tempFile.Close() // Close after writing

	// Attempt to extract sproto.yaml config from the temp zip file
	sprotoConfig, err := extractConfigFromZip(tempFilePath)
	if err != nil {
		// Log the error but treat it as non-fatal for now (allow publishing without sproto.yaml)
		log.Printf("Warning: Could not extract or parse sproto.yaml from artifact: %v", err)
		// Set sprotoConfig to nil to indicate it wasn't successfully parsed
		sprotoConfig = nil
		err = nil // Reset error so we don't fail the publish
	}

	// --- Database and Storage Operations (Transaction) ---
	gormDB := db.GetDB()
	storageProvider := storage.GetStorageProvider() // Get the initialized provider

	var module models.Module
	var moduleVersion models.ModuleVersion
	var storageKey string

	// Start transaction
	tx := gormDB.Begin()
	if tx.Error != nil {
		log.Printf("Error starting database transaction: %v", tx.Error)
		response.Error(w, http.StatusInternalServerError, "Database error")
		return
	}
	// Defer rollback in case of errors
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback() // Rollback on panic
			panic(r)      // Re-panic
		} else if err != nil {
			log.Printf("Rolling back transaction due to error: %v", err)
			tx.Rollback() // Rollback on explicit error
		}
	}()

	// 1. Find or Create Module, potentially updating ImportPath
	err = tx.Where("namespace = ? AND name = ?", namespace, moduleName).First(&module).Error
	isNewModule := false
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Module doesn't exist, create it
		isNewModule = true
		module = models.Module{
			Namespace: namespace,
			Name:      moduleName,
			// Set ImportPath only if sprotoConfig is valid and parsed
			ImportPath: nil, // Default to nil
		}
		if sprotoConfig != nil {
			// Validate config name matches path params
			if sprotoConfig.Name != fmt.Sprintf("%s/%s", namespace, moduleName) {
				err = fmt.Errorf("module name in sproto.yaml ('%s') does not match URL path ('%s/%s')", sprotoConfig.Name, namespace, moduleName)
				log.Println(err.Error())
				response.Error(w, http.StatusBadRequest, err.Error())
				return // Triggers rollback
			}
			module.ImportPath = &sprotoConfig.ImportPath // Assign parsed import path
		}
		err = tx.Create(&module).Error
	} else if err == nil {
		// Module exists, check if we should update ImportPath
		if module.ImportPath == nil && sprotoConfig != nil {
			// Validate config name matches path params
			if sprotoConfig.Name != fmt.Sprintf("%s/%s", namespace, moduleName) {
				err = fmt.Errorf("module name in sproto.yaml ('%s') does not match URL path ('%s/%s')", sprotoConfig.Name, namespace, moduleName)
				log.Println(err.Error())
				response.Error(w, http.StatusBadRequest, err.Error())
				return // Triggers rollback
			}
			// Only update if current path is nil and we have a valid new one
			log.Printf("Updating existing module %s/%s with import path from sproto.yaml: %s", namespace, moduleName, sprotoConfig.ImportPath)
			err = tx.Model(&module).Update("import_path", sprotoConfig.ImportPath).Error
		} else if module.ImportPath != nil && sprotoConfig != nil && *module.ImportPath != sprotoConfig.ImportPath {
			// Existing path differs from config path - log warning, don't update automatically
			log.Printf("Warning: Module %s/%s already has import path '%s'. Ignoring different path '%s' from sproto.yaml.",
				namespace, moduleName, *module.ImportPath, sprotoConfig.ImportPath)
		}
	}
	// Handle potential errors from create/update
	if err != nil {
		log.Printf("Error finding/creating/updating module %s/%s: %v", namespace, moduleName, err)
		response.Error(w, http.StatusInternalServerError, "Database error during module operation")
		return // Triggers deferred rollback
	}

	// 2. Check for existing version (Conflict)
	err = tx.Where("module_id = ? AND version = ?", module.ID, versionStr).First(&models.ModuleVersion{}).Error
	if err == nil {
		// Found existing version - Conflict
		err = fmt.Errorf("version '%s' already exists for module '%s/%s'", versionStr, namespace, moduleName)
		log.Println(err.Error())
		response.Error(w, http.StatusConflict, err.Error())
		return // Triggers deferred rollback
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// Unexpected DB error during check
		log.Printf("Error checking for existing version %s/%s@%s: %v", namespace, moduleName, versionStr, err)
		response.Error(w, http.StatusInternalServerError, "Database error during version check")
		return // Triggers deferred rollback
	}
	// Reset err as ErrRecordNotFound is expected if version doesn't exist
	err = nil

	// 3. Upload to Storage Provider (reading from the temp file)
	storageKey = fmt.Sprintf("modules/%s/%s/protos.zip", module.ID.String(), versionStr) // Define storage key structure
	tempFileReader, err := os.Open(tempFilePath)
	if err != nil {
		log.Printf("Error reopening temporary file for upload %s: %v", tempFilePath, err)
		response.Error(w, http.StatusInternalServerError, "Failed to process artifact file for upload")
		return // Triggers rollback
	}
	defer tempFileReader.Close()

	err = storageProvider.UploadFile(r.Context(), storageKey, tempFileReader, header.Size, "application/zip")
	if err != nil {
		log.Printf("Error uploading artifact to storage (Key: %s): %v", storageKey, err)
		response.Error(w, http.StatusInternalServerError, "Failed to upload artifact to storage")
		return // Triggers deferred rollback
	}
	log.Printf("Successfully uploaded %s (Key: %s, Size: %d)", header.Filename, storageKey, header.Size)
	tempFileReader.Close() // Close reader after upload

	// 4. Create ModuleVersion record (digest already calculated)
	moduleVersion = models.ModuleVersion{
		ModuleID:           module.ID,
		Version:            versionStr,
		ArtifactDigest:     artifactDigestHex,
		ArtifactStorageKey: storageKey,
		// CreatedAt is set by default
	}
	err = tx.Create(&moduleVersion).Error
	if err != nil {
		log.Printf("Error creating module version record %s/%s@%s: %v", namespace, moduleName, versionStr, err)
		// Attempt to clean up MinIO object if DB insert fails? Maybe too complex.
		response.Error(w, http.StatusInternalServerError, "Database error saving module version")
		return // Triggers deferred rollback
	}

	// 6. Store Dependencies if sproto.yaml was parsed successfully
	if sprotoConfig != nil && len(sprotoConfig.Dependencies) > 0 {
		err = storeDependencies(tx, module.ID, sprotoConfig.Dependencies)
		if err != nil {
			// storeDependencies already logs details
			// Return specific error message from storeDependencies
			response.Error(w, http.StatusBadRequest, fmt.Sprintf("Failed to store module dependencies: %v", err)) // Use Bad Request as it's likely a missing dep
			return                                                                                                // Triggers rollback
		}
	}

	// 7. Explicitly update the parent module's updated_at timestamp (only if needed)
	if isNewModule || moduleVersion.CreatedAt.After(module.UpdatedAt) { // Update if new module or new version is latest
		err = tx.Model(&module).Update("updated_at", moduleVersion.CreatedAt).Error // Use version creation time
		if err != nil {
			// Log the error but don't fail the whole operation just for the timestamp update
			log.Printf("Warning: Failed to update module %s/%s updated_at timestamp: %v", namespace, moduleName, err)
			err = nil // Reset error so commit doesn't rollback
		}
	}

	// 8. Commit Transaction
	err = tx.Commit().Error
	if err != nil {
		log.Printf("Error committing transaction for %s/%s@%s: %v", namespace, moduleName, versionStr, err)
		response.Error(w, http.StatusInternalServerError, "Database error during commit")
		return // Already rolled back by commit error
	}

	// --- Success Response ---
	// Fetch stored dependencies to include in the response
	storedDeps, fetchErr := models.GetDependenciesForModule(gormDB, module.ID) // Use the non-transactional DB reader
	if fetchErr != nil {
		// Log the error but don't fail the response just because we couldn't fetch deps for it
		log.Printf("Warning: Failed to fetch stored dependencies for response for module %s: %v", module.ID, fetchErr)
	}
	respDeps := make([]DependencyResponse, 0, len(storedDeps))
	// Need to fetch details for each required module to populate DependencyResponse fully
	for _, sd := range storedDeps {
		var reqMod models.Module
		// This could be optimized by fetching all required modules in one query beforehand
		if res := gormDB.First(&reqMod, sd.RequiredModuleID); res.Error == nil {
			respDeps = append(respDeps, DependencyResponse{
				Namespace:         reqMod.Namespace,
				Name:              reqMod.Name,
				ImportPath:        reqMod.FullImportPath(), // Use helper method
				VersionConstraint: sd.VersionConstraint,
			})
		} else {
			log.Printf("Warning: Could not fetch details for required module %s for response: %v", sd.RequiredModuleID, res.Error)
		}
	}

	respData := PublishModuleVersionResponse{
		Namespace:      namespace,
		ModuleName:     moduleName,
		Version:        versionStr,
		ImportPath:     module.ImportPath,             // Include import path in response
		ArtifactDigest: "sha256:" + artifactDigestHex, // Add prefix for clarity
		CreatedAt:      moduleVersion.CreatedAt,       // Use the timestamp from the created record
		Dependencies:   respDeps,                      // Include dependencies
	}
	response.JSON(w, http.StatusCreated, respData)
}

// --- Dependency Handlers ---

// DependencyResponse defines the structure for a single dependency in API responses.
type DependencyResponse struct {
	Namespace         string `json:"namespace"`
	Name              string `json:"name"`
	ImportPath        string `json:"import_path,omitempty"` // Import path of the required module
	VersionConstraint string `json:"version_constraint"`
}

// ListModuleDependenciesResponse defines the structure for the list module dependencies endpoint.
type ListModuleDependenciesResponse struct {
	Namespace    string               `json:"namespace"`
	ModuleName   string               `json:"module_name"`
	Dependencies []DependencyResponse `json:"dependencies"`
}

// HandleListModuleDependencies handles requests to list dependencies for a specific module.
// GET /api/v1/modules/{namespace}/{module_name}/dependencies
func HandleListModuleDependencies(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	moduleName := vars["module_name"]

	if namespace == "" || moduleName == "" {
		response.Error(w, http.StatusBadRequest, "Namespace and module name are required")
		return
	}

	gormDB := db.GetDB()
	var module models.Module

	// 1. Find the module
	err := gormDB.Where("namespace = ? AND name = ?", namespace, moduleName).First(&module).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Module not found for listing dependencies: %s/%s", namespace, moduleName)
			response.Error(w, http.StatusNotFound, "Module not found")
		} else {
			log.Printf("Error finding module %s/%s for dependencies: %v", namespace, moduleName, err)
			response.Error(w, http.StatusInternalServerError, "Failed to retrieve module")
		}
		return
	}

	// 2. Get dependencies for the module
	// Use a struct to fetch required module details along with the dependency
	type DependencyWithDetails struct {
		models.ModuleDependency
		RequiredNamespace  string  `gorm:"column:required_namespace"`
		RequiredName       string  `gorm:"column:required_name"`
		RequiredImportPath *string `gorm:"column:required_import_path"`
	}
	var dependenciesWithDetails []DependencyWithDetails

	err = gormDB.Table("module_dependencies md").
		Select("md.*, rm.namespace as required_namespace, rm.name as required_name, rm.import_path as required_import_path").
		Joins("JOIN modules rm ON rm.id = md.required_module_id").
		Where("md.dependent_module_id = ?", module.ID).
		Order("required_namespace, required_name"). // Order for consistency
		Scan(&dependenciesWithDetails).Error

	if err != nil {
		log.Printf("Error retrieving dependencies for module %s/%s (ID: %s): %v", namespace, moduleName, module.ID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve module dependencies")
		return
	}

	// 3. Format the response
	respDeps := make([]DependencyResponse, 0, len(dependenciesWithDetails))
	for _, dep := range dependenciesWithDetails {
		importPath := ""
		if dep.RequiredImportPath != nil {
			importPath = *dep.RequiredImportPath
		}
		respDeps = append(respDeps, DependencyResponse{
			Namespace:         dep.RequiredNamespace,
			Name:              dep.RequiredName,
			ImportPath:        importPath,
			VersionConstraint: dep.VersionConstraint,
		})
	}

	respData := ListModuleDependenciesResponse{
		Namespace:    namespace,
		ModuleName:   moduleName,
		Dependencies: respDeps,
	}
	if respDeps == nil {
		respData.Dependencies = []DependencyResponse{} // Ensure empty array, not null
	}

	response.JSON(w, http.StatusOK, respData)
}

// HandleListModuleVersionDependencies - Placeholder as dependencies are currently module-level.
// GET /api/v1/modules/{namespace}/{module_name}/{version}/dependencies
// For now, this will return the same module-level dependencies regardless of version.
func HandleListModuleVersionDependencies(w http.ResponseWriter, r *http.Request) {
	// Implementation is identical to HandleListModuleDependencies for now,
	// as the schema links dependencies to modules, not specific versions.
	// We just need to validate the version exists before proceeding.
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	moduleName := vars["module_name"]
	version := vars["version"]

	if namespace == "" || moduleName == "" || version == "" {
		response.Error(w, http.StatusBadRequest, "Namespace, module name, and version are required")
		return
	}

	gormDB := db.GetDB()

	// 1. Verify the module version exists first
	var moduleVersion models.ModuleVersion
	err := gormDB.Joins("JOIN modules ON modules.id = module_versions.module_id").
		Where("modules.namespace = ? AND modules.name = ? AND module_versions.version = ?", namespace, moduleName, version).
		Select("module_versions.module_id"). // Only need module_id
		First(&moduleVersion).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Module version not found for listing dependencies: %s/%s@%s", namespace, moduleName, version)
			response.Error(w, http.StatusNotFound, "Module version not found")
		} else {
			log.Printf("Error finding module version %s/%s@%s for dependencies: %v", namespace, moduleName, version, err)
			response.Error(w, http.StatusInternalServerError, "Failed to retrieve module version details")
		}
		return
	}

	// 2. Get dependencies for the module (using the found module ID)
	type DependencyWithDetails struct {
		models.ModuleDependency
		RequiredNamespace  string  `gorm:"column:required_namespace"`
		RequiredName       string  `gorm:"column:required_name"`
		RequiredImportPath *string `gorm:"column:required_import_path"`
	}
	var dependenciesWithDetails []DependencyWithDetails

	err = gormDB.Table("module_dependencies md").
		Select("md.*, rm.namespace as required_namespace, rm.name as required_name, rm.import_path as required_import_path").
		Joins("JOIN modules rm ON rm.id = md.required_module_id").
		Where("md.dependent_module_id = ?", moduleVersion.ModuleID). // Use module ID from the version check
		Order("required_namespace, required_name").
		Scan(&dependenciesWithDetails).Error

	if err != nil {
		log.Printf("Error retrieving dependencies for module version %s/%s@%s (ModuleID: %s): %v", namespace, moduleName, version, moduleVersion.ModuleID, err)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve module dependencies")
		return
	}

	// 3. Format the response (same as module-level)
	respDeps := make([]DependencyResponse, 0, len(dependenciesWithDetails))
	for _, dep := range dependenciesWithDetails {
		importPath := ""
		if dep.RequiredImportPath != nil {
			importPath = *dep.RequiredImportPath
		}
		respDeps = append(respDeps, DependencyResponse{
			Namespace:         dep.RequiredNamespace,
			Name:              dep.RequiredName,
			ImportPath:        importPath,
			VersionConstraint: dep.VersionConstraint,
		})
	}

	// Response struct is the same as module-level for now
	respData := ListModuleDependenciesResponse{
		Namespace:    namespace,
		ModuleName:   moduleName,
		Dependencies: respDeps,
	}
	if respDeps == nil {
		respData.Dependencies = []DependencyResponse{}
	}

	response.JSON(w, http.StatusOK, respData)
}

// HandleResolveDependencies - Placeholder for dependency resolution logic (Phase 2/3).
// GET /api/v1/resolve?module={namespace}/{module_name}&version={version} (Example query params)
func HandleResolveDependencies(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement dependency resolution logic.
	// This will involve:
	// 1. Parsing module/version from query params.
	// 2. Fetching the root module's dependencies.
	// 3. Recursively fetching dependencies of dependencies.
	// 4. Resolving version constraints using semver logic (e.g., finding the highest compatible version).
	// 5. Detecting and handling conflicts or cycles.
	// 6. Returning a flattened list of resolved dependencies (specific versions) and their import paths.
	log.Println("Placeholder: HandleResolveDependencies called")
	response.Error(w, http.StatusNotImplemented, "Dependency resolution endpoint not yet implemented")
}

// --- Structs for Dependency Resolution Response (Placeholder) ---

// DependencyResolutionResponse defines the structure for the dependency resolution endpoint.
type DependencyResolutionResponse struct {
	Root         ModuleVersionInfo   `json:"root"`
	Dependencies []ModuleVersionInfo `json:"dependencies"`     // Flattened list of resolved dependencies
	ImportPaths  map[string]string   `json:"import_paths"`     // Maps import paths to module versions (e.g., "github.com/org/mod/proto" -> "org/mod@v1.2.3")
	Errors       []string            `json:"errors,omitempty"` // Any resolution errors encountered
}

// ModuleVersionInfo contains details for a specific resolved module version.
type ModuleVersionInfo struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	ImportPath string `json:"import_path,omitempty"`
	// Could add Digest here if needed for fetching artifacts
}

// Helper function for semantic version sorting
func sortVersionsDesc(versions []string) {
	semvers := make([]*semver.Version, 0, len(versions))
	for _, vStr := range versions {
		v, err := semver.NewVersion(vStr)
		if err == nil {
			semvers = append(semvers, v)
		} else {
			log.Printf("Warning: Could not parse version '%s' for sorting: %v", vStr, err)
			// Decide how to handle unparseable versions - maybe keep original string?
		}
	}

	// Sort descending
	sort.Sort(sort.Reverse(semver.Collection(semvers)))

	// Overwrite the original slice with sorted versions
	for i, v := range semvers {
		// Ensure 'v' prefix if it was potentially missing, though spec implies it's always there
		versions[i] = "v" + v.String()
	}
}
