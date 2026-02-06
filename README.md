# Price Feeding Library

A comprehensive Go library for monitoring both Chainlink and Pyth Network price feeds across multiple blockchain networks. This library provides robust building blocks for building price monitoring applications with automatic RPC endpoint switching, local price caching, and real-time price updates.

## 🚀 Quick Start

### Installation

```bash
go get github.com/morpheum-labs/pricefeeding
```

### Basic Usage

```bash
# Run Chainlink price feed monitor
go run . --chainlink

# Run Pyth price feed monitor  
go run . --pyth

# Build and run
make run-chainlink
make run-pyth
```

## 🏗️ Building Blocks

This library provides several key building blocks that you can use to build your own price monitoring applications:

### 1. **RPC Scanner & Switcher** (`rpcscan/`)
- **Automatic RPC Endpoint Discovery**: Scans and tests multiple RPC endpoints
- **Latency-Based Selection**: Automatically selects the fastest available endpoint
- **Failover Support**: Seamlessly switches to backup endpoints when needed
- **Multi-Network Support**: Manages RPC clients for multiple blockchain networks

### 2. **Chainlink Price Monitor** (`pricefeed/`)
- **Real-time Price Monitoring**: Monitors Chainlink price feeds with configurable intervals
- **Thread-Safe Operations**: All operations are thread-safe for concurrent access
- **Local Price Caching**: Stores price data locally with timestamps
- **Network-Agnostic**: Works with any EVM-compatible network

### 3. **Pyth Price Monitor** (`pricefeed/`)
- **Pyth Network Integration**: Direct integration with Pyth Network Hermes API
- **Multiple Asset Classes**: Supports crypto, equities, FX, metals, and more
- **Real-time Streaming**: Optional real-time price updates via Server-Sent Events
- **Configurable Precision**: Handles different decimal precisions per asset

### 4. **Price Cache Manager** (`pricefeed/`)
- **Thread-Safe Storage**: Concurrent access to price data
- **Persistence Tracking**: Tracks when prices were last updated
- **Network Isolation**: Separate caches per network
- **Memory Efficient**: Optimized for high-frequency updates

### 5. **Configuration System**
- **YAML Configuration**: Human-readable configuration files
- **Multi-Asset Support**: Configure crypto, stocks, and other assets
- **Network-Specific Settings**: Per-network RPC and feed configurations
- **Validation**: Built-in configuration validation with sensible defaults

## Architecture

### RPC Scanner (`rpcscan/`)
- **Concurrent Latency Testing**: Tests all RPC endpoints in parallel with timeout protection
- **Automatic Failover**: Switches to the best available endpoint based on response time
- **Thread-Safe Client Management**: Uses proper mutexes to prevent race conditions
- **Continuous Monitoring**: Regularly checks and updates RPC endpoint performance

### Price Feed Monitor (`pricefeed/`)
- **Chainlink Integration**: Direct integration with Chainlink AggregatorV3Interface
- **Concurrent Price Fetching**: Fetches prices from multiple feeds simultaneously
- **Local Caching**: Thread-safe price data storage with timestamps
- **Configurable Intervals**: Customizable monitoring frequency (default: 30 seconds)

## Key Improvements Made

### 1. Race Condition Fixes
- **Proper Mutex Usage**: Added read/write locks for all shared data structures
- **Channel Management**: Fixed channel closing and goroutine coordination
- **Context Timeouts**: Added proper timeout handling to prevent hanging operations

### 2. Performance Optimizations
- **Concurrent Processing**: Parallel RPC endpoint testing and price fetching
- **Efficient Channel Usage**: Proper buffering and closing of channels
- **Memory Management**: Reduced memory allocations and improved garbage collection
- **Timeout Protection**: Prevents hanging operations with configurable timeouts

### 3. Code Quality Improvements
- **Removed Dependencies**: Eliminated references to non-existent packages
- **Clean Architecture**: Separated concerns into logical packages
- **Error Handling**: Comprehensive error handling and logging
- **Type Safety**: Proper type definitions and validation

### 4. Configuration System Upgrade
- **YAML Configuration**: Upgraded from TOML to YAML for better readability
- **Extended Configuration**: Added RPC-specific and monitoring settings
- **Backward Compatibility**: Maintains support for legacy JSON configuration
- **Validation**: Comprehensive configuration validation with sensible defaults

## Configuration

The application now uses **YAML configuration** instead of TOML for better readability and maintainability.

