# Pricefeed System Review

## Overview

The pricefeed system is a Go application that monitors price feeds from two sources:
1. **Chainlink** - On-chain price feeds via Ethereum smart contracts
2. **Pyth Network** - Off-chain price feeds via HTTP API

## Architecture

### Main Entry Point (`main.go`)

The application supports two modes:
- `--chainlink`: Monitors Chainlink price feeds
- `--pyth`: Monitors Pyth Network price feeds

Both modes cannot run simultaneously.

### 1. Chainlink Price Feed Monitor

#### Configuration Flow
1. **Price Feed Configuration** (`rpcscan/pricefeed_config.go`)
   - Loads price feed configs from YAML files:
     - `conf/crytos.yaml` - Cryptocurrency price feeds
     - `conf/stocks.yaml` - Stock price feeds
   - Each feed contains:
     - `symbol`: Trading symbol (e.g., "BTC/USD")
     - `proxy`: Contract address on-chain
     - `decimals`: Price precision
     - `min_answer`, `max_answer`: Price bounds
     - `threshold`, `heartbeat`, `staleness_threshold`: Validation parameters

2. **RPC Configuration** (`rpcscan/rpcswitcher.go`)
   - Loads RPC endpoints from `conf/extraRpcs.json`
   - Creates `NetworkConfiguration` with multiple RPC endpoints per network
   - Default network: Arbitrum (Chain ID: 42161)

#### RPC Endpoint Management

**Continuous Monitoring** (`MonitorAllRPCEndpoints`):
- Runs every 30 seconds (configurable)
- Tests all RPC endpoints concurrently for each network
- Measures latency using `web3_clientVersion` RPC call
- Selects endpoint with lowest latency
- Updates `NetworkConfiguration.ClientUse` map with best client

**Immediate RPC Switching**:
- Triggered when error code `-32097` is detected (execution reverted)
- `SwitchRPCEndpointImmediately()` finds alternative endpoint
- Updates client without waiting for next monitoring cycle
- Retries price fetch with new endpoint

#### Price Monitoring Flow (`pricefeed/chainlink_monitor.go`)

1. **Initialization**:
   ```go
   priceMonitor := pricefeed.NewPriceMonitorWithImmediateMode(30*time.Second, true)
   ```

2. **Client Setup**:
   - Waits 35 seconds for RPC monitoring to establish clients
   - Adds clients for each network from `NetworkConfiguration`
   - Adds price feeds for each network

3. **Price Fetching** (`updateAllPrices`):
   - Runs every 30 seconds (configurable)
   - Concurrent fetching with semaphore (max 10 concurrent requests)
   - For each feed:
     - Creates `AggregatorV3Interface` contract instance
     - Calls `LatestRoundData()` to get price
     - Handles RPC errors with automatic switching
     - Updates `PriceCache`

4. **Price Cache** (`pricefeed/price_cache.go`):
   - Thread-safe storage: `map[networkID]map[feedAddress]*PriceData`
   - Stores:
     - `RoundID`: Chainlink round identifier
     - `Answer`: Price value (8 decimals)
     - `StartedAt`, `UpdatedAt`: Timestamps
     - `AnsweredInRound`: Round when answer was finalized

5. **Price Cache Manager** (`pricefeed/price_cache.go`):
   - Wraps `PriceCache` with `lastSaved` timestamp tracking
   - Updates every 15 seconds from monitor results
   - Provides thread-safe access methods

6. **Immediate Mode**:
   - When enabled, prints price updates immediately:
     ```
     🔄 CHAINLINK PRICE UPDATE [12:34:56]
        Symbol: BTC/USD
        Network ID: 42161
        Feed Address: 0x...
        Price: $50000.00000000
        Round ID: 12345
        ...
     ```

#### Status Monitoring

Multiple goroutines run concurrently:
1. **Price Cache Updater** (15s interval):
   - Syncs prices from monitor to cache manager

2. **Client Refresh** (15s interval):
   - Updates clients from network configuration

3. **Status Display** (60s interval):
   - Prints monitor status
   - Displays all current prices with symbols

4. **RPC Health Monitoring** (60s interval):
   - Logs active RPC clients and last update times

### 2. Pyth Network Price Feed Monitor

#### Configuration Flow
1. **Ticker Configuration**:
   - Loads from `conf/pyth_tickers.yaml`
   - Maps `price_id` (hex string) to `symbol`
   - Fallback to hardcoded defaults if file missing

2. **Client Setup** (`pyth/client.go`):
   - Uses Hermes API endpoint: `https://hermes.pyth.network`
   - HTTP client with 5-second timeout
   - 3 retries with exponential backoff

#### Price Monitoring Flow (`pricefeed/pyth_monitor.go`)

1. **Initialization**:
   ```go
   monitor := pricefeed.NewPythPriceMonitor(endpoint, 10*time.Second, true)
   ```

2. **Price Fetching** (`fetchPriceData`):
   - Runs every 10 seconds (configurable)
   - Calls `GetLatestPriceUpdates()` with all price IDs
   - Requests parsed data format
   - Processes each feed in response

3. **Price Data Structure** (`PythPriceResetData`):
   - `ID`: Price feed identifier
   - `Price`: Price value (big.Int)
   - `Confidence`: Confidence interval (big.Int)
   - `Exponent`: Decimal exponent (can be negative)
   - `PublishTime`: Unix timestamp
   - `Slot`: Solana slot number
   - `EMA`, `EMAConfidence`: Exponential moving average (optional)

