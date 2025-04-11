package api

import (
	// For multipart body
	"errors" // Ensure fmt is imported
	// For creating multipart request
	"net/http"
	"net/http/httptest" // Re-add httptest

	// Add url import
	"regexp" // For sqlmock query matching
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Suhaibinator/SProto/internal/db" // Import db package
	// Keep storage import
	"github.com/google/uuid" // For generating UUIDs in tests
	"github.com/gorilla/mux" // For setting URL vars
	// Keep minio import
	"github.com/stretchr/testify/assert" // Use testify/assert

	// Removed unused imports: net/url
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// --- Test Setup (Mocks, etc.) ---

// --- Test Setup (Mocks, etc.) ---

// setupMockDB initializes sqlmock and returns a mock GORM DB instance and the sqlmock controller.
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	mockDb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	// Ensure mockDb is closed at the end of the test
	t.Cleanup(func() { mockDb.Close() })

	dialector := postgres.New(postgres.Config{
		Conn:       mockDb,
		DriverName: "postgres",
	})
	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open gorm v2 db: %v", err)
	}

	// Replace the global DB instance with the mock
	// Prevent panic in db.GetDB() if called before mock is set
	db.SetDB(&gorm.DB{}) // Set a dummy non-nil DB temporarily
	// Handle case where original DB might be nil in test environment
	// var originalDB *gorm.DB // Removed unused variable
	// Check if db.DB is exported and accessible, or add a helper like db.IsInitialized()
	// Assuming we can access it or know it's nil initially in tests.
	// If db.DB is not exported, we might need a different approach or assume it's nil.
	// Let's proceed assuming we can check or it's nil.
	// A safer check might involve a TryGetDB that doesn't panic.
	// For now, let's just set the mock and restore nil in cleanup if needed.

	db.SetDB(gormDB) // Set the mock DB

	// Restore nil after test, as it wasn't initialized
	t.Cleanup(func() { db.SetDB(nil) })

	return gormDB, mock
}

// --- Tests for ListModulesHandler ---

