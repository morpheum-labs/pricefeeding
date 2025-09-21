package rpcscan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aelmanaa/chainlink-price-feed-golang/shared"
	"gopkg.in/yaml.v3"
)

// ExtraRPCConfig represents the structure of the extraRpcs.json file
type ExtraRPCConfig struct {
	RPCs []interface{} `json:"rpcs"`
}

// ExtraRPCsData represents the complete structure of extraRpcs.json
type ExtraRPCsData map[string]ExtraRPCConfig

// RPCEndpoint represents a single RPC endpoint with tracking info
type RPCEndpoint struct {
	URL             string `json:"url"`
	Tracking        string `json:"tracking"`
	TrackingDetails string `json:"trackingDetails"`
}

// NetworkInfo contains network metadata for common networks
type NetworkInfo struct {
	NameStd      string
	NameCoinr    string
	WrappedToken string
}

// ExtendedConfig extends the shared Configuration with RPC-specific settings
type ExtendedConfig struct {
	shared.Configuration
	Monitoring struct {
		RPCCheckInterval   int `yaml:"rpc_check_interval"`
		PriceFetchInterval int `yaml:"price_fetch_interval"`
		RPCTimeout         int `yaml:"rpc_timeout"`
		MaxConcurrentCalls int `yaml:"max_concurrent_calls"`
	} `yaml:"monitoring"`

	PriceFeeds map[string]struct {
		ChainID int `yaml:"chain_id"`
		Feeds   []struct {
			Name     string `yaml:"name"`
			Address  string `yaml:"address"`
			Decimals int    `yaml:"decimals"`
		} `yaml:"feeds"`
	} `yaml:"price_feeds"`

	Cache struct {
		Enabled    bool `yaml:"enabled"`
		Expiration int  `yaml:"expiration"`
		MaxSize    int  `yaml:"max_size"`
	} `yaml:"cache"`
}

// LoadYamlConfig loads the YAML configuration file
func LoadYamlConfig(configPath string) (*ExtendedConfig, error) {
	// If the path is a directory, append the default config file name
	if info, err := os.Stat(configPath); err == nil && info.IsDir() {
		configPath = filepath.Join(configPath, "vault_config.yaml")
	}

	fileContent, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	var config ExtendedConfig
	if err := yaml.Unmarshal(fileContent, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %w", err)
	}

	// Validate required fields
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validateConfig validates the configuration parameters
func validateConfig(config *ExtendedConfig) error {
	if config.Port == 0 {
		return fmt.Errorf("port configuration parameter not found")
	}
	if config.SecretHash == "" {
		return fmt.Errorf("secret_hash configuration parameter not found")
	}
	if config.Database.Postgres.DBConn == "" {
		return fmt.Errorf("db_conn configuration parameter not found")
	}
	if config.Database.Postgres.DBConnPool == 0 {
		return fmt.Errorf("db_conn_pool configuration parameter not found")
	}
	if len(config.ArbitrumRPCs.URLs) == 0 {
		return fmt.Errorf("arbitrum_rpcs urls configuration parameter not found")
	}
	if len(config.EthereumRPCs.URLs) == 0 {
		return fmt.Errorf("ethereum_rpcs urls configuration parameter not found")
	}

	// Set defaults for monitoring if not specified
	if config.Monitoring.RPCCheckInterval == 0 {
		config.Monitoring.RPCCheckInterval = 30
	}
	if config.Monitoring.PriceFetchInterval == 0 {
		config.Monitoring.PriceFetchInterval = 30
	}
	if config.Monitoring.RPCTimeout == 0 {
		config.Monitoring.RPCTimeout = 10
	}
	if config.Monitoring.MaxConcurrentCalls == 0 {
		config.Monitoring.MaxConcurrentCalls = 10
	}

	// Set defaults for cache if not specified
	if !config.Cache.Enabled {
		config.Cache.Enabled = true
	}
	if config.Cache.Expiration == 0 {
		config.Cache.Expiration = 300
	}
	if config.Cache.MaxSize == 0 {
		config.Cache.MaxSize = 1000
	}

	return nil
}

// LoadExtraRPCs loads the extraRpcs.json file
func LoadExtraRPCs(filePath string) (*ExtraRPCsData, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read extraRpcs.json: %w", err)
	}

	var extraRPCs ExtraRPCsData
	if err := json.Unmarshal(fileContent, &extraRPCs); err != nil {
		return nil, fmt.Errorf("failed to parse extraRpcs.json: %w", err)
	}

	return &extraRPCs, nil
}