### Main Configuration (`vault_config.yaml`)
```yaml
# Server configuration
port: 8080
secret_hash: "YOUR_SECRET_HASH_HERE"

# Database configuration
database:
  postgres:
    db_conn: "postgresql://username:password@localhost:5432/chainlinkPrice_feed?sslmode=disable"
    db_conn_pool: 10

# RPC endpoint configurations
arbitrum_rpcs:
  urls:
    - "https://arb1.arbitrum.io/rpc"
    - "https://arbitrum-mainnet.infura.io/v3/YOUR_INFURA_PROJECT_ID"

ethereum_rpcs:
  urls:
    - "https://mainnet.infura.io/v3/YOUR_INFURA_PROJECT_ID"
    - "https://eth-mainnet.g.alchemy.com/v2/YOUR_ALCHEMY_API_KEY"
    - "https://rpc.ankr.com/eth"

# Price feed monitoring configuration
priceFeeds:
  ethereum:
    chainId: 1
    feeds:
      - name: "ETH/USD"
        address: "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419"
        decimals: 8
      - name: "BTC/USD"
        address: "0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c"
        decimals: 8

# Monitoring configuration
monitoring:
  rpc_check_interval: 30      # seconds
  price_fetch_interval: 30    # seconds
  rpcTimeout: 10             # seconds
  max_concurrent_calls: 10

# Cache configuration
cache:
  enabled: true
  expiration: 300             # seconds
  maxSize: 1000
```

### Legacy Network Configuration (`networks.json`)
The application still supports the legacy JSON configuration for backward compatibility:
```json
{
  "networks": [
    {
      "networkId": "1",
      "name_1": "Ethereum Mainnet",
      "name_2": "ETH",
      "gas_token": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
      "endpoints": [
        "https://mainnet.infura.io/v3/YOUR_KEY",
        "https://eth-mainnet.g.alchemy.com/v2/YOUR_KEY"
      ],
      "check": {
        "chainlink_eth_usd": "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419"
      }
    }
  ]
}
```

## 📖 Usage Examples

### Building Your Own Chainlink Price Monitor

```go
package main

import (
    "log"
    "time"
    "github.com/morpheum-labs/pricefeeding/pricefeed"
    "github.com/morpheum-labs/pricefeeding/rpcscan"
)

func main() {
    // 1. Create RPC configuration
    config := &rpcscan.Config{
        RootDir: ".",
    }

    // 2. Create price feed manager for a specific network
    priceFeedManager := rpcscan.NewPriceFeedManager(42161) // Arbitrum

    // 3. Load price feed configurations
    if err := priceFeedManager.LoadConfig("conf"); err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // 4. Create network configuration
    networkConfig := priceFeedManager.CreateNetworkConfig()

    // 5. Start RPC monitoring
    stopChan := make(chan struct{})
    go rpcscan.MonitorAllRPCEndpoints(config, networkConfig, 30*time.Second, stopChan)

    // 6. Wait for RPC clients to be established
    time.Sleep(35 * time.Second)

    // 7. Create price monitor
    priceMonitor := pricefeed.NewPriceMonitorWithImmediateMode(30*time.Second, true)
    priceMonitor.SetNetworkConfig(networkConfig)

    // 8. Add clients and feeds
    clients := networkConfig.GetAllClients()
    for networkID, client := range clients {
        priceMonitor.AddClient(networkID, client.GetClient())
        
        // Add feeds for this network
        feeds := priceFeedManager.GetFeedsForNetwork(networkID)
        for _, feed := range feeds {
            if feed.Address != "" {
                priceMonitor.AddPriceFeedWithSymbol(networkID, feed.Address, feed.Symbol)
            }
        }
    }

    // 9. Start monitoring
    go priceMonitor.Start()

    // 10. Get prices
    prices := priceMonitor.GetAllPrices(42161)
    for address, priceData := range prices {
        log.Printf("Price for %s: %s", address, priceData.Answer.String())
    }
}
```

### Building Your Own Pyth Price Monitor

```go
package main

import (
    "log"
    "time"
    "github.com/morpheum-labs/pricefeeding/pricefeed"
)

func main() {
    // 1. Create Pyth price monitor
    endpoint := "https://hermes.pyth.network"
    interval := 10 * time.Second
    immediateMode := true
    
    monitor := pricefeed.NewPythPriceMonitor(endpoint, interval, immediateMode)

    // 2. Add price feeds
    monitor.AddPriceFeed("e62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43", "BTC/USD")
    monitor.AddPriceFeed("ff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace", "ETH/USD")
    monitor.AddPriceFeed("47a156470288850a440df3a6ce85a55917b813a19bb5b31128a33a986566a362", "TSLAX/USD")

    // 3. Start monitoring
    go monitor.Start()

    // 4. Get prices
    allPrices := monitor.GetAllPrices()
    for _, priceData := range allPrices {
        log.Printf("%s: %s (Updated: %s)", 
            priceData.Symbol, 
            priceData.Price.String(), 
            priceData.Timestamp.Format("15:04:05"))
    }
}
```

