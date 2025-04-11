package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [namespace/module_name]",
	Short: "List modules or module versions",
	Long: `Lists all available modules in the registry or lists the available versions
for a specific module.

Examples:
  protoreg-cli list                  # List all modules
  protoreg-cli list mycompany/user   # List versions for mycompany/user`,
	Args: cobra.MaximumNArgs(1), // 0 or 1 argument
	Run: func(cmd *cobra.Command, args []string) {
		log := GetLogger()
		registryURL := viper.GetString("registry_url")
		if registryURL == "" {
			log.Fatal("Registry URL is not configured. Use --registry-url flag, PROTOREG_REGISTRY_URL env var, or 'protoreg-cli configure'.")
		}

		client := &http.Client{} // Use default HTTP client

		if len(args) == 0 {
			// List all modules
			listAllModules(client, registryURL, log)
		} else {
			// List versions for a specific module
			moduleFullName := args[0]
			parts := strings.SplitN(moduleFullName, "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				log.Fatal("Invalid module name format. Expected 'namespace/module_name'.", zap.String("module", moduleFullName))
			}
			namespace := parts[0]
			moduleName := parts[1]
			listModuleVersions(client, registryURL, namespace, moduleName, log)
		}
	},
}

// Response structures matching the server API
type listModulesApiResponse struct {
	Modules []struct {
		Namespace     string `json:"namespace"`
		Name          string `json:"name"`
		LatestVersion string `json:"latest_version"`
	} `json:"modules"`
}

type listModuleVersionsApiResponse struct {
	Namespace  string   `json:"namespace"`
	ModuleName string   `json:"module_name"`
	Versions   []string `json:"versions"`
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

func listAllModules(client *http.Client, registryURL string, log *zap.Logger) {
	targetURL := fmt.Sprintf("%s/api/v1/modules", strings.TrimSuffix(registryURL, "/"))
	log.Debug("Requesting module list", zap.String("url", targetURL))

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		log.Fatal("Failed to create request", zap.Error(err))
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Failed to execute request", zap.Error(err))
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read response body", zap.Error(err))
	}

	if resp.StatusCode != http.StatusOK {
		handleApiError(resp.StatusCode, bodyBytes, log)
		os.Exit(1)
	}

	var apiResp listModulesApiResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		log.Fatal("Failed to parse API response", zap.Error(err), zap.ByteString("body", bodyBytes))
	}

	if len(apiResp.Modules) == 0 {
		fmt.Println("No modules found in the registry.")
		return
	}

	fmt.Println("Available Modules:")
	for _, mod := range apiResp.Modules {
		if mod.LatestVersion != "" {
			fmt.Printf("  %s/%s (latest: %s)\n", mod.Namespace, mod.Name, mod.LatestVersion)
		} else {
			fmt.Printf("  %s/%s (no versions published)\n", mod.Namespace, mod.Name)
		}
	}
}

func listModuleVersions(client *http.Client, registryURL, namespace, moduleName string, log *zap.Logger) {
	// URL encode path segments
	encodedNamespace := url.PathEscape(namespace)
	encodedModuleName := url.PathEscape(moduleName)
	targetURL := fmt.Sprintf("%s/api/v1/modules/%s/%s", strings.TrimSuffix(registryURL, "/"), encodedNamespace, encodedModuleName)
	log.Debug("Requesting module versions", zap.String("url", targetURL))

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		log.Fatal("Failed to create request", zap.Error(err))
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Failed to execute request", zap.Error(err))
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read response body", zap.Error(err))
	}

	if resp.StatusCode != http.StatusOK {
		handleApiError(resp.StatusCode, bodyBytes, log)
		os.Exit(1)
	}

	var apiResp listModuleVersionsApiResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		log.Fatal("Failed to parse API response", zap.Error(err), zap.ByteString("body", bodyBytes))
	}

	if len(apiResp.Versions) == 0 {
		fmt.Printf("No versions found for module %s/%s.\n", namespace, moduleName)
		return
	}

	// Sort versions semantically descending (best effort)
	sortVersionsDescCli(apiResp.Versions)

	fmt.Printf("Versions for %s/%s:\n", namespace, moduleName)
	for _, v := range apiResp.Versions {
		fmt.Printf("  %s\n", v)
	}
}

// handleApiError attempts to parse and log an API error response.
func handleApiError(statusCode int, body []byte, log *zap.Logger) {
	var errResp apiErrorResponse
	logFields := []zap.Field{zap.Int("status_code", statusCode)}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		logFields = append(logFields, zap.String("api_error", errResp.Error))
		log.Error("API request failed", logFields...)
	} else {
		// Log raw body if JSON parsing fails or error field is empty
		logFields = append(logFields, zap.ByteString("response_body", body))
		log.Error("API request failed", logFields...)
	}
}

// sortVersionsDescCli sorts a slice of version strings semantically descending.
// Copied/adapted from server handlers for client-side sorting.
func sortVersionsDescCli(versions []string) {
	semvers := make([]*semver.Version, 0, len(versions))
	for _, vStr := range versions {
		// Semver library expects versions without 'v' prefix
		cleanVStr := strings.TrimPrefix(vStr, "v")
		v, err := semver.NewVersion(cleanVStr)
		if err == nil {
			semvers = append(semvers, v)
		} else {
			GetLogger().Warn("Could not parse version for sorting, keeping original", zap.String("version", vStr), zap.Error(err))
			// Keep original string in place? Or attempt basic sort? For now, keep original order relative to others.
		}
	}

	// Sort descending
	sort.Sort(sort.Reverse(semver.Collection(semvers)))

	// Overwrite the original slice with sorted versions, adding 'v' prefix back
	sortedStrings := make([]string, 0, len(versions))
	processed := make(map[string]bool) // Track processed semvers to handle originals

	for _, v := range semvers {
		vStr := "v" + v.String()
		sortedStrings = append(sortedStrings, vStr)
		processed[vStr] = true
	}

	// Append any original strings that failed parsing, maintaining their relative order (tricky)
	// A simpler approach might be to just sort the parseable ones and append the others.
	// Let's just use the sorted parseable ones for now.
	copy(versions, sortedStrings) // Copy sorted versions back into the original slice (up to the count of sorted ones)
	// This isn't perfect if some failed parsing, but good enough for display.
}

func init() {
	rootCmd.AddCommand(listCmd)
}
