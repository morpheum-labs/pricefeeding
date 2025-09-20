package rpcscan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// RPCConfig represents the configuration for a network.
type (
	RPCConfig struct {
		NetworkID    string            `json:"network_id"`
		NameStd      string            `json:"name_1"`
		NameCoinr    string            `json:"name_2"`
		WrappedToken string            `json:"gas_token"`
		Endpoints    []string          `json:"endpoints"`
		ApprovalSrc  map[string]string `json:"check"`
	}

	EthereumClient struct {
		NetworkID    uint64
		mu           sync.RWMutex
		eth_client   *ethclient.Client
		last_updated time.Time
	}

	NetworkConfiguration struct {
		Networks  []RPCConfig `json:"networks"`
		ClientUse map[uint64]*EthereumClient
		mu        sync.RWMutex // Add mutex for thread-safe access
	}

	LatencyConcurrentBox struct {
		endpoint  string
		latency   time.Duration
		networkId uint64
	}

	// Config represents the application configuration
	Config struct {
		RootDir string `json:"root_dir"`
	}

	// ChainRegistryData represents the structure of chain registry JavaScript files
	ChainRegistryData struct {
		Name     string   `json:"name"`
		Chain    string   `json:"chain"`
		Icon     string   `json:"icon,omitempty"`
		RPC      []string `json:"rpc"`
		Features []struct {
			Name string `json:"name"`
		} `json:"features,omitempty"`
		Faucets        interface{} `json:"faucets,omitempty"` // Can be []string or string
		NativeCurrency struct {
			Name     string `json:"name"`
			Symbol   string `json:"symbol"`
			Decimals int    `json:"decimals"`
		} `json:"nativeCurrency"`
		InfoURL   string `json:"infoURL,omitempty"`
		ShortName string `json:"shortName,omitempty"`
		ChainID   int    `json:"chainId"`
		NetworkID int    `json:"networkId"`
		Explorers []struct {
			Name     string `json:"name"`
			URL      string `json:"url"`
			Icon     string `json:"icon,omitempty"`
			Standard string `json:"standard,omitempty"`
		} `json:"explorers,omitempty"`
		Testnet bool `json:"testnet,omitempty"`
	}
)

/*
now based on Rayingri (v1.14.8) go-ethereum package
look up the chainlist.org for all public mainnets on different networks and make a list to read all the RPC https endpoints as the chain RPC.

how build RPC switcher for given chain ID or network ID,
chainID and networkID are the same thing
assumed that each networkID contains multiple possible RPC endpoints and they will need to be constantly checked to select the best RRC connection endpoint.
we take the lowest latency as the bet connection PRC endpoint
make a loop function to check for each network the best RPC endpoint

without crashing anything parameters the program can select any RPC by networkID at anytime
*/

func checkLatencyCon(netID, endpoint_rpc string) LatencyConcurrentBox {
	start := time.Now()
	value, _ := strconv.ParseUint(netID, 10, 64)
	client, err := rpc.Dial(endpoint_rpc)
	if err != nil {
		return LatencyConcurrentBox{
			endpoint:  endpoint_rpc,
			latency:   0,
			networkId: value,
		}
	}
	defer client.Close()
	var result string
	err = client.Call(&result, "web3_clientVersion")
	if err != nil {
		return LatencyConcurrentBox{
			networkId: value,
			endpoint:  endpoint_rpc,
			latency:   0,
		}
	}

	return LatencyConcurrentBox{
		endpoint:  endpoint_rpc,
		networkId: value,
		latency:   time.Since(start),
	}
}

func Readendpts(conf *Config) *NetworkConfiguration {
	// Read the JSON file
	// Construct the file path
	_file_path := filepath.Join(conf.RootDir, "networks.json")
	file, err := os.Open(_file_path)
	if err != nil {
		log.Fatalf("Failed to open JSON file: %v", err)
	}
	defer file.Close()
	// Read the file content
	data, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read JSON file: %v", err)
	}
	// Unmarshal JSON data into Config struct
	var config NetworkConfiguration
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Failed to unmarshal JSON data: %v", err)
	}
	// Initialize the client map with proper mutex protection
	config.ClientUse = make(map[uint64]*EthereumClient)
	return &config
}