### Using the RPC Scanner Independently

```go
package main

import (
    "log"
    "time"
    "github.com/morpheum-labs/pricefeeding/rpcscan"
)

func main() {
    // 1. Create configuration
    config := &rpcscan.Config{
        RootDir: ".",
    }

    // 2. Start RPC monitoring
    stopChan, networkConfig := rpcscan.RuntimeWeb3Selection(config)

    // 3. Wait for clients to be established
    time.Sleep(10 * time.Second)

    // 4. Get the best client for a network
    client := networkConfig.GetBestClient(42161) // Arbitrum
    if client != nil {
        log.Printf("Best client for Arbitrum: %v", client.GetLastUpdated())
    }

    // 5. Get all available networks
    networkIDs := networkConfig.GetAllNetworkIDs()
    log.Printf("Available networks: %v", networkIDs)

    // Cleanup
    close(stopChan)
}
```

### Using the Price Cache Manager

```go
package main

import (
    "log"
    "time"
    "github.com/morpheum-labs/pricefeeding/pricefeed"
)

func main() {
    // 1. Create price cache manager
    cacheManager := pricefeed.NewPriceCacheManager()

    // 2. Add feeds to cache
    cacheManager.AddFeed(42161, "0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612") // ETH/USD on Arbitrum
    cacheManager.AddFeed(42161, "0x6ce185860a4963106506C203335A2910413708e9") // BTC/USD on Arbitrum

    // 3. Update prices (typically done by price monitor)
    priceData := &pricefeed.PriceData{
        Answer:    big.NewInt(200000000000), // $2000 with 8 decimals
        Timestamp: time.Now(),
        NetworkID: 42161,
    }
    cacheManager.UpdatePrice(42161, "0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612", priceData)

    // 4. Retrieve prices
    price, err := cacheManager.GetPrice(42161, "0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612")
    if err != nil {
        log.Printf("Error getting price: %v", err)
    } else {
        log.Printf("ETH price: %s", price.Answer.String())
    }

    // 5. Get all prices for a network
    allPrices := cacheManager.GetAllPrices(42161)
    for address, priceData := range allPrices {
        log.Printf("Price for %s: %s", address, priceData.Answer.String())
    }
}
```

## 🔧 Configuration

### Chainlink Configuration (`conf/crytos.yaml`)

```yaml
# Example configuration for crypto price feeds
btc:
  ticker:      BTC/USD
  proxy:       "0x6ce185860a4963106506C203335A2910413708e9"
  decimals:    8
  minAnswer:  "10000000000000"
  maxAnswer:  "1000000000000000000000000"
  threshold:   1
  heartbeat:   3600
  staleness_threshold: 3600

eth:
  ticker:      ETH/USD
  proxy:       "0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612"
  decimals:    8
  minAnswer:  "100000000"
  maxAnswer:  "10000000000000000000"
  threshold:   1
  heartbeat:   3600
  staleness_threshold: 3600
```

### Pyth Configuration (`conf/pyth_tickers.yaml`)

```yaml
# Example configuration for Pyth price feeds
btc:
  ticker:      BTC/USD
  priceId:    "e62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43"
  decimals:    5
  description: "Bitcoin / US Dollar"
  category:    "crypto"

eth:
  ticker:      ETH/USD
  priceId:    "ff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace"
  decimals:    5
  description: "Ethereum / US Dollar"
  category:    "crypto"

tslax:
  ticker:      TSLAX/USD
  priceId:    "47a156470288850a440df3a6ce85a55917b813a19bb5b31128a33a986566a362"
  decimals:    5
  description: "Tesla / US Dollar"
  category:    "equity"
```

### Stock Configuration (`conf/stocks.yaml`)

```yaml
# Example configuration for stock price feeds
nvda:
  ticker:      NVDA/USD
  proxy:       "0x4881A4418b5F2460B21d6F08CD5aA0678a7f262F"
  decimals:    2
  minAnswer:  "10"
  maxAnswer:  "10000"
  threshold:   0.5
  heartbeat:   3600
  staleness_threshold: 3600
```

## 📚 API Reference

### RPC Scanner (`rpcscan/`)