// getNetworkInfo returns network metadata for common networks
func getNetworkInfo(chainID string) NetworkInfo {
	networkMap := map[string]NetworkInfo{
		"1": {
			NameStd:      "Ethereum Mainnet",
			NameCoinr:    "ETH",
			WrappedToken: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
		},
		"42161": {
			NameStd:      "Arbitrum Mainnet",
			NameCoinr:    "ARB",
			WrappedToken: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1",
		},
		"137": {
			NameStd:      "Polygon Mainnet",
			NameCoinr:    "MATIC",
			WrappedToken: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270",
		},
		"56": {
			NameStd:      "BSC Mainnet",
			NameCoinr:    "BNB",
			WrappedToken: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c",
		},
		"10": {
			NameStd:      "Optimism Mainnet",
			NameCoinr:    "ETH",
			WrappedToken: "0x4200000000000000000000000000000000000006",
		},
		"250": {
			NameStd:      "Fantom Mainnet",
			NameCoinr:    "FTM",
			WrappedToken: "0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83",
		},
		"43114": {
			NameStd:      "Avalanche Mainnet",
			NameCoinr:    "AVAX",
			WrappedToken: "0xB31f66AA3C1e785363F0875A1B74E27b85FD66c7",
		},
	}

	if info, exists := networkMap[chainID]; exists {
		return info
	}

	// Default fallback for unknown networks
	return NetworkInfo{
		NameStd:      fmt.Sprintf("Network %s", chainID),
		NameCoinr:    "UNKNOWN",
		WrappedToken: "",
	}
}

// extractRPCURLs extracts RPC URLs from the mixed array format
func extractRPCURLs(rpcs []interface{}) []string {
	var urls []string
	for _, rpc := range rpcs {
		switch v := rpc.(type) {
		case string:
			// Simple string URL
			urls = append(urls, v)
		case map[string]interface{}:
			// Object with URL field
			if url, ok := v["url"].(string); ok {
				urls = append(urls, url)
			}
		}
	}
	return urls
}

// GetRPCCheckInterval returns the RPC check interval as a duration
func (c *ExtendedConfig) GetRPCCheckInterval() time.Duration {
	return time.Duration(c.Monitoring.RPCCheckInterval) * time.Second
}

// GetPriceFetchInterval returns the price fetch interval as a duration
func (c *ExtendedConfig) GetPriceFetchInterval() time.Duration {
	return time.Duration(c.Monitoring.PriceFetchInterval) * time.Second
}

// GetRPCTimeout returns the RPC timeout as a duration
func (c *ExtendedConfig) GetRPCTimeout() time.Duration {
	return time.Duration(c.Monitoring.RPCTimeout) * time.Second
}

// GetCacheExpiration returns the cache expiration as a duration
func (c *ExtendedConfig) GetCacheExpiration() time.Duration {
	return time.Duration(c.Cache.Expiration) * time.Second
}

// GetPriceFeedsForNetwork returns price feeds for a specific network
func (c *ExtendedConfig) GetPriceFeedsForNetwork(networkID uint64) []PriceFeedInfo {
	var feeds []PriceFeedInfo

	for networkName, networkConfig := range c.PriceFeeds {
		if networkConfig.ChainID == int(networkID) {
			for _, feed := range networkConfig.Feeds {
				feeds = append(feeds, PriceFeedInfo{
					Name:     feed.Name,
					Address:  feed.Address,
					Decimals: feed.Decimals,
					Network:  networkName,
				})
			}
			break
		}
	}

	return feeds
}

// PriceFeedInfo represents information about a price feed
type PriceFeedInfo struct {
	Name     string
	Address  string
	Decimals int
	Network  string
	Symbol   string
}