4. **Price Conversion**:
   - Converts Pyth data to `PriceData` format for cache compatibility
   - Maps `Slot` → `RoundID`
   - Maps `PublishTime` → `StartedAt`
   - Uses network ID 0 for all Pyth feeds

5. **Price Display**:
   - Calculates actual price: `price * 10^exponent` (handles negative exponents)
   - Prints formatted update:
     ```
     🔄 PYTH PRICE UPDATE [12:34:56]
        Symbol: BTC/USD
        Price ID: 0xe62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43
        Price: 50000.00000000
        Confidence: ±1.00000000
        ...
     ```

6. **Status Display** (30s interval):
   - Shows cache status with `lastSaved` timestamp
   - Displays all current prices

## Key Features

### 1. RPC Reliability
- **Automatic RPC Switching**: Monitors all endpoints and selects best
- **Immediate Failover**: Switches on specific errors (-32097)
- **Concurrent Testing**: Tests all endpoints in parallel
- **Latency-Based Selection**: Chooses fastest endpoint

### 2. Concurrent Processing
- **Semaphore-Limited Fetching**: Max 10 concurrent Chainlink requests
- **Parallel RPC Testing**: All endpoints tested simultaneously
- **Thread-Safe Caching**: All cache operations use mutexes

### 3. Error Handling
- **RPC Error Detection**: Detects specific error codes
- **Automatic Retry**: Retries with new endpoint after switch
- **Graceful Degradation**: Continues monitoring other feeds on failure

### 4. Monitoring & Observability
- **Status Logging**: Regular status updates
- **Price Display**: Formatted price updates
- **Cache Tracking**: Last saved timestamp
- **Health Monitoring**: RPC client status

### 5. Configuration Management
- **YAML-Based Config**: Easy to modify price feeds
- **JSON RPC Config**: Network and endpoint configuration
- **Symbol Mapping**: Human-readable symbols for addresses/IDs

## Data Flow Diagrams

### Chainlink Flow
```
Config Files (YAML)
    ↓
PriceFeedManager
    ↓
NetworkConfiguration (RPC endpoints)
    ↓
MonitorAllRPCEndpoints (selects best RPC)
    ↓
PriceMonitor (fetches prices)
    ↓
PriceCache (stores prices)
    ↓
PriceCacheManager (tracks lastSaved)
    ↓
Status Display
```

### Pyth Flow
```
Config File (YAML)
    ↓
PythPriceMonitor
    ↓
HermesClient (HTTP API)
    ↓
PriceCacheManager (stores prices)
    ↓
Status Display
```

## Thread Safety

All shared data structures use mutexes:
- `PriceCache`: `sync.RWMutex` for read/write operations
- `PriceMonitor`: `sync.RWMutex` for client and symbol maps
- `NetworkConfiguration`: `sync.RWMutex` for client map
- `EthereumClient`: `sync.RWMutex` for client access

## Performance Optimizations

1. **Concurrent Fetching**: Multiple feeds fetched in parallel
2. **Semaphore Limiting**: Prevents overwhelming RPC endpoints
3. **Caching**: Reduces redundant API calls
4. **Parallel RPC Testing**: Fast endpoint selection
5. **Immediate Mode**: Optional real-time updates without polling cache

## Configuration Examples

### Chainlink Feed (crytos.yaml)
```yaml
BTC/USD:
  symbol: BTC/USD
  proxy: "0x6ce185860a4963106506C203335A2910413708e9"
  decimals: 8
  min_answer: "1000"
  max_answer: "100000000"
  threshold: 0.5
  heartbeat: 3600
  staleness_threshold: 86400
```

### Pyth Feed (pyth_tickers.yaml)
```yaml
BTC/USD:
  symbol: BTC/USD
  price_id: "e62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43"
  decimals: 8
  description: "Bitcoin / US Dollar"
  category: "crypto"
```

## Graceful Shutdown

Both monitors support graceful shutdown:
1. Signal handling (SIGINT, SIGTERM)
2. Stop channels for goroutines
3. Context cancellation
4. Clean resource cleanup

## Testing

Test files available:
- `pricefeed/chainlink_monitor_test.go`
- `pricefeed/pyth_monitor_test.go`
- `rpcscan/chain_registry_test.go`

## Dependencies

- `github.com/ethereum/go-ethereum`: Ethereum client library
- `gopkg.in/yaml.v2/v3`: YAML parsing
- Standard Go libraries: `sync`, `time`, `context`, `math/big`

## Usage

### Chainlink Mode
```bash
go run . --chainlink
```

### Pyth Mode
```bash
go run . --pyth
```

## Summary

The pricefeed system is a robust, production-ready solution for monitoring both on-chain (Chainlink) and off-chain (Pyth) price feeds. It features:

- **Reliability**: Automatic RPC switching and error recovery
- **Performance**: Concurrent processing and caching
- **Observability**: Comprehensive logging and status monitoring
- **Flexibility**: Configurable intervals, immediate mode, multiple networks
- **Thread Safety**: All operations are safe for concurrent access

The system is well-architected with clear separation of concerns between configuration, RPC management, price monitoring, and caching layers.
