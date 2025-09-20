package rpcscan

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ReadChainRegistryFromJS reads chain configurations from JavaScript files in the additionalChainRegistry directory
func ReadChainRegistryFromJS(conf *Config) (*NetworkConfiguration, error) {
	// Construct the path to the additionalChainRegistry directory
	registryPath := filepath.Join(conf.RootDir, "constants", "additionalChainRegistry")

	// Check if the directory exists
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("additionalChainRegistry directory not found: %s", registryPath)
	}

	// Read all JavaScript files in the directory
	files, err := filepath.Glob(filepath.Join(registryPath, "chainid-*.js"))
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var networks []RPCConfig
	clientUse := make(map[uint64]*EthereumClient)

	for _, file := range files {
		chainData, err := parseChainRegistryFile(file)
		if err != nil {
			log.Printf("Warning: failed to parse %s: %v", file, err)
			continue
		}

		// Convert ChainRegistryData to RPCConfig
		rpcConfig := convertToRPCConfig(chainData)
		networks = append(networks, rpcConfig)
	}

	return &NetworkConfiguration{
		Networks:  networks,
		ClientUse: clientUse,
	}, nil
}

// parseChainRegistryFile parses a single JavaScript chain registry file
func parseChainRegistryFile(filePath string) (*ChainRegistryData, error) {
	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Convert JavaScript export to JSON-like format
	jsonContent, err := convertJSToJSON(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to convert JS to JSON: %w", err)
	}

	// Parse JSON
	var chainData ChainRegistryData
	if err := json.Unmarshal([]byte(jsonContent), &chainData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &chainData, nil
}

// convertJSToJSON converts JavaScript export format to JSON format
func convertJSToJSON(jsContent string) (string, error) {
	// Remove export const data = and trailing semicolon
	re := regexp.MustCompile(`export\s+const\s+data\s*=\s*`)
	jsonContent := re.ReplaceAllString(jsContent, "")

	// Remove any semicolons that might be in the middle of the content (after closing braces)
	jsonContent = regexp.MustCompile(`;(\s*[}\]])`).ReplaceAllString(jsonContent, "$1")

	// Remove trailing semicolon if present (handle multiple semicolons)
	for strings.HasSuffix(jsonContent, ";") {
		jsonContent = strings.TrimSuffix(jsonContent, ";")
	}
	jsonContent = strings.TrimSpace(jsonContent)

	// Handle single quotes by converting to double quotes (basic approach)
	// This is a simplified approach - for production, consider using a proper JS parser
	jsonContent = strings.ReplaceAll(jsonContent, "'", "\"")

	// Fix common JavaScript object formatting issues
	// Remove trailing commas before closing braces/brackets
	jsonContent = regexp.MustCompile(`,(\s*[}\]])`).ReplaceAllString(jsonContent, "$1")

	// Remove trailing commas after objects in arrays (like }, ])
	jsonContent = regexp.MustCompile(`},(\s*\])`).ReplaceAllString(jsonContent, "}$1")

	// Fix unquoted property names (basic approach) - but be more careful
	// Only match property names that are at the beginning of a line or after a comma/brace
	// and are not already quoted and not part of a URL
	jsonContent = regexp.MustCompile(`([,{]\s*)(\w+):`).ReplaceAllString(jsonContent, `$1"$2":`)

	// Fix the first property if it's unquoted
	jsonContent = regexp.MustCompile(`^\s*{\s*(\w+):`).ReplaceAllString(jsonContent, `{"$1":`)

	// Final cleanup - remove any remaining semicolons
	jsonContent = strings.ReplaceAll(jsonContent, ";", "")
	jsonContent = strings.TrimSpace(jsonContent)

	return jsonContent, nil
}

// convertToRPCConfig converts ChainRegistryData to RPCConfig format
func convertToRPCConfig(chainData *ChainRegistryData) RPCConfig {
	// Create approval source map (empty for now, can be populated with price feed addresses)
	approvalSrc := make(map[string]string)

	// Determine wrapped token address based on chain
	var wrappedToken string
	switch chainData.ChainID {
	case 1: // Ethereum mainnet
		wrappedToken = "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2" // WETH
	case 42161: // Arbitrum mainnet
		wrappedToken = "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1" // WETH
	default:
		// For other chains, use a placeholder or determine based on chain
		wrappedToken = "0x0000000000000000000000000000000000000000"
	}

	return RPCConfig{
		NetworkID:    strconv.Itoa(chainData.ChainID),
		NameStd:      chainData.Name,
		NameCoinr:    chainData.NativeCurrency.Symbol,
		WrappedToken: wrappedToken,
		Endpoints:    chainData.RPC,
		ApprovalSrc:  approvalSrc,
	}
}

// LoadChainRegistryConfig loads chain configurations from both JSON and JavaScript files
func LoadChainRegistryConfig(conf *Config) (*NetworkConfiguration, error) {
	// First try to load from JavaScript files
	jsConfig, err := ReadChainRegistryFromJS(conf)
	if err != nil {
		log.Printf("Warning: failed to load from JS files: %v", err)
		// Fall back to JSON file
		return Readendpts(conf), nil
	}

	// If we have both JS and JSON configs, merge them
	jsonConfig := Readendpts(conf)

	// Merge networks from both sources
	allNetworks := append(jsConfig.Networks, jsonConfig.Networks...)

	// Merge client use maps
	mergedClientUse := make(map[uint64]*EthereumClient)
	for k, v := range jsConfig.ClientUse {
		mergedClientUse[k] = v
	}
	for k, v := range jsonConfig.ClientUse {
		mergedClientUse[k] = v
	}

	return &NetworkConfiguration{
		Networks:  allNetworks,
		ClientUse: mergedClientUse,
	}, nil
}

// Example usage function to demonstrate how to use the new chain registry functionality
func ExampleLoadChainRegistry() {
	// Create a config pointing to your project root
	conf := &Config{
		RootDir: "/Users/hesdx/Documents/b95/swapbiz/chainlink-price-feed",
	}

	// Load chain configurations from JavaScript files
	networkConfig, err := ReadChainRegistryFromJS(conf)
	if err != nil {
		log.Printf("Error loading chain registry: %v", err)
		return
	}

	// Print information about loaded networks
	log.Printf("Loaded %d networks from chain registry:", len(networkConfig.Networks))
	for _, network := range networkConfig.Networks {
		log.Printf("Network ID: %s, Name: %s, Symbol: %s, RPC Endpoints: %d",
			network.NetworkID, network.NameStd, network.NameCoinr, len(network.Endpoints))
	}

	// You can also use the combined loader that merges JS and JSON configs
	combinedConfig, err := LoadChainRegistryConfig(conf)
	if err != nil {
		log.Printf("Error loading combined config: %v", err)
		return
	}

	log.Printf("Combined config has %d networks", len(combinedConfig.Networks))
}
