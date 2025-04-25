package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// RegisterRoutes sets up the API routes for the registry server.
func RegisterRoutes(router *mux.Router, authToken string) {
	// Define the base path for API v1
	apiV1 := router.PathPrefix("/api/v1").Subrouter()

	// --- Public Routes (No Auth Required) ---

	// List All Modules: GET /api/v1/modules
	apiV1.HandleFunc("/modules", ListModulesHandler).Methods("GET")

	// List Module Versions: GET /api/v1/modules/{namespace}/{module_name}
	apiV1.HandleFunc("/modules/{namespace}/{module_name}", ListModuleVersionsHandler).Methods("GET")

	// Fetch Module Version Artifact: GET /api/v1/modules/{namespace}/{module_name}/{version}/artifact
	apiV1.HandleFunc("/modules/{namespace}/{module_name}/{version}/artifact", FetchModuleVersionArtifactHandler).Methods("GET")

	// List Module Dependencies: GET /api/v1/modules/{namespace}/{module_name}/dependencies
	apiV1.HandleFunc("/modules/{namespace}/{module_name}/dependencies", HandleListModuleDependencies).Methods("GET")

	// List Module Version Dependencies: GET /api/v1/modules/{namespace}/{module_name}/{version}/dependencies
	// Note: Currently returns module-level dependencies, but validates version exists.
	apiV1.HandleFunc("/modules/{namespace}/{module_name}/{version}/dependencies", HandleListModuleVersionDependencies).Methods("GET")

	// Resolve Dependencies (Placeholder): GET /api/v1/resolve?module=...&version=...
	apiV1.HandleFunc("/resolve", HandleResolveDependencies).Methods("GET") // Query params handled in handler

	// --- Protected Routes (Auth Required) ---

	// Publish Module Version: POST /api/v1/modules/{namespace}/{module_name}/{version}
	// Wrap the handler with the authentication middleware
	publishHandler := http.HandlerFunc(PublishModuleVersionHandler)
	apiV1.Handle("/modules/{namespace}/{module_name}/{version}", ApplyAuth(publishHandler, authToken)).Methods("POST")

	// --- Health Check (Outside API versioning for simplicity) ---
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK")) // Explicitly ignore error
	}).Methods("GET")
}