// Get the best RPC endpoint for each network (deprecated - use getBestRPCEndpointsParallel instead)
func getBestRPCEndpoints(netconf *NetworkConfiguration) (map[string]string, error) {
	bestEndpoints := make(map[string]string)
	for _, network := range netconf.Networks {
		bestEndpoint := ""
		lowestLatency := time.Duration(1<<63 - 1) // Set to maximum duration
		for _, endpoint := range network.Endpoints {
			latency := checkLatencyCon(network.NetworkID, endpoint)
			if latency.latency > 0 && latency.latency < lowestLatency {
				lowestLatency = latency.latency
				bestEndpoint = endpoint
			}
		}
		if bestEndpoint == "" {
			log.Printf("No working RPC endpoints found for network ID %s", network.NetworkID)
		} else {
			log.Printf("Best RPC endpoint for network %s: %s with latency %v", network.NetworkID, bestEndpoint, lowestLatency)
			bestEndpoints[network.NetworkID] = bestEndpoint
		}
	}
	return bestEndpoints, nil
}

func getBestRPCEndpointsParallel(netconf *NetworkConfiguration, timeout time.Duration) (map[uint64]string, error) {
	bestEndpoints := make(map[uint64]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process each network concurrently
	for _, network := range netconf.Networks {
		wg.Add(1)
		go func(network RPCConfig) {
			defer wg.Done()

			networkID, err := strconv.ParseUint(network.NetworkID, 10, 64)
			if err != nil {
				log.Printf("Invalid network ID: %s", network.NetworkID)
				return
			}

			// Channel to collect latency results for this network
			latencyChan := make(chan LatencyConcurrentBox, len(network.Endpoints))
			var endpointWg sync.WaitGroup

			// Test all endpoints for this network concurrently
			for _, endpoint := range network.Endpoints {
				endpointWg.Add(1)
				go func(ep string) {
					defer endpointWg.Done()

					// Use context with timeout to prevent hanging
					ctx, cancel := context.WithTimeout(context.Background(), timeout)
					defer cancel()

					// Create a channel to receive the result
					resultChan := make(chan LatencyConcurrentBox, 1)

					go func() {
						resultChan <- checkLatencyCon(network.NetworkID, ep)
					}()

					select {
					case result := <-resultChan:
						latencyChan <- result
					case <-ctx.Done():
						// Timeout occurred, send a failed result
						latencyChan <- LatencyConcurrentBox{
							endpoint:  ep,
							latency:   0,
							networkId: networkID,
						}
					}
				}(endpoint)
			}

			// Close the latency channel when all endpoint tests are done
			go func() {
				endpointWg.Wait()
				close(latencyChan)
			}()

			// Find the best endpoint for this network
			var bestEndpoint string
			var bestLatency time.Duration = time.Duration(1<<63 - 1) // Max duration

			for result := range latencyChan {
				if result.latency > 0 && result.latency < bestLatency {
					bestLatency = result.latency
					bestEndpoint = result.endpoint
				}
			}

			// Store the best endpoint for this network
			if bestEndpoint != "" {
				mu.Lock()
				bestEndpoints[networkID] = bestEndpoint
				mu.Unlock()
				log.Printf("Best RPC endpoint for network %s: %s with latency %v", network.NetworkID, bestEndpoint, bestLatency)
			} else {
				log.Printf("No working RPC endpoints found for network ID %s", network.NetworkID)
			}
		}(network)
	}

	// Wait for all networks to be processed
	wg.Wait()
	return bestEndpoints, nil
}

// MonitorAllRPCEndpoints monitors all network RPC endpoints continuously
func MonitorAllRPCEndpoints(conf *Config, netconf *NetworkConfiguration, interval time.Duration, stopChan chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			log.Println("Stopping monitoring for all networks")
			return
		case <-ticker.C:
			start := time.Now()
			log.Printf("Starting RPC endpoint monitoring at %s", start.Format(time.RFC3339))

			bestEndpoints, err := getBestRPCEndpointsParallel(netconf, 5*time.Second)
			if err != nil {
				log.Printf("Error finding best RPC endpoints: %v", err)
				continue
			}

			// Update clients with proper synchronization
			for networkID, endpoint := range bestEndpoints {
				client, err := NewEthereumClient(endpoint)
				if err != nil {
					log.Printf("Failed to create client for network %d: %v", networkID, err)
					continue
				}

				client.NetworkID = networkID
				client.last_updated = time.Now()

				// Thread-safe client update
				netconf.mu.Lock()
				netconf.ClientUse[networkID] = client
				netconf.mu.Unlock()
				log.Printf("Updated client for network %d to use endpoint: %s", networkID, endpoint)
			}

			log.Printf("RPC monitoring completed in %v", time.Since(start))
		}
	}
}

