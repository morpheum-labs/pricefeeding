# Unified Price Cache Implementation

## Overview

The pricefeed system has been refactored to support a unified cache that can store price data from both Chainlink and Pyth sources using an interface-based abstraction.

## Key Changes

### 1. Interface-Based Design

**New Interface: `PriceInfo`**
- `GetSource() PriceSource` - Returns the data source (chainlink/pyth)
- `GetNetworkID() uint64` - Returns the network ID
- `GetTimestamp() time.Time` - Returns the timestamp
- `GetPrice() (*big.Int, int)` - Returns the raw price value and exponent
- `GetIdentifier() string` - Returns the unique identifier (feed address for Chainlink, price ID for Pyth)

**Price Source Types:**
```go
type PriceSource string

const (
    SourceChainlink PriceSource = "chainlink"
    SourcePyth      PriceSource = "pyth"
)
```

### 2. Source-Specific Implementations

**ChainlinkPrice** - Implements `PriceInfo` for Chainlink data
- Contains all Chainlink-specific fields (RoundID, Answer, StartedAt, etc.)
- Stores `FeedAddress` for identifier

**PythPrice** - Implements `PriceInfo` for Pyth data
- Contains all Pyth-specific fields (ID, Price, Confidence, Exponent, EMA, etc.)
- Uses `ID` as identifier

### 3. Unified PriceCache

The `PriceCache` now stores `PriceInfo` interface instead of concrete types:
- Uses prefixed identifiers: `"chainlink:0xfeedaddr"` or `"pyth:priceid"`
- Prevents identifier collisions between sources
- Thread-safe operations maintained with `sync.RWMutex`
- Automatic cache pruning when size exceeds 10MB (`MaxCacheSizeBytes`)
- Size estimation based on actual data structures (ChainlinkPrice vs PythPrice)

**New Methods:**
- `AddFeed(networkID, identifier, source)` - Add feed with source specification
- `GetPrice(networkID, identifier, source)` - Get price by source
- `GetAllPrices(networkID)` - Get all prices (all sources)
- `GetAllPricesBySource(networkID, source)` - Get prices filtered by source
- `UpdatePrice(networkID, identifier, source, priceInfo)` - Update price (triggers auto-pruning if needed)
- `estimateSize()` / `estimateSizeUnlocked()` - Estimate cache size in bytes
- `prune()` - Remove old entries while keeping latest for each feed

**Legacy Methods (for backward compatibility):**
- `AddFeedLegacy()`, `GetPriceLegacy()`, `GetAllPricesLegacy()`, `UpdatePriceLegacy()`
- These assume Chainlink source and convert to/from old `PriceData` format

### 4. Updated Components

**PriceCacheManager:**
- Wraps `PriceCache` with additional persistence tracking
- Updated to work with `PriceInfo` interface
- New methods: `GetAllPricesBySource()`, `UpdatePrice()` with source parameter
- Cache size management: `GetCacheSize()`, `PruneCache()` for automatic pruning at 10MB limit
- Status tracking: `UpdateLastSaved()`, `GetLastSaved()`, `PrintStatus()` for monitoring
- Direct cache access: `GetCache()` for advanced use cases
- Legacy methods maintained for backward compatibility (in `legacy.go`)

**PriceCacheManager Methods:**
- `NewPriceCacheManager()` - Creates a new manager with initialized cache
- `UpdatePrice(networkID, identifier, source, priceInfo)` - Updates price in cache
- `GetPrice(networkID, identifier, source)` - Retrieves price from cache
- `GetAllPrices(networkID)` - Gets all prices for a network
- `GetAllPricesBySource(networkID, source)` - Gets prices filtered by source
- `AddFeed(networkID, identifier, source)` - Adds feed to monitor
- `GetCacheSize()` - Returns estimated cache size in bytes
- `PruneCache()` - Manually triggers cache pruning
- `UpdateLastSaved()` / `GetLastSaved()` - Tracks last save timestamp
- `PrintStatus()` - Prints cache status (size, feeds, last saved time)

**Chainlink PriceMonitor:**
- Now uses `ChainlinkPrice` instead of `PriceData`
- Methods updated to work with unified cache
- `GetAllPrices()` returns `map[string]*ChainlinkPrice`

**Pyth PriceMonitor:**
- Now uses `PythPrice` instead of `PythPriceResetData`
- Methods updated to work with unified cache
- `GetAllPrices()` returns `map[string]*PythPrice`
- Feeds are added to cache when `AddPriceFeed()` is called

**main.go:**
- Updated to use new cache methods with source specification
- Chainlink prices: `types.SourceChainlink` (imported from `types` package)
- Pyth prices: `types.SourcePyth` (imported from `types` package)
- Price display logic updated to handle type assertions (`*types.ChainlinkPrice`, `*types.PythPrice`)
- Separate cache managers for Chainlink and Pyth modes
- Periodic cache updates every 15 seconds (Chainlink mode)
- Status monitoring every 60 seconds with price display

## Benefits

1. **Unified Storage**: Single cache system handles both sources
2. **Type Safety**: Interface ensures consistent operations
3. **Extensibility**: Easy to add new price sources (just implement `PriceInfo`)
4. **No Data Loss**: Source-specific fields preserved
5. **Backward Compatibility**: Legacy methods allow gradual migration
6. **Performance**: No overhead beyond interface method calls
7. **Automatic Memory Management**: Cache pruning prevents unbounded growth (10MB limit)
8. **Thread Safety**: All operations are protected with read/write mutexes
9. **Monitoring**: Built-in status tracking and reporting capabilities

