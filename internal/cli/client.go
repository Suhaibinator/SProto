package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Suhaibinator/SProto/internal/api" // Import API response types
	"go.uber.org/zap"
)

// RegistryClient is a client for interacting with the SProto registry API.
type RegistryClient struct {
	RegistryURL string
	APIToken    string
	HTTPClient  *http.Client
	Logger      *zap.Logger
}

// NewRegistryClient creates a new instance of RegistryClient.
func NewRegistryClient(registryURL, apiToken string, logger *zap.Logger) *RegistryClient {
	// Use a default HTTP client with a timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	return &RegistryClient{
		RegistryURL: strings.TrimSuffix(registryURL, "/"), // Remove trailing slash
		APIToken:    apiToken,
		HTTPClient:  httpClient,
		Logger:      logger,
	}
}

// FetchModuleMetadata retrieves module details (including import path and latest version) from the registry.
// Corresponds to GET /api/v1/modules/{namespace}/{module_name} but adapted to return ModuleInfo.
// Note: The API currently returns versions, not full Module metadata here.
// We might need a new API endpoint or adapt existing ones.
// For now, let's fetch versions and infer latest, and maybe need a separate call for import path if not in list.
// Re-reading Task 1.3.3, the ListModulesHandler *does* include import_path.
// Let's adapt to fetch a single module's details if possible, or fetch all and filter.
// The API spec in README shows GET /api/v1/modules/{namespace}/{module_name} lists versions.
// GET /api/v1/modules lists modules with latest version.
// We need a way to get a single module's full metadata including import path.
// Let's assume for now we can fetch all modules and find the one we need.
// TODO: Consider adding a dedicated GET /api/v1/modules/{namespace}/{module_name}/metadata endpoint if needed.
func (c *RegistryClient) FetchModuleMetadata(namespace, name string) (*api.ModuleInfo, error) {
	c.Logger.Debug("Fetching module metadata", zap.String("namespace", namespace), zap.String("name", name))

	// Fetch all modules and filter client-side for now
	allModules, err := c.FetchAllModules()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all modules to find metadata for %s/%s: %w", namespace, name, err)
	}

	for _, moduleInfo := range allModules {
		if moduleInfo.Namespace == namespace && moduleInfo.Name == name {
			c.Logger.Debug("Found module metadata", zap.String("module", fmt.Sprintf("%s/%s", namespace, name)))
			return &moduleInfo, nil
		}
	}

	return nil, fmt.Errorf("module '%s/%s' not found in registry", namespace, name)
}

// FetchAllModules retrieves metadata for all modules from the registry.
// Corresponds to GET /api/v1/modules.
func (c *RegistryClient) FetchAllModules() ([]api.ModuleInfo, error) {
	c.Logger.Debug("Fetching all modules")
	targetURL := fmt.Sprintf("%s/api/v1/modules", c.RegistryURL)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to list modules: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list modules from %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list modules: received status %d %s, body: %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(bodyBytes))
	}

	var listResp api.ListModulesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode list modules response: %w", err)
	}

	c.Logger.Debug("Successfully fetched all modules", zap.Int("count", len(listResp.Modules)))
	return listResp.Modules, nil
}

// FetchModuleVersions retrieves all versions for a specific module.
// Corresponds to GET /api/v1/modules/{namespace}/{module_name}.
func (c *RegistryClient) FetchModuleVersions(namespace, name string) ([]string, error) {
	c.Logger.Debug("Fetching module versions", zap.String("namespace", namespace), zap.String("name", name))
	// URL encode path segments
	encodedNamespace := url.PathEscape(namespace)
	encodedName := url.PathEscape(name)
	targetURL := fmt.Sprintf("%s/api/v1/modules/%s/%s", c.RegistryURL, encodedNamespace, encodedName)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to list module versions: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list module versions from %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list module versions: received status %d %s, body: %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(bodyBytes))
	}

	var listResp api.ListModuleVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode list module versions response: %w", err)
	}

	c.Logger.Debug("Successfully fetched module versions", zap.String("module", fmt.Sprintf("%s/%s", namespace, name)), zap.Int("count", len(listResp.Versions)))
	return listResp.Versions, nil
}

// FetchArtifact downloads the artifact for a specific module version.
// Corresponds to GET /api/v1/modules/{namespace}/{module_name}/{version}/artifact.
// Returns an io.ReadCloser for the artifact content.
func (c *RegistryClient) FetchArtifact(namespace, name, version string) (io.ReadCloser, error) {
	c.Logger.Debug("Fetching artifact", zap.String("module", fmt.Sprintf("%s/%s", namespace, name)), zap.String("version", version))
	// URL encode path segments
	encodedNamespace := url.PathEscape(namespace)
	encodedName := url.PathEscape(name)
	encodedVersion := url.PathEscape(version)
	targetURL := fmt.Sprintf("%s/api/v1/modules/%s/%s/%s/artifact", c.RegistryURL, encodedNamespace, encodedName, encodedVersion)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to fetch artifact: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch artifact from %s: %w", targetURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch artifact: received status %d %s, body: %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(bodyBytes))
	}

	c.Logger.Debug("Successfully initiated artifact download", zap.String("module", fmt.Sprintf("%s/%s", namespace, name)), zap.String("version", version))
	return resp.Body, nil // Caller is responsible for closing the body
}

// FetchModuleDependencies retrieves the list of dependencies for a specific module.
// Corresponds to GET /api/v1/modules/{namespace}/{module_name}/dependencies.
func (c *RegistryClient) FetchModuleDependencies(namespace, name string) ([]api.DependencyResponse, error) {
	c.Logger.Debug("Fetching module dependencies", zap.String("namespace", namespace), zap.String("name", name))
	// URL encode path segments
	encodedNamespace := url.PathEscape(namespace)
	encodedName := url.PathEscape(name)
	targetURL := fmt.Sprintf("%s/api/v1/modules/%s/%s/dependencies", c.RegistryURL, encodedNamespace, encodedName)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to list module dependencies: %w", err)
	}

	// Add authentication if needed (assuming dependencies endpoint might require it)
	// if c.APIToken != "" {
	// 	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	// }

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list module dependencies from %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		// Handle 404 specifically - module might exist but have no dependencies defined yet
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("module '%s/%s' not found when fetching dependencies", namespace, name)
		}
		return nil, fmt.Errorf("failed to list module dependencies: received status %d %s, body: %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(bodyBytes))
	}

	var listResp api.ListModuleDependenciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode list module dependencies response: %w", err)
	}

	c.Logger.Debug("Successfully fetched module dependencies", zap.String("module", fmt.Sprintf("%s/%s", namespace, name)), zap.Int("count", len(listResp.Dependencies)))
	// Ensure empty slice, not null, if no dependencies
	if listResp.Dependencies == nil {
		return []api.DependencyResponse{}, nil
	}
	return listResp.Dependencies, nil
}

// TODO: Add method for resolving dependencies (Task 1.3.2 endpoint)
// TODO: Add method for publishing (adapt from internal/cli/publish.go)
