# Pyth Price Monitor

This package provides a comprehensive price monitoring solution for Pyth Network price feeds with immediate price printing and cache persistence tracking.

## Features

- **Real-time Price Monitoring**: Monitor multiple Pyth price feeds simultaneously
- **Immediate Price Printing**: Print price updates to screen as they arrive
- **Cache Persistence**: Track when prices were last saved with `lastSaved` timestamp
- **Thread-safe Operations**: All operations are thread-safe for concurrent access
- **Configurable Update Intervals**: Set custom intervals for price updates
- **Graceful Shutdown**: Proper cleanup and status reporting on shutdown

## Usage

### Basic Setup

```go
package main

import (
    "time"
    "github.com/morpheum-labs/pricefeeding/pricefeed"
)

func main() {
    // Create a new Pyth price monitor
    endpoint := "https://hermes.pyth.network"
    interval := 5 * time.Second
    immediateMode := true // Print prices immediately when received
    
    monitor := pricefeed.NewPythPriceMonitor(endpoint, interval, immediateMode)
    
    // Add price feeds to monitor
    monitor.AddPriceFeed("0xe62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43", "BTC/USD")
    monitor.AddPriceFeed("0xff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace", "ETH/USD")
    
    // Start monitoring
    go monitor.Start()
    
    // Your application logic here...
}
```

### Price Feed Management

```go
// Add a new price feed
monitor.AddPriceFeed("price-id", "SYMBOL")

// Get a specific price
price, err := monitor.GetPrice("price-id")
if err != nil {
    log.Printf("Error getting price: %v", err)
} else {
    log.Printf("Price: %s", price.Price.String())
}

// Get all prices
allPrices := monitor.GetAllPrices()
for priceID, priceData := range allPrices {
    log.Printf("%s: %s", priceData.Symbol, priceData.Price.String())
}
```

### Cache Status and Last Saved Tracking

```go
// Print current cache status
monitor.PrintLastSavedStatus()

// Get the cache manager for advanced operations
cacheManager := monitor.GetCacheManager()
lastSaved := cacheManager.GetLastSaved()
log.Printf("Last saved: %s", lastSaved.Format("2006-01-02 15:04:05"))
```

### Configuration

```go
// Toggle immediate mode
monitor.SetImmediateMode(false) // Disable immediate printing

// Change update interval (requires restart)
monitor := pricefeed.NewPythPriceMonitor(endpoint, 10*time.Second, true)
```

## Data Structures

### PythPriceData

```go
type PythPriceData struct {
    ID            string    `json:"id"`
    Symbol        string    `json:"ticker,omitempty"`
    Price         *big.Int  `json:"price"`
    Confidence    *big.Int  `json:"confidence"`
    Exponent      int       `json:"exponent"`
    PublishTime   int64     `json:"publish_time"`
    Slot          int64     `json:"slot"`
    Timestamp     time.Time `json:"timestamp"`
    NetworkID     uint64    `json:"network_id"`
    EMA           *big.Int  `json:"ema,omitempty"`
    EMAConfidence *big.Int  `json:"ema_confidence,omitempty"`
}
```

## Example Output

When `immediateMode` is enabled, the monitor will print price updates like this:

```
🔄 PYTH PRICE UPDATE [12:34:56]
   Symbol: BTC/USD
   Price ID: 0xe62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43
   Price: 50000.00000000
   Confidence: ±1.00000000
   Publish Time: 12:34:55
   Slot: 12345
   EMA: 49950.00000000
   Last Saved: 12:34:56
   --------------------------------------------------
```

## Status Output

The cache status shows:

```
📊 PYTH CACHE STATUS
   Last Saved: 2024-01-15 12:34:56
   Time Since Last Save: 2.5s
   Monitored Feeds: 3
   --------------------------------------------------
```

## Error Handling

The monitor includes comprehensive error handling:

- Network connection failures are logged and retried
- Invalid price IDs are handled gracefully
- Timeout errors are managed with configurable timeouts
- All operations are thread-safe

## Testing

Run the tests to verify functionality:

```bash
go test ./pricefeed -v
```

## Dependencies

- `github.com/morpheum-labs/pricefeeding/pyth` - Pyth client library
- Standard Go libraries for HTTP, JSON, and concurrency

## Notes

- The monitor uses a default network ID of 0 for Pyth feeds
- Price data is cached locally and updated at the specified interval
- The `lastSaved` timestamp is updated every time new price data is received
- All price calculations account for the exponent to show actual decimal values