#### Core Functions
- `RuntimeWeb3Selection(config *Config)`: Starts RPC monitoring and returns stop channel and network config
- `MonitorAllRPCEndpoints(config, networkConfig, interval, stopChan)`: Monitors all RPC endpoints
- `NewPriceFeedManager(networkID uint64)`: Creates a new price feed manager for a specific network

#### Network Configuration
- `GetBestClient(networkID uint64)`: Returns the best available client for a network
- `GetAllNetworkIDs()`: Returns all available network IDs
- `GetAllClients()`: Returns all available clients
- `CreateNetworkConfig()`: Creates network configuration from price feed configs

#### Price Feed Management
- `LoadConfig(configPath string)`: Loads price feed configurations from YAML files
- `GetAllFeeds()`: Returns all loaded price feeds
- `GetFeedsForNetwork(networkID uint64)`: Returns feeds for a specific network

### Chainlink Price Monitor (`pricefeed/`)

#### Core Functions
- `NewPriceMonitor(interval time.Duration)`: Creates a new price monitor
- `NewPriceMonitorWithImmediateMode(interval, immediateMode)`: Creates monitor with immediate mode
- `SetNetworkConfig(networkConfig)`: Sets network configuration for RPC switching

#### Client Management
- `AddClient(networkID uint64, client *ethclient.Client)`: Adds an Ethereum client
- `UpdateClient(networkID uint64, client *ethclient.Client)`: Updates an existing client

#### Feed Management
- `AddPriceFeed(networkID uint64, feedAddress string)`: Adds a price feed to monitor
- `AddPriceFeedWithSymbol(networkID, feedAddress, ticker)`: Adds a price feed with ticker

#### Price Retrieval
- `GetPrice(networkID uint64, feedAddress string)`: Gets latest price for a feed
- `GetAllPrices(networkID uint64)`: Gets all prices for a network

#### Control
- `Start()`: Starts the price monitoring
- `Stop()`: Stops the price monitoring
- `PrintStatus()`: Prints current monitor status

### Pyth Price Monitor (`pricefeed/`)

#### Core Functions
- `NewPythPriceMonitor(endpoint, interval, immediateMode)`: Creates a new Pyth price monitor
- `SetImmediateMode(enabled bool)`: Toggles immediate price printing

#### Feed Management
- `AddPriceFeed(priceID, ticker string)`: Adds a Pyth price feed to monitor

#### Price Retrieval
- `GetPrice(priceID string)`: Gets latest price for a specific feed
- `GetAllPrices()`: Gets all current prices

#### Cache Management
- `GetCacheManager()`: Returns the cache manager for advanced operations
- `PrintLastSavedStatus()`: Prints cache status and last saved timestamp

#### Control
- `Start()`: Starts the Pyth price monitoring
- `Stop()`: Stops the Pyth price monitoring

### Price Cache Manager (`pricefeed/`)

#### Core Functions
- `NewPriceCacheManager()`: Creates a new price cache manager
- `AddFeed(networkID uint64, feedAddress string)`: Adds a feed to the cache

#### Price Management
- `UpdatePrice(networkID, feedAddress, priceData)`: Updates price data for a feed
- `GetPrice(networkID, feedAddress)`: Retrieves price data for a specific feed
- `GetAllPrices(networkID)`: Gets all prices for a network

#### Status
- `GetLastSaved()`: Returns the last saved timestamp

## 🛠️ Build & Development

### Prerequisites

- Go 1.25.1 or later
- Git

### Building the Application

```bash
# Clone the repository
git clone https://github.com/morpheum-labs/pricefeeding.git
cd pricefeeding

# Download dependencies
go mod download

# Build the application
make build

# Build for multiple platforms
make build-all

# Run tests
make test-safe

# Run with coverage
make test-coverage
```

### Development Commands

```bash
# Run in development mode (Chainlink)
make run-dev-chainlink

# Run in development mode (Pyth)
make run-dev-pyth

# Format code
make fmt

# Lint code
make lint

# Clean build artifacts
make clean
```

### Docker Support

```bash
# Build Docker image
make docker-build

# Run Chainlink mode in Docker
make docker-run-chainlink

# Run Pyth mode in Docker
make docker-run-pyth
```

## 🏃‍♂️ Quick Start Examples

### Example 1: Simple Chainlink Monitor

