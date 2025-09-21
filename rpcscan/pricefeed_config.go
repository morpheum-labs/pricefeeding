package rpcscan

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// PriceFeedConfig represents the structure of individual price feed entries
type PriceFeedConfig struct {
	Symbol             string  `yaml:"symbol"`
	Proxy              string  `yaml:"proxy"`
	Decimals           int     `yaml:"decimals"`
	MinAnswer          string  `yaml:"min_answer"`
	MaxAnswer          string  `yaml:"max_answer"`
	Threshold          float64 `yaml:"threshold"`
	Heartbeat          int     `yaml:"heartbeat"`
	StalenessThreshold int     `yaml:"staleness_threshold"`
}

// PriceFeedFileConfig represents the structure of the YAML files
type PriceFeedFileConfig struct {
	Feeds map[string]PriceFeedConfig `yaml:",inline"`
}

// PriceFeedManager manages price feed configurations from multiple YAML files
type PriceFeedManager struct {
	CryptoFeeds map[string]PriceFeedConfig
	StockFeeds  map[string]PriceFeedConfig
	NetworkID   uint64 // Default network ID (Arbitrum: 42161)
}

// NewPriceFeedManager creates a new price feed manager
func NewPriceFeedManager(networkID uint64) *PriceFeedManager {
	return &PriceFeedManager{
		CryptoFeeds: make(map[string]PriceFeedConfig),
		StockFeeds:  make(map[string]PriceFeedConfig),
		NetworkID:   networkID,
	}
}

// LoadConfig loads price feed configurations from YAML files
func (pfm *PriceFeedManager) LoadConfig(configDir string) error {
	// Load crypto feeds
	cryptoPath := filepath.Join(configDir, "crytos.yaml")
	if err := pfm.loadConfigFile(cryptoPath, &pfm.CryptoFeeds); err != nil {
		return fmt.Errorf("failed to load crypto feeds: %w", err)
	}

	// Load stock feeds
	stockPath := filepath.Join(configDir, "stocks.yaml")
	if err := pfm.loadConfigFile(stockPath, &pfm.StockFeeds); err != nil {
		return fmt.Errorf("failed to load stock feeds: %w", err)
	}

	return nil
}

// loadConfigFile loads a single YAML configuration file
func (pfm *PriceFeedManager) loadConfigFile(filePath string, target *map[string]PriceFeedConfig) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", filePath)
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var config PriceFeedFileConfig
	if err := yaml.Unmarshal(fileContent, &config); err != nil {
		return fmt.Errorf("failed to parse YAML file %s: %w", filePath, err)
	}

	*target = config.Feeds
	return nil
}

// GetAllFeeds returns all price feeds (crypto + stocks) as PriceFeedInfo slice
func (pfm *PriceFeedManager) GetAllFeeds() []PriceFeedInfo {
	var feeds []PriceFeedInfo

	// Add crypto feeds
	for name, config := range pfm.CryptoFeeds {
		feeds = append(feeds, PriceFeedInfo{
			Name:     name,
			Address:  config.Proxy,
			Decimals: config.Decimals,
			Network:  "crypto",
			Symbol:   config.Symbol,
		})
	}

	// Add stock feeds
	for name, config := range pfm.StockFeeds {
		feeds = append(feeds, PriceFeedInfo{
			Name:     name,
			Address:  config.Proxy,
			Decimals: config.Decimals,
			Network:  "stocks",
			Symbol:   config.Symbol,
		})
	}

	return feeds
}

// GetCryptoFeeds returns only crypto price feeds
func (pfm *PriceFeedManager) GetCryptoFeeds() []PriceFeedInfo {
	var feeds []PriceFeedInfo
	for name, config := range pfm.CryptoFeeds {
		feeds = append(feeds, PriceFeedInfo{
			Name:     name,
			Address:  config.Proxy,
			Decimals: config.Decimals,
			Network:  "crypto",
			Symbol:   config.Symbol,
		})
	}
	return feeds
}

// GetStockFeeds returns only stock price feeds
func (pfm *PriceFeedManager) GetStockFeeds() []PriceFeedInfo {
	var feeds []PriceFeedInfo
	for name, config := range pfm.StockFeeds {
		feeds = append(feeds, PriceFeedInfo{
			Name:     name,
			Address:  config.Proxy,
			Decimals: config.Decimals,
			Network:  "stocks",
			Symbol:   config.Symbol,
		})
	}
	return feeds
}

// GetFeedsForNetwork returns feeds for a specific network ID
func (pfm *PriceFeedManager) GetFeedsForNetwork(networkID uint64) []PriceFeedInfo {
	if networkID != pfm.NetworkID {
		return []PriceFeedInfo{}
	}
	return pfm.GetAllFeeds()
}

// CreateNetworkConfig creates a NetworkConfiguration from the price feed configs and extraRpcs.json
func (pfm *PriceFeedManager) CreateNetworkConfig() *NetworkConfiguration {
	// Try to load extraRpcs.json file first
	extraRPCs, err := LoadExtraRPCs("conf/extraRpcs.json")
	if err != nil {
		// Fallback to original behavior if extraRpcs.json is not available
		return pfm.createNetworkConfigFromFeeds()
	}

	var networks []RPCConfig

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

		// Get price feeds for this network (only if it matches our configured network)
		feeds := make(map[string]string)
		if chainIDUint == pfm.NetworkID {
			// Add crypto feeds to approval source
			for name, config := range pfm.CryptoFeeds {
				feeds[name] = config.Proxy
			}

			// Add stock feeds to approval source
			for name, config := range pfm.StockFeeds {
				feeds[name] = config.Proxy
			}
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

	// If no networks were loaded from extraRpcs.json, fallback to original behavior
	if len(networks) == 0 {
		return pfm.createNetworkConfigFromFeeds()
	}

	return &NetworkConfiguration{
		Networks:  networks,
		ClientUse: make(map[uint64]*EthereumClient),
	}
}

// createNetworkConfigFromFeeds creates a NetworkConfiguration from the price feed configs (fallback method)
func (pfm *PriceFeedManager) createNetworkConfigFromFeeds() *NetworkConfiguration {
	// Create approval source map with all feeds
	approvalSrc := make(map[string]string)

	// Add crypto feeds to approval source
	for name, config := range pfm.CryptoFeeds {
		approvalSrc[name] = config.Proxy
	}

	// Add stock feeds to approval source
	for name, config := range pfm.StockFeeds {
		approvalSrc[name] = config.Proxy
	}

	// Create RPC config for Arbitrum (default network)
	rpcConfig := RPCConfig{
		NetworkID:    strconv.FormatUint(pfm.NetworkID, 10),
		NameStd:      "Arbitrum Mainnet",
		NameCoinr:    "ARB",
		WrappedToken: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1",
		Endpoints: []string{
			"https://arb1.arbitrum.io/rpc",
			"https://arbitrum.publicnode.com",
			"https://arbitrum-one.public.blastapi.io",
		},
		ApprovalSrc: approvalSrc,
	}

	return &NetworkConfiguration{
		Networks:  []RPCConfig{rpcConfig},
		ClientUse: make(map[uint64]*EthereumClient),
	}
}

// GetDefaultRPCCheckInterval returns the default RPC check interval
func (pfm *PriceFeedManager) GetDefaultRPCCheckInterval() time.Duration {
	return 30 * time.Second
}

// GetDefaultPriceFetchInterval returns the default price fetch interval
func (pfm *PriceFeedManager) GetDefaultPriceFetchInterval() time.Duration {
	return 30 * time.Second
}
