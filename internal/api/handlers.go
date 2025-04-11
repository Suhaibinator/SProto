package api

import (
	"log"
	"net/http"
	"sort"
	"strings"

	"errors"

	"fmt"
	"io"
	"net/url"

	// "strings" // Removed duplicate import line

	"github.com/Masterminds/semver/v3" // For potential semantic version sorting later
	"github.com/minio/minio-go/v7"     // Added import (using v7)

	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/Suhaibinator/SProto/internal/api/response"
	"github.com/Suhaibinator/SProto/internal/config" // Need config for Minio Bucket
	"github.com/Suhaibinator/SProto/internal/db"
	"github.com/Suhaibinator/SProto/internal/models"

	"github.com/Suhaibinator/SProto/internal/storage" // Need storage client
	// Will be needed
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// ListModulesResponse defines the structure for the list modules endpoint.
type ListModulesResponse struct {
	Modules []ModuleInfo `json:"modules"`
}

// ModuleInfo contains details for a single module in the list response.
type ModuleInfo struct {
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	LatestVersion string `json:"latest_version"` // Based on creation time for now
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

	// Get MinIO client and bucket name from config (assuming config is loaded globally or passed somehow)
	// For simplicity here, let's assume config is accessible or re-load it.
	// A better approach would be dependency injection.
	cfg, err := config.LoadConfig() // Re-loading config here is not ideal, should be injected
	if err != nil {
		log.Printf("Error loading config for MinIO bucket: %v", err)
		response.Error(w, http.StatusInternalServerError, "Internal server error (config)")
		return
	}
	minioClient := storage.GetMinioClient()
	bucketName := cfg.MinioBucket

	// Get the object from MinIO
	object, err := minioClient.GetObject(r.Context(), bucketName, moduleVersion.ArtifactStorageKey, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("Error getting object '%s' from bucket '%s': %v", moduleVersion.ArtifactStorageKey, bucketName, err)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve artifact from storage")
		return
	}
	defer object.Close() // Ensure the object reader is closed

	// Stat the object to get metadata like size (optional but good)
	objInfo, err := object.Stat()
	if err != nil {
		log.Printf("Error stating object '%s' from bucket '%s': %v", moduleVersion.ArtifactStorageKey, bucketName, err)
		// Don't necessarily fail the request, but log it. We can still try to stream.
	}

	// Set headers
	w.Header().Set("Content-Type", "application/zip")
	// Encode filename according to RFC 5987 for broader compatibility
	encodedFilename := url.PathEscape(fmt.Sprintf("%s.zip", version))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, version+".zip", encodedFilename))
	if moduleVersion.ArtifactDigest != "" {
		// Use the stored digest as ETag. Note: MinIO might set its own ETag based on object hash.
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, moduleVersion.ArtifactDigest))
	}
	if objInfo.Size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", objInfo.Size))
	}

	// Stream the object content to the response writer
	_, err = io.Copy(w, object)
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
	Namespace      string    `json:"namespace"`
	ModuleName     string    `json:"module_name"`
	Version        string    `json:"version"`
	ArtifactDigest string    `json:"artifact_digest"` // sha256:<hex_digest>
	CreatedAt      time.Time `json:"created_at"`
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

	// Calculate SHA256 digest while reading the file for upload
	hasher := sha256.New()
	// Use io.TeeReader to write to hasher while reading for upload
	teeReader := io.TeeReader(file, hasher)

	// --- Database and Storage Operations (Transaction) ---
	gormDB := db.GetDB()
	minioClient := storage.GetMinioClient()
	cfg, _ := config.LoadConfig() // Assuming config is loaded or accessible
	bucketName := cfg.MinioBucket

	var module models.Module
	var moduleVersion models.ModuleVersion
	var artifactDigestHex string
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

	// 1. Find or Create Module
	err = tx.Where(models.Module{Namespace: namespace, Name: moduleName}).
		Attrs(models.Module{Namespace: namespace, Name: moduleName}). // Set attributes if creating
		FirstOrCreate(&module).Error
	if err != nil {
		log.Printf("Error finding or creating module %s/%s: %v", namespace, moduleName, err)
		response.Error(w, http.StatusInternalServerError, "Database error during module lookup/creation")
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

	// 3. Upload to MinIO (using the TeeReader)
	storageKey = fmt.Sprintf("modules/%s/%s/protos.zip", module.ID.String(), versionStr)
	uploadInfo, err := minioClient.PutObject(r.Context(), bucketName, storageKey, teeReader, header.Size, minio.PutObjectOptions{
		ContentType: "application/zip",
		// Add user metadata if needed: UserMetadata: map[string]string{"module": fmt.Sprintf("%s/%s", namespace, moduleName)},
	})
	if err != nil {
		log.Printf("Error uploading artifact to MinIO (Bucket: %s, Key: %s): %v", bucketName, storageKey, err)
		response.Error(w, http.StatusInternalServerError, "Failed to upload artifact to storage")
		return // Triggers deferred rollback
	}
	log.Printf("Successfully uploaded %s of size %d to %s/%s", header.Filename, uploadInfo.Size, bucketName, storageKey)

	// 4. Get the final digest
	artifactDigestHex = hex.EncodeToString(hasher.Sum(nil))

	// 5. Create ModuleVersion record
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

	// 6. Explicitly update the parent module's updated_at timestamp
	err = tx.Model(&module).Update("updated_at", time.Now()).Error
	if err != nil {
		// Log the error but don't fail the whole operation just for the timestamp update
		log.Printf("Warning: Failed to update module %s/%s updated_at timestamp: %v", namespace, moduleName, err)
		err = nil // Reset error so commit doesn't rollback
	}

	// 7. Commit Transaction
	err = tx.Commit().Error
	if err != nil {
		log.Printf("Error committing transaction for %s/%s@%s: %v", namespace, moduleName, versionStr, err)
		response.Error(w, http.StatusInternalServerError, "Database error during commit")
		return // Already rolled back by commit error
	}

	// --- Success Response ---
	respData := PublishModuleVersionResponse{
		Namespace:      namespace,
		ModuleName:     moduleName,
		Version:        versionStr,
		ArtifactDigest: "sha256:" + artifactDigestHex, // Add prefix for clarity
		CreatedAt:      moduleVersion.CreatedAt,       // Use the timestamp from the created record
	}
	response.JSON(w, http.StatusCreated, respData)
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