func TestListModulesHandler_Success(t *testing.T) {
	_, mock := setupMockDB(t) // Setup mock DB

	// Define expected SQL query (use regexp for flexibility)
	// Note: GORM might generate slightly different SQL, adjust regex as needed.
	// This regex tries to match the core parts of the raw query used in the handler.
	expectedSQL := regexp.QuoteMeta(`
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
	`)

	// Define expected rows returned by the mock
	rows := sqlmock.NewRows([]string{"namespace", "name", "latest_version"}).
		AddRow("my-org", "module-a", "v1.1.0").
		AddRow("my-org", "module-b", "v0.1.0").
		AddRow("other-org", "cool-mod", "") // Module with no versions

	// Expect the query to be executed
	mock.ExpectQuery(expectedSQL).WillReturnRows(rows)

	// --- Execute Request ---
	req, err := http.NewRequest("GET", "/api/v1/modules", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ListModulesHandler)
	handler.ServeHTTP(rr, req)

	// --- Assertions ---
	// Assert status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Assert response body
	expectedBody := `{"modules":[{"namespace":"my-org","name":"module-a","latest_version":"v1.1.0"},{"namespace":"my-org","name":"module-b","latest_version":"v0.1.0"},{"namespace":"other-org","name":"cool-mod","latest_version":""}]}`
	assert.JSONEq(t, expectedBody, rr.Body.String())

	// Ensure all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestListModulesHandler_DBError(t *testing.T) {
	_, mock := setupMockDB(t) // Setup mock DB

	// Define expected SQL query (use regexp)
	expectedSQL := regexp.QuoteMeta(`
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
	`)

	// Expect the query to be executed and return an error
	mock.ExpectQuery(expectedSQL).WillReturnError(errors.New("database connection lost"))

	// --- Execute Request ---
	req, err := http.NewRequest("GET", "/api/v1/modules", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ListModulesHandler)
	handler.ServeHTTP(rr, req)

	// --- Assertions ---
	// Assert status code
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	// Assert error response body
	// Note: Need to read internal/api/response/response.go to confirm the exact JSON structure. Assuming {"error": "message"}
	expectedBody := `{"error":"Failed to retrieve modules"}`
	assert.JSONEq(t, expectedBody, rr.Body.String())

	// Ensure all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// --- Tests for ListModuleVersionsHandler ---

func TestListModuleVersionsHandler_Success(t *testing.T) {
	_, mock := setupMockDB(t)
	namespace := "my-org"
	moduleName := "my-module"
	moduleID := uuid.New()

	// Mock finding the module
	moduleRows := sqlmock.NewRows([]string{"id", "namespace", "name"}).
		AddRow(moduleID, namespace, moduleName)
	// GORM uses prepared statements, so ExpectQuery with args. Match GORM's LIMIT $3 pattern.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "modules" WHERE namespace = $1 AND name = $2 ORDER BY "modules"."id" LIMIT $3`)).
		WithArgs(namespace, moduleName, 1). // Add limit argument
		WillReturnRows(moduleRows)

	// Mock finding the versions
	versionRows := sqlmock.NewRows([]string{"version"}).
		AddRow("v1.0.0").
		AddRow("v1.1.0").
		AddRow("v0.9.0") // Unsorted initially
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "version" FROM "module_versions" WHERE module_id = $1 ORDER BY created_at DESC`)).
		WithArgs(moduleID).
		WillReturnRows(versionRows)

	// --- Execute Request ---
	req, err := http.NewRequest("GET", "/api/v1/modules/"+namespace+"/"+moduleName, nil)
	assert.NoError(t, err)
	// Set URL vars for mux
	req = mux.SetURLVars(req, map[string]string{
		"namespace":   namespace,
		"module_name": moduleName,
	})

	rr := httptest.NewRecorder()
	// Need a router to correctly handle mux.Vars
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/modules/{namespace}/{module_name}", ListModuleVersionsHandler)
	router.ServeHTTP(rr, req)

	// --- Assertions ---
	assert.Equal(t, http.StatusOK, rr.Code)
	// Note: The handler sorts versions semantically descending
	expectedBody := `{"namespace":"my-org","module_name":"my-module","versions":["v1.1.0","v1.0.0","v0.9.0"]}`
	assert.JSONEq(t, expectedBody, rr.Body.String())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListModuleVersionsHandler_ModuleNotFound(t *testing.T) {
	_, mock := setupMockDB(t)
	namespace := "my-org"
	moduleName := "nonexistent-module"

	// Mock finding the module returning not found. Match GORM's LIMIT $3 pattern.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "modules" WHERE namespace = $1 AND name = $2 ORDER BY "modules"."id" LIMIT $3`)).
		WithArgs(namespace, moduleName, 1). // Add limit argument
		WillReturnError(gorm.ErrRecordNotFound)

	// --- Execute Request ---
	req, err := http.NewRequest("GET", "/api/v1/modules/"+namespace+"/"+moduleName, nil)
	assert.NoError(t, err)
	req = mux.SetURLVars(req, map[string]string{
		"namespace":   namespace,
		"module_name": moduleName,
	})

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/modules/{namespace}/{module_name}", ListModuleVersionsHandler)
	router.ServeHTTP(rr, req)

	// --- Assertions ---
	assert.Equal(t, http.StatusNotFound, rr.Code)
	expectedBody := `{"error":"Module not found"}`
	assert.JSONEq(t, expectedBody, rr.Body.String())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListModuleVersionsHandler_DBErrorFindingModule(t *testing.T) {
	_, mock := setupMockDB(t)
	namespace := "my-org"
	moduleName := "error-module"
	dbErr := errors.New("connection refused")

	// Mock finding the module returning a generic error. Match GORM's LIMIT $3 pattern.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "modules" WHERE namespace = $1 AND name = $2 ORDER BY "modules"."id" LIMIT $3`)).
		WithArgs(namespace, moduleName, 1). // Add limit argument
		WillReturnError(dbErr)

	// --- Execute Request ---
	req, err := http.NewRequest("GET", "/api/v1/modules/"+namespace+"/"+moduleName, nil)
	assert.NoError(t, err)
	req = mux.SetURLVars(req, map[string]string{
		"namespace":   namespace,
		"module_name": moduleName,
	})

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/modules/{namespace}/{module_name}", ListModuleVersionsHandler)
	router.ServeHTTP(rr, req)

	// --- Assertions ---
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	expectedBody := `{"error":"Failed to retrieve module"}`
	assert.JSONEq(t, expectedBody, rr.Body.String())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListModuleVersionsHandler_DBErrorFindingVersions(t *testing.T) {
	_, mock := setupMockDB(t)
	namespace := "my-org"
	moduleName := "version-error-module"
	moduleID := uuid.New()
	dbErr := errors.New("table dropped")

	// Mock finding the module successfully. Match GORM's LIMIT $3 pattern.
	moduleRows := sqlmock.NewRows([]string{"id", "namespace", "name"}).
		AddRow(moduleID, namespace, moduleName)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "modules" WHERE namespace = $1 AND name = $2 ORDER BY "modules"."id" LIMIT $3`)).
		WithArgs(namespace, moduleName, 1). // Add limit argument
		WillReturnRows(moduleRows)

	// Mock finding the versions returning an error
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "version" FROM "module_versions" WHERE module_id = $1 ORDER BY created_at DESC`)).
		WithArgs(moduleID).
		WillReturnError(dbErr)

	// --- Execute Request ---
	req, err := http.NewRequest("GET", "/api/v1/modules/"+namespace+"/"+moduleName, nil)
	assert.NoError(t, err)
	req = mux.SetURLVars(req, map[string]string{
		"namespace":   namespace,
		"module_name": moduleName,
	})

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/modules/{namespace}/{module_name}", ListModuleVersionsHandler)
	router.ServeHTTP(rr, req)

	// --- Assertions ---
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	expectedBody := `{"error":"Failed to retrieve module versions"}`
	assert.JSONEq(t, expectedBody, rr.Body.String())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Tests for FetchModuleVersionArtifactHandler ---
