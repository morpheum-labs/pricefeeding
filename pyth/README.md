# Pyth Hermes Go Client

A Go client library for interacting with the Pyth Network Hermes service, providing real-time pricing data for various asset classes including cryptocurrency, equities, FX, and commodities.

## Features

- **Price Feed Discovery**: Query available price feeds by symbol and asset type
- **Latest Price Updates**: Get the most recent price updates for specific price feed IDs
- **Historical Price Updates**: Retrieve price updates at specific timestamps
- **TWAP (Time Weighted Average Price)**: Calculate TWAPs over configurable time windows
- **Publisher Stake Caps**: Get latest publisher stake cap information
- **Real-time Streaming**: Subscribe to live price updates via Server-Sent Events
- **Robust HTTP Client**: Built-in retry logic with exponential backoff
- **Configurable Timeouts**: Customizable request timeouts and retry behavior

## Installation

```bash
go get github.com/morpheum/chainlink-price-feed-golang/pyth
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/morpheum/chainlink-price-feed-golang/pyth"
)

func main() {
    // Create a new Hermes client
    client := pyth.NewHermesClient("https://hermes.pyth.network", nil)
    
    ctx := context.Background()
    
    // Get price feeds
    priceFeeds, err := client.GetPriceFeeds(ctx, &pyth.GetPriceFeedsOptions{
        Query:     stringPtr("btc"),
        AssetType: &pyth.AssetTypeCrypto,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    for _, feed := range priceFeeds {
        fmt.Printf("ID: %s, Symbol: %s\n", feed.ID, feed.Symbol)
    }
    
    // Get latest price updates
    priceIds := []pyth.HexString{
        "0xe62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43", // BTC/USD
        "0xff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace", // ETH/USD
    }
    
    parsed := true
    updates, err := client.GetLatestPriceUpdates(ctx, priceIds, &pyth.GetLatestPriceUpdatesOptions{
        Encoding: &pyth.EncodingTypeHex,
        Parsed:   &parsed,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    if updates.Parsed != nil {
        for _, feed := range updates.Parsed.PriceFeeds {
            fmt.Printf("Price Feed: %s, Price: %s\n", feed.ID, feed.Price.Price)
        }
    }
}

func stringPtr(s string) *string {
    return &s
}
```

## Streaming Price Updates

```go
// Create client with custom configuration
config := &pyth.HermesClientConfig{
    Timeout:     pyth.DurationInMsPtr(10000),
    HTTPRetries: intPtr(5),
    Headers: map[string]string{
        "Authorization": "Bearer your-token",
    },
}
client := pyth.NewHermesClient("https://hermes.pyth.network", config)

// Stream price updates
eventSource, err := client.GetPriceUpdatesStream(ctx, priceIds, &pyth.GetPriceUpdatesStreamOptions{
    Encoding:       &pyth.EncodingTypeHex,
    Parsed:         &parsed,
    AllowUnordered: boolPtr(false),
    BenchmarksOnly: boolPtr(true),
})
if err != nil {
    log.Fatal(err)
}

// Set up event handlers
eventSource.OnMessage(func(data string) {
    fmt.Println("Received price update:", data)
})

eventSource.OnError(func(err error) {
    fmt.Printf("Stream error: %v\n", err)
    eventSource.Close()
})

// Close when done
defer eventSource.Close()
```

## Configuration

### HermesClientConfig

```go
type HermesClientConfig struct {
    // Timeout of each request (for all of retries). Default: 5000ms
    Timeout *DurationInMs
    
    // Number of times a HTTP request will be retried before the API returns a failure. Default: 3.
    // The connection uses exponential back-off for the delay between retries.
    HTTPRetries *int
    
    // Optional headers to be included in every request.
    Headers map[string]string
}
```

### Asset Types

```go
const (
    AssetTypeCrypto               AssetType = "crypto"
    AssetTypeEquity               AssetType = "equity"
    AssetTypeFX                   AssetType = "fx"
    AssetTypeMetal                AssetType = "metal"
    AssetTypeRates                AssetType = "rates"
    AssetTypeCryptoRedemptionRate AssetType = "crypto_redemption_rate"
)
```

### Encoding Types

```go
const (
    EncodingTypeHex    EncodingType = "hex"
    EncodingTypeBase64 EncodingType = "base64"
)
```

## API Methods

### GetPriceFeeds

Retrieve available price feeds with optional filtering.

```go
feeds, err := client.GetPriceFeeds(ctx, &pyth.GetPriceFeedsOptions{
    Query:     stringPtr("bitcoin"),
    AssetType: &pyth.AssetTypeCrypto,
})
```

### GetLatestPriceUpdates

Get the most recent price updates for specific price feed IDs.

```go
updates, err := client.GetLatestPriceUpdates(ctx, priceIds, &pyth.GetLatestPriceUpdatesOptions{
    Encoding:            &pyth.EncodingTypeHex,
    Parsed:              boolPtr(true),
    IgnoreInvalidPriceIds: boolPtr(false),
})
```

### GetPriceUpdatesAtTimestamp

Retrieve price updates at a specific timestamp.

```go
updates, err := client.GetPriceUpdatesAtTimestamp(ctx, timestamp, priceIds, &pyth.GetPriceUpdatesAtTimestampOptions{
    Encoding: &pyth.EncodingTypeHex,
    Parsed:   boolPtr(true),
})
```

### GetLatestTwaps

Calculate Time Weighted Average Prices over a specified window.

```go
twaps, err := client.GetLatestTwaps(ctx, priceIds, 300, &pyth.GetLatestTwapsOptions{
    Encoding: &pyth.EncodingTypeHex,
    Parsed:   boolPtr(true),
})
```

### GetLatestPublisherCaps

Get the latest publisher stake cap information.

```go
caps, err := client.GetLatestPublisherCaps(ctx, &pyth.GetLatestPublisherCapsOptions{
    Encoding: &pyth.EncodingTypeHex,
    Parsed:   boolPtr(true),
})
```

### GetPriceUpdatesStream

Subscribe to real-time price updates via Server-Sent Events.

```go
eventSource, err := client.GetPriceUpdatesStream(ctx, priceIds, &pyth.GetPriceUpdatesStreamOptions{
    Encoding:            &pyth.EncodingTypeHex,
    Parsed:              boolPtr(true),
    AllowUnordered:      boolPtr(false),
    BenchmarksOnly:      boolPtr(true),
    IgnoreInvalidPriceIds: boolPtr(false),
})
```

## Hermes Endpoints

Pyth offers a free public endpoint at [https://hermes.pyth.network](https://hermes.pyth.network). However, it is recommended to obtain a private endpoint from one of the Hermes RPC providers for more reliability. You can find more information about Hermes RPC providers [here](https://docs.pyth.network/documentation/pythnet-price-feeds/hermes#public-endpoint).

## Error Handling

The client includes robust error handling with automatic retries and exponential backoff. All methods return errors that should be checked:

```go
updates, err := client.GetLatestPriceUpdates(ctx, priceIds, nil)
if err != nil {
    log.Printf("Failed to get price updates: %v", err)
    return
}
```

## Testing

Run the tests with:

```bash
go test ./pyth/...
```

## Example

See the `example/` directory for a complete example that demonstrates all the client features.

## License

This project is licensed under the Apache-2.0 License.