```go
package main

import (
    "log"
    "time"
    "github.com/morpheum-labs/pricefeeding/pricefeed"
    "github.com/morpheum-labs/pricefeeding/rpcscan"
)

func main() {
    // Quick setup for Arbitrum ETH/USD price
    config := &rpcscan.Config{RootDir: "."}
    priceFeedManager := rpcscan.NewPriceFeedManager(42161)
    priceFeedManager.LoadConfig("conf")
    
    networkConfig := priceFeedManager.CreateNetworkConfig()
    stopChan := make(chan struct{})
    go rpcscan.MonitorAllRPCEndpoints(config, networkConfig, 30*time.Second, stopChan)
    
    time.Sleep(35 * time.Second) // Wait for RPC clients
    
    priceMonitor := pricefeed.NewPriceMonitorWithImmediateMode(30*time.Second, true)
    priceMonitor.SetNetworkConfig(networkConfig)
    
    clients := networkConfig.GetAllClients()
    for networkID, client := range clients {
        priceMonitor.AddClient(networkID, client.GetClient())
        feeds := priceFeedManager.GetFeedsForNetwork(networkID)
        for _, feed := range feeds {
            if feed.Address != "" {
                priceMonitor.AddPriceFeedWithSymbol(networkID, feed.Address, feed.Symbol)
            }
        }
    }
    
    go priceMonitor.Start()
    
    // Your application logic here
    select {}
}
```

### Example 2: Simple Pyth Monitor

```go
package main

import (
    "log"
    "time"
    "github.com/morpheum-labs/pricefeeding/pricefeed"
)

func main() {
    // Quick setup for BTC, ETH, and Tesla prices
    monitor := pricefeed.NewPythPriceMonitor(
        "https://hermes.pyth.network",
        10*time.Second,
        true,
    )
    
    monitor.AddPriceFeed("e62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43", "BTC/USD")
    monitor.AddPriceFeed("ff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace", "ETH/USD")
    monitor.AddPriceFeed("47a156470288850a440df3a6ce85a55917b813a19bb5b31128a33a986566a362", "TSLAX/USD")
    
    go monitor.Start()
    
    // Your application logic here
    select {}
}
```

## 📊 Performance Characteristics

- **RPC Endpoint Testing**: ~2-5 seconds for all endpoints (parallel)
- **Price Fetching**: ~1-3 seconds per network (concurrent)
- **Memory Usage**: ~10-50MB depending on number of feeds
- **CPU Usage**: Low, mostly I/O bound operations
- **Concurrent Requests**: Configurable limits to prevent rate limiting

## 🛡️ Error Handling

The library includes comprehensive error handling:

- **Network Connectivity**: Automatic retry with exponential backoff
- **RPC Endpoint Failures**: Automatic failover to backup endpoints
- **Chainlink Contract Failures**: Graceful handling of contract call errors
- **Configuration Errors**: Validation with helpful error messages
- **Rate Limiting**: Built-in request throttling and backoff
- **Timeout Protection**: Configurable timeouts for all operations

## 📈 Monitoring and Logging

- **Detailed RPC Performance**: Latency tracking and endpoint health monitoring
- **Price Update Notifications**: Real-time price update logging
- **Error Context**: Comprehensive error logging with context
- **Performance Metrics**: Latency, update frequency, and success rates
- **Status Reporting**: Regular status updates and health checks

## 🔧 Advanced Usage

### Custom RPC Endpoints

```go
// Add custom RPC endpoints to your configuration
config := &rpcscan.Config{
    RootDir: ".",
    CustomEndpoints: map[uint64][]string{
        42161: {
            "https://your-custom-arbitrum-rpc.com",
            "https://another-arbitrum-rpc.com",
        },
    },
}
```

### Custom Price Feed Validation

```go
// Add custom validation for price feeds
priceMonitor.SetPriceValidator(func(priceData *pricefeed.PriceData) bool {
    // Custom validation logic
    return priceData.Answer.Cmp(big.NewInt(0)) > 0
})
```

### Event-Driven Architecture

```go
// Set up event handlers for price updates
priceMonitor.SetPriceUpdateHandler(func(networkID uint64, feedAddress string, priceData *pricefeed.PriceData) {
    // Handle price updates
    log.Printf("Price update: %s = %s", feedAddress, priceData.Answer.String())
})
```

## 📦 Dependencies

- `github.com/ethereum/go-ethereum`: Ethereum client library
- `gopkg.in/yaml.v2`: YAML configuration parsing
- `github.com/google/uuid`: UUID generation for tracking
- Standard Go libraries for concurrency, networking, and JSON handling

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite: `make test-coverage`
6. Submit a pull request

## 📄 License

This project is open source and available under the MIT License.

## 🆘 Support

- **Documentation**: Check this README and the individual package documentation
- **Issues**: Report bugs and request features on GitHub Issues
- **Examples**: See the `example/` directory for complete working examples
- **Tests**: Run `make test-safe` to verify your setup