// GetNetworkRPCs returns RPC endpoints for a specific network
func (c *ExtendedConfig) GetNetworkRPCs(networkID uint64) []string {
	switch networkID {
	case 1: // Ethereum mainnet
		return c.EthereumRPCs.URLs
	case 42161: // Arbitrum mainnet
		return c.ArbitrumRPCs.URLs
	default:
		// Try to find in price feeds configuration
		for _, networkConfig := range c.PriceFeeds {
			if networkConfig.ChainID == int(networkID) {
				// Return default RPCs based on network type
				if networkID == 1 {
					return c.EthereumRPCs.URLs
				} else if networkID == 42161 {
					return c.ArbitrumRPCs.URLs
				}
			}
		}
		return []string{}
	}
}

// CreateNetworkConfig creates a NetworkConfiguration from the YAML config and extraRpcs.json
func (c *ExtendedConfig) CreateNetworkConfig() *NetworkConfiguration {
	var networks []RPCConfig

	// Try to load extraRpcs.json file
	extraRPCs, err := LoadExtraRPCs("conf/extraRpcs.json")
	if err != nil {
		// Fallback to original behavior if extraRpcs.json is not available
		return c.createNetworkConfigFromYAML()
	}

	// Create networks from extraRpcs.json
	for chainID, rpcConfig := range *extraRPCs {
		if len(rpcConfig.RPCs) == 0 {
			continue
		}

		// Extract RPC URLs
		endpoints := extractRPCURLs(rpcConfig.RPCs)
		if len(endpoints) == 0 {
			continue
		}

		// Get network info
		networkInfo := getNetworkInfo(chainID)

		// Convert chainID to uint64 for price feed lookup
		chainIDUint, err := strconv.ParseUint(chainID, 10, 64)
		if err != nil {
			continue
		}

		// Get price feeds for this network
		feeds := make(map[string]string)
		for _, feed := range c.GetPriceFeedsForNetwork(chainIDUint) {
			feeds[feed.Name] = feed.Address
		}

		// Create RPC config
		networks = append(networks, RPCConfig{
			NetworkID:    chainID,
			NameStd:      networkInfo.NameStd,
			NameCoinr:    networkInfo.NameCoinr,
			WrappedToken: networkInfo.WrappedToken,
			Endpoints:    endpoints,
			ApprovalSrc:  feeds,
		})
	}

	// If no networks were loaded from extraRpcs.json, fallback to YAML config
	if len(networks) == 0 {
		return c.createNetworkConfigFromYAML()
	}

	return &NetworkConfiguration{
		Networks:  networks,
		ClientUse: make(map[uint64]*EthereumClient),
	}
}

// createNetworkConfigFromYAML creates a NetworkConfiguration from the YAML config (fallback method)
func (c *ExtendedConfig) createNetworkConfigFromYAML() *NetworkConfiguration {
	var networks []RPCConfig

	// Add Ethereum mainnet
	if len(c.EthereumRPCs.URLs) > 0 {
		ethereumFeeds := make(map[string]string)
		for _, feed := range c.GetPriceFeedsForNetwork(1) {
			ethereumFeeds[feed.Name] = feed.Address
		}

		networks = append(networks, RPCConfig{
			NetworkID:    "1",
			NameStd:      "Ethereum Mainnet",
			NameCoinr:    "ETH",
			WrappedToken: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
			Endpoints:    c.EthereumRPCs.URLs,
			ApprovalSrc:  ethereumFeeds,
		})
	}

	// Add Arbitrum mainnet
	if len(c.ArbitrumRPCs.URLs) > 0 {
		arbitrumFeeds := make(map[string]string)
		for _, feed := range c.GetPriceFeedsForNetwork(42161) {
			arbitrumFeeds[feed.Name] = feed.Address
		}

		networks = append(networks, RPCConfig{
			NetworkID:    "42161",
			NameStd:      "Arbitrum Mainnet",
			NameCoinr:    "ARB",
			WrappedToken: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1",
			Endpoints:    c.ArbitrumRPCs.URLs,
			ApprovalSrc:  arbitrumFeeds,
		})
	}

	return &NetworkConfiguration{
		Networks:  networks,
		ClientUse: make(map[uint64]*EthereumClient),
	}
}
