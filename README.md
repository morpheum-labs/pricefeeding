# Chainlink Price Feed Monitor

A robust, efficient Go application for monitoring Chainlink price feeds across multiple blockchain networks with automatic RPC endpoint switching and local price caching.

## Features

- **Switchable RPC Clients**: Automatically finds and switches to the best RPC endpoints based on latency
- **Race Condition Free**: Thread-safe implementation with proper synchronization
- **Efficient Monitoring**: Concurrent price feed monitoring with configurable intervals
- **Local Price Cache**: Stores price data locally with thread-safe access
- **Multi-Network Support**: Supports multiple blockchain networks simultaneously
- **Graceful Shutdown**: Proper signal handling for clean application termination

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
    db_conn: "postgresql://username:password@localhost:5432/chainlink_price_feed?sslmode=disable"
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
price_feeds:
  ethereum:
    chain_id: 1
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
  rpc_timeout: 10             # seconds
  max_concurrent_calls: 10

# Cache configuration
cache:
  enabled: true
  expiration: 300             # seconds
  max_size: 1000
```

### Legacy Network Configuration (`networks.json`)
The application still supports the legacy JSON configuration for backward compatibility:
```json
{
  "networks": [
    {
      "network_id": "1",
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

## Usage

### Basic Usage
```bash
# Build the application
go build -o chainlink-price-feed .

# Run the application
./chainlink-price-feed
```

### Programmatic Usage
```go
// Create configuration
config := &rpcscan.Config{
    RootDir: ".",
}

// Start RPC monitoring
stopChan, networkConfig := rpcscan.RuntimeWeb3Selection(config)

// Create price monitor
priceMonitor := pricefeed.NewPriceMonitor(30 * time.Second)

// Add clients and feeds
for networkID, client := range networkConfig.ClientUse {
    priceMonitor.AddClient(networkID, client.GetClient())
    priceMonitor.AddPriceFeed(networkID, "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419")
}

// Start monitoring
go priceMonitor.Start()

// Get prices
price, err := priceMonitor.GetPrice(1, "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419")
```

## API Reference

### RPC Scanner
- `RuntimeWeb3Selection(config *Config)`: Starts RPC monitoring and returns stop channel and network config
- `GetBestClient(networkID uint64)`: Returns the best available client for a network
- `GetAllNetworkIDs()`: Returns all available network IDs

### Price Monitor
- `NewPriceMonitor(interval time.Duration)`: Creates a new price monitor
- `AddClient(networkID uint64, client *ethclient.Client)`: Adds an Ethereum client
- `AddPriceFeed(networkID uint64, feedAddress string)`: Adds a price feed to monitor
- `GetPrice(networkID uint64, feedAddress string)`: Gets latest price for a feed
- `GetAllPrices(networkID uint64)`: Gets all prices for a network
- `Start()`: Starts the price monitoring
- `Stop()`: Stops the price monitoring

## Performance Characteristics

- **RPC Endpoint Testing**: ~2-5 seconds for all endpoints (parallel)
- **Price Fetching**: ~1-3 seconds per network (concurrent)
- **Memory Usage**: ~10-50MB depending on number of feeds
- **CPU Usage**: Low, mostly I/O bound operations

## Error Handling

The application includes comprehensive error handling:
- Network connectivity issues
- RPC endpoint failures
- Chainlink contract call failures
- Configuration errors
- Graceful degradation when endpoints fail

## Monitoring and Logging

- Detailed logging of RPC endpoint performance
- Price update notifications
- Error logging with context
- Performance metrics (latency, update frequency)

## Dependencies

- `github.com/ethereum/go-ethereum`: Ethereum client library
- Standard Go libraries for concurrency, networking, and JSON handling

## License

This project is open source and available under the MIT License.