## Usage Examples

### Storing Chainlink Price
```go
clPrice := &ChainlinkPrice{
    RoundID: roundData.RoundId,
    Answer: roundData.Answer,
    Exponent: -8, // Negative of contract decimals (e.g., -8 for 8 decimals)
    // ... other fields
    FeedAddress: "0xfeedaddr",
}
cache.UpdatePrice(networkID, "0xfeedaddr", SourceChainlink, clPrice)
```

### Storing Pyth Price
```go
pythPrice := &PythPrice{
    ID: "priceid",
    Price: price,
    Confidence: confidence,
    // ... other fields
}
cache.UpdatePrice(networkID, "priceid", SourcePyth, pythPrice)
```

### Retrieving Prices
```go
// Using PriceCacheManager (recommended)
cacheManager := pricefeed.NewPriceCacheManager()

// Get Chainlink price
clPrice, err := cacheManager.GetPrice(networkID, "0xfeedaddr", types.SourceChainlink)
if err == nil {
    if chainlinkPrice, ok := clPrice.(*types.ChainlinkPrice); ok {
        // Use chainlinkPrice.Answer, chainlinkPrice.RoundID, chainlinkPrice.Exponent, etc.
        // Convert to USD using Exponent: price = Answer / 10^(-Exponent)
        priceFloat := new(big.Float).SetInt(chainlinkPrice.Answer)
        divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-chainlinkPrice.Exponent)), nil))
        priceFloat.Quo(priceFloat, divisor)
        usdPrice, _ := priceFloat.Float64()
    }
}

// Get Pyth price
pythPrice, err := cacheManager.GetPrice(networkID, "priceid", types.SourcePyth)
if err == nil {
    if pythPriceData, ok := pythPrice.(*types.PythPrice); ok {
        // Use pythPriceData.Price, pythPriceData.Confidence, etc.
        // Convert to USD using Exponent
        priceFloat := new(big.Float).SetInt(pythPriceData.Price)
        var exponent *big.Float
        if pythPriceData.Exponent < 0 {
            exponent = new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-pythPriceData.Exponent)), nil))
            priceFloat.Quo(priceFloat, exponent)
        } else {
            exponent = new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(pythPriceData.Exponent)), nil))
            priceFloat.Mul(priceFloat, exponent)
        }
        usdPrice := priceFloat.Text('f', 8)
    }
}

// Get all Chainlink prices for a network
chainlinkPrices := cacheManager.GetAllPricesBySource(networkID, types.SourceChainlink)

// Get all prices (both sources)
allPrices := cacheManager.GetAllPrices(networkID)

// Monitor cache status
cacheManager.PrintStatus() // Prints: size, feeds count, last saved time
cacheSize := cacheManager.GetCacheSize() // Returns size in bytes
```

## Migration Notes

- Old `PriceData` struct is deprecated but still available for backward compatibility
- Old `PythPriceResetData` struct is deprecated but still available
- Legacy cache methods (`*Legacy`) can be used during migration
- All new code should use the new interface-based methods

## Testing

The code compiles successfully and all tests pass. Test files (`chainlink_monitor_test.go`, `pyth_monitor_test.go`) have been updated to use the new interface-based methods:

- `TestCLPriceMonitorCreation` - Tests Chainlink monitor creation with cache manager
- `TestCLPriceMonitorAddFeedWithSymbol` - Tests adding feeds with symbols
- `TestPythPriceMonitorCreation` - Tests Pyth monitor creation with cache manager
- `TestPythPriceMonitorAddFeed` - Tests adding Pyth feeds
- `TestCacheManagerIntegration` - Tests unified cache integration

All tests use `NewPriceCacheManager()` and the new source-aware methods.

## Implementation Details

### Cache Size Management

The cache automatically prunes entries when the estimated size exceeds 10MB (`MaxCacheSizeBytes`). The pruning algorithm:

1. Estimates size based on actual data structures (different for ChainlinkPrice vs PythPrice)
2. Sorts entries by timestamp (oldest first)
3. Keeps the most recent entry for each feed
4. Removes older entries until under the limit
5. Logs pruning activity with size information

### Identifier Format

Identifiers are prefixed with the source to prevent collisions:
- Chainlink: `"chainlink:0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612"`
- Pyth: `"pyth:e62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43"`

When retrieving prices with `GetAllPricesBySource()`, the prefix is automatically stripped from the returned map keys.

### main.go Integration

The main application uses separate cache managers for each mode:

**Chainlink Mode:**
- Creates `PriceCacheManager` at startup
- Updates cache every 15 seconds from monitor prices
- Displays prices every 60 seconds using `GetAllPricesBySource(networkID, types.SourceChainlink)`
- Type asserts to `*types.ChainlinkPrice` for display

**Pyth Mode:**
- Creates separate `PriceCacheManager` instance
- Feeds added to cache when `AddPriceFeed()` is called
- Prices updated immediately when received
- Status displayed every 30 seconds

## Future Enhancements

1. Add more price sources (e.g., Band Protocol, Tellor)
2. Add source-specific query methods
3. Add price aggregation across sources
4. Add price validation and comparison utilities
5. Add persistent storage backend (database, file)
6. Add cache statistics and metrics
7. Add TTL (time-to-live) for cached entries
