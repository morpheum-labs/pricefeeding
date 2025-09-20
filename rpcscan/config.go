package rpcscan

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aelmanaa/chainlink-price-feed-golang/shared"
	"gopkg.in/yaml.v3"
)

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

// CreateNetworkConfig creates a NetworkConfiguration from the YAML config
func (c *ExtendedConfig) CreateNetworkConfig() *NetworkConfiguration {
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