// NewEthereumClient creates a new Ethereum client.
func NewEthereumClient(endpoint string) (*EthereumClient, error) {
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		return nil, err
	}
	return &EthereumClient{eth_client: client}, nil
}

// GetClient returns the underlying ethclient.Client
func (q *EthereumClient) GetClient() *ethclient.Client {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.eth_client
}

// GetNetworkID returns the network ID
func (q *EthereumClient) GetNetworkID() uint64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.NetworkID
}

// GetLastUpdated returns when the client was last updated
func (q *EthereumClient) GetLastUpdated() time.Time {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.last_updated
}

func RuntimeWeb3Selection(conf *Config) (chan struct{}, *NetworkConfiguration) {
	stopChan := make(chan struct{})

	// Load network configuration from both JS and JSON files
	data, err := LoadChainRegistryConfig(conf)
	if err != nil {
		log.Printf("Warning: failed to load chain registry config: %v", err)
		// Fall back to JSON only
		data = Readendpts(conf)
	}

	// Start monitoring with 30-second intervals
	go MonitorAllRPCEndpoints(conf, data, 30*time.Second, stopChan)

	return stopChan, data
}

// Function to find keys by address
func FindKeysByAddress(check map[string]string, address string) string {
	tag := ""
	// Iterate through the map to find matching addresses
	for key, value := range check {
		if value == address {
			tag = key
		}
	}
	return tag
}

// GetBestClient returns the best available client for a given network ID
func (netconf *NetworkConfiguration) GetBestClient(networkID uint64) (*EthereumClient, error) {
	netconf.mu.RLock()
	defer netconf.mu.RUnlock()

	client, exists := netconf.ClientUse[networkID]
	if !exists {
		return nil, fmt.Errorf("no client available for network ID %d", networkID)
	}
	return client, nil
}

// GetAllNetworkIDs returns all available network IDs
func (netconf *NetworkConfiguration) GetAllNetworkIDs() []uint64 {
	var networkIDs []uint64
	for _, network := range netconf.Networks {
		if id, err := strconv.ParseUint(network.NetworkID, 10, 64); err == nil {
			networkIDs = append(networkIDs, id)
		}
	}
	return networkIDs
}

// GetAllClients returns a copy of all clients for safe iteration
func (netconf *NetworkConfiguration) GetAllClients() map[uint64]*EthereumClient {
	netconf.mu.RLock()
	defer netconf.mu.RUnlock()

	clients := make(map[uint64]*EthereumClient)
	for networkID, client := range netconf.ClientUse {
		clients[networkID] = client
	}
	return clients
}

// Contains checks if a slice contains a specific element
func contains(slice []uint64, element uint64) bool {
	for _, item := range slice {
		if item == element {
			return true
		}
	}
	return false
}

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
