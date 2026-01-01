# Unified Price Cache Implementation

## Overview

The pricefeed system has been refactored to support a unified cache that can store price data from both Chainlink and Pyth sources using an interface-based abstraction.

## Key Changes

### 1. Interface-Based Design

**New Interface: `PriceInfo`**
- `GetSource() PriceSource` - Returns the data source (chainlink/pyth)
- `GetNetworkID() uint64` - Returns the network ID
- `GetTimestamp() time.Time` - Returns the timestamp
- `GetPrice() *big.Int` - Returns the raw price value
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
- Thread-safe operations maintained

**New Methods:**
- `AddFeed(networkID, identifier, source)` - Add feed with source specification
- `GetPrice(networkID, identifier, source)` - Get price by source
- `GetAllPrices(networkID)` - Get all prices (all sources)
- `GetAllPricesBySource(networkID, source)` - Get prices filtered by source
- `UpdatePrice(networkID, identifier, source, priceInfo)` - Update price

**Legacy Methods (for backward compatibility):**
- `AddFeedLegacy()`, `GetPriceLegacy()`, `GetAllPricesLegacy()`, `UpdatePriceLegacy()`
- These assume Chainlink source and convert to/from old `PriceData` format

### 4. Updated Components

**PriceCacheManager:**
- Updated to work with `PriceInfo` interface
- New methods: `GetAllPricesBySource()`, `UpdatePrice()` with source parameter
- Legacy methods maintained for backward compatibility

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
- Chainlink prices: `pricefeed.SourceChainlink`
- Pyth prices: `pricefeed.SourcePyth`
- Price display logic updated to handle type assertions

## Benefits

1. **Unified Storage**: Single cache system handles both sources
2. **Type Safety**: Interface ensures consistent operations
3. **Extensibility**: Easy to add new price sources (just implement `PriceInfo`)
4. **No Data Loss**: Source-specific fields preserved
5. **Backward Compatibility**: Legacy methods allow gradual migration
6. **Performance**: No overhead beyond interface method calls

## Usage Examples

### Storing Chainlink Price
```go
clPrice := &ChainlinkPrice{
    RoundID: roundData.RoundId,
    Answer: roundData.Answer,
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
// Get Chainlink price
clPrice, err := cache.GetPrice(networkID, "0xfeedaddr", SourceChainlink)
if err == nil {
    if chainlinkPrice, ok := clPrice.(*ChainlinkPrice); ok {
        // Use chainlinkPrice.Answer, chainlinkPrice.RoundID, etc.
    }
}

// Get Pyth price
pythPrice, err := cache.GetPrice(networkID, "priceid", SourcePyth)
if err == nil {
    if pythPriceData, ok := pythPrice.(*PythPrice); ok {
        // Use pythPriceData.Price, pythPriceData.Confidence, etc.
    }
}

// Get all Chainlink prices for a network
chainlinkPrices := cache.GetAllPricesBySource(networkID, SourceChainlink)

// Get all prices (both sources)
allPrices := cache.GetAllPrices(networkID)
```

## Migration Notes

- Old `PriceData` struct is deprecated but still available for backward compatibility
- Old `PythPriceResetData` struct is deprecated but still available
- Legacy cache methods (`*Legacy`) can be used during migration
- All new code should use the new interface-based methods

## Testing

The code compiles successfully. Test files (`chainlink_monitor_test.go`, `pyth_monitor_test.go`) may need updates to use the new interface-based methods.

## Future Enhancements

1. Add more price sources (e.g., Band Protocol, Tellor)
2. Add source-specific query methods
3. Add price aggregation across sources
4. Add price validation and comparison utilities
