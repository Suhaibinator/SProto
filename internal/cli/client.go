package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Suhaibinator/SProto/internal/api"      // Import API response types
	"github.com/Suhaibinator/SProto/internal/resolver" // Added import
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

// GetModuleInfo implements the resolver.RegistryAccessor interface.
// Retrieves module details including namespace, name, and import path.
func (c *RegistryClient) GetModuleInfo(namespace, name string) (*resolver.ModuleInfo, error) {
	c.Logger.Debug("Fetching module metadata (GetModuleInfo)", zap.String("namespace", namespace), zap.String("name", name))

	// Fetch all modules and filter client-side for now
	allModules, err := c.FetchAllModules()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all modules to find metadata for %s/%s: %w", namespace, name, err)
	}

	for _, apiModuleInfo := range allModules {
		if apiModuleInfo.Namespace == namespace && apiModuleInfo.Name == name {
			c.Logger.Debug("Found module metadata", zap.String("module", fmt.Sprintf("%s/%s", namespace, name)))
			// Convert api.ModuleInfo to resolver.ModuleInfo
			resolverInfo := &resolver.ModuleInfo{
				Namespace:  apiModuleInfo.Namespace,
				Name:       apiModuleInfo.Name,
				ImportPath: apiModuleInfo.ImportPath,
			}
			return resolverInfo, nil
		}
	}

	return nil, fmt.Errorf("module '%s/%s' not found in registry", namespace, name)
}

// FetchAllModules retrieves metadata for all modules from the registry.
// Corresponds to GET /api/v1/modules.
// Note: Returns api.ModuleInfo, used internally by GetModuleInfo.
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

// GetModuleVersions implements the resolver.RegistryAccessor interface.
// Retrieves all available versions for a specific module.
func (c *RegistryClient) GetModuleVersions(namespace, name string) ([]string, error) {
	c.Logger.Debug("Fetching module versions (GetModuleVersions)", zap.String("namespace", namespace), zap.String("name", name))
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

// GetModuleDependencies implements the resolver.RegistryAccessor interface.
// Retrieves a list of dependencies for a specific module.
func (c *RegistryClient) GetModuleDependencies(namespace, name string) ([]resolver.DependencyInfo, error) {
	c.Logger.Debug("Fetching module dependencies (GetModuleDependencies)", zap.String("namespace", namespace), zap.String("name", name))
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

	// Convert api.DependencyResponse to resolver.DependencyInfo
	resolverDeps := make([]resolver.DependencyInfo, 0, len(listResp.Dependencies))
	if listResp.Dependencies != nil {
		for _, dep := range listResp.Dependencies {
			resolverDeps = append(resolverDeps, resolver.DependencyInfo{
				Namespace:         dep.Namespace,
				Name:              dep.Name,
				VersionConstraint: dep.VersionConstraint,
				// ImportPath is not part of DependencyInfo, it's fetched via GetModuleInfo
			})
		}
	}

	return resolverDeps, nil
}

// -- Backward Compatibility Methods --

// FetchModuleMetadata is a backward-compatibility wrapper around GetModuleInfo.
// This maintains compatibility with existing code that expects the old method.
func (c *RegistryClient) FetchModuleMetadata(namespace, name string) (*api.ModuleInfo, error) {
	resolverInfo, err := c.GetModuleInfo(namespace, name)
	if err != nil {
		return nil, err
	}
	// Convert resolver.ModuleInfo back to api.ModuleInfo
	return &api.ModuleInfo{
		Namespace:  resolverInfo.Namespace,
		Name:       resolverInfo.Name,
		ImportPath: resolverInfo.ImportPath,
	}, nil
}

// FetchModuleVersions is a backward-compatibility wrapper around GetModuleVersions.
// This maintains compatibility with existing code that expects the old method.
func (c *RegistryClient) FetchModuleVersions(namespace, name string) ([]string, error) {
	return c.GetModuleVersions(namespace, name)
}

// FetchModuleDependencies is a backward-compatibility wrapper around GetModuleDependencies.
// This maintains compatibility with existing code that expects the old method.
func (c *RegistryClient) FetchModuleDependencies(namespace, name string) ([]api.DependencyResponse, error) {
	resolverDeps, err := c.GetModuleDependencies(namespace, name)
	if err != nil {
		return nil, err
	}
	// Convert resolver.DependencyInfo to api.DependencyResponse
	apiDeps := make([]api.DependencyResponse, 0, len(resolverDeps))
	for _, dep := range resolverDeps {
		apiDeps = append(apiDeps, api.DependencyResponse{
			Namespace:         dep.Namespace,
			Name:              dep.Name,
			VersionConstraint: dep.VersionConstraint,
			// ImportPath will be empty, but should not be needed by existing code
		})
	}
	return apiDeps, nil
}

// ResolveDependencies resolves the dependencies for a module using the registry API
// This method redirects to the server-side dependency resolution endpoint (Task 1.3.2)
func (c *RegistryClient) ResolveDependencies(namespace, name, version string) (map[string]string, error) {
	c.Logger.Debug("Resolving dependencies via API",
		zap.String("module", fmt.Sprintf("%s/%s@%s", namespace, name, version)))

	// URL encode path segments
	encodedNamespace := url.PathEscape(namespace)
	encodedName := url.PathEscape(name)
	encodedVersion := url.PathEscape(version)
	targetURL := fmt.Sprintf("%s/api/v1/modules/%s/%s/%s/resolve",
		c.RegistryURL, encodedNamespace, encodedName, encodedVersion)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to resolve dependencies: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies from %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to resolve dependencies: received status %d %s, body: %s",
			resp.StatusCode, http.StatusText(resp.StatusCode), string(bodyBytes))
	}

	// Use the generic map structure to avoid type errors since the API structure
	// already exists in handlers.go
	var resolveResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&resolveResp); err != nil {
		return nil, fmt.Errorf("failed to decode resolve dependencies response: %w", err)
	}

	// Extract the resolved_dependencies map
	resolvedDeps := make(map[string]string)
	if depMap, ok := resolveResp["resolved_dependencies"].(map[string]interface{}); ok {
		for modID, version := range depMap {
			if versionStr, ok := version.(string); ok {
				resolvedDeps[modID] = versionStr
			}
		}
	}

	c.Logger.Debug("Successfully resolved dependencies via API",
		zap.String("module", fmt.Sprintf("%s/%s@%s", namespace, name, version)),
		zap.Int("resolved_count", len(resolvedDeps)))

	return resolvedDeps, nil
}
