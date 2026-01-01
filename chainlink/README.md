# Chainlink Client Package

This package provides a clean interface for interacting with Chainlink price feed aggregator contracts on Ethereum-compatible networks.

## Overview

The package extracts the core Chainlink EVM request mechanism from the monitoring layer, providing a reusable client for fetching price data from Chainlink aggregator contracts.

## Features

- **Contract Interaction**: Direct interaction with Chainlink AggregatorV3Interface contracts
- **RPC Switching**: Automatic RPC endpoint switching on errors (error code -32097)
- **Retry Logic**: Configurable retry mechanism with exponential backoff
- **Error Detection**: Built-in detection of execution reverted errors
- **Thread-Safe**: Designed for concurrent use

## Usage

### Basic Usage

```go
import (
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/morpheum-labs/pricefeeding/chainlink"
)

// Create an Ethereum client
client, _ := ethclient.Dial("https://arb1.arbitrum.io/rpc")

// Fetch price data
opts := chainlink.FetchPriceDataOptions{
    NetworkID:   42161, // Arbitrum
    FeedAddress: "0x6ce185860a4963106506C203335A2910413708e9", // BTC/USD feed
    Client:      client,
    MaxRetries:  1,
    RetryDelay:  2 * time.Second,
}

priceData, err := chainlink.FetchPriceData(opts)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Price: %s\n", priceData.Answer.String())
fmt.Printf("Exponent: %d\n", priceData.Exponent) // Negative of decimals (e.g., -8 for 8 decimals)

// Convert to USD using Exponent
priceFloat := new(big.Float).SetInt(priceData.Answer)
divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-priceData.Exponent)), nil))
priceFloat.Quo(priceFloat, divisor)
fmt.Printf("USD Price: %s\n", priceFloat.Text('f', 8))
```

### With RPC Switching

```go
// Implement RPCSwitcher interface
type MyRPCSwitcher struct {
    // ... your implementation
}

func (m *MyRPCSwitcher) SwitchRPCEndpointImmediately(networkID uint64) error {
    // Switch RPC endpoint logic
    return nil
}

func (m *MyRPCSwitcher) GetBestClient(networkID uint64) (*ethclient.Client, error) {
    // Return best available client
    return client, nil
}

// Use with RPC switcher
opts := chainlink.FetchPriceDataOptions{
    NetworkID:   42161,
    FeedAddress: "0x6ce185860a4963106506C203335A2910413708e9",
    Client:      client,
    RPCSwitcher: &MyRPCSwitcher{},
    MaxRetries:  1,
    RetryDelay:  2 * time.Second,
}

priceData, err := chainlink.FetchPriceData(opts)
```

## API Reference

### Types

#### `RPCSwitcher` Interface
```go
type RPCSwitcher interface {
    SwitchRPCEndpointImmediately(networkID uint64) error
    GetBestClient(networkID uint64) (*ethclient.Client, error)
}
```

#### `FetchPriceDataOptions` Struct
```go
type FetchPriceDataOptions struct {
    NetworkID   uint64         // Network ID (e.g., 42161 for Arbitrum)
    FeedAddress string         // Chainlink aggregator contract address
    Client      *ethclient.Client // Ethereum client
    RPCSwitcher RPCSwitcher    // Optional RPC switcher for retry logic
    MaxRetries  int            // Maximum retries (default: 1)
    RetryDelay  time.Duration  // Delay between retries (default: 2s)
}
```

### Functions

#### `FetchPriceData(opts FetchPriceDataOptions) (*types.ChainlinkPrice, error)`
Main entry point for fetching Chainlink price data. Handles contract interaction, error detection, and optional RPC switching.

Returns a `ChainlinkPrice` struct that includes:
- `Answer`: The raw price value from the contract
- `Exponent`: Negative of the contract's decimals (e.g., -8 for 8 decimals)
- Other round data fields (RoundID, Timestamp, etc.)

The `Exponent` field is automatically fetched from the contract's `decimals()` function and stored as a negative value to match Pyth's format. This allows consistent price conversion across different oracle sources.

#### `IsErrorCode32097(err error) bool`
Checks if an error contains the specific error code -32097, which typically indicates execution reverted and may require RPC switching.

## Error Handling

The client automatically detects error code -32097 (execution reverted) and can trigger RPC switching if an `RPCSwitcher` is provided. This helps maintain reliability when RPC endpoints become unavailable.

## Integration with Price Monitor

The `pricefeed.CLPriceMonitor` uses this package internally for all Chainlink contract interactions. The monitor provides an adapter (`rpcSwitcherAdapter`) that bridges the `rpcscan.NetworkConfiguration` to the `chainlink.RPCSwitcher` interface.

## Integration with Unified Cache

The Chainlink client returns `*types.ChainlinkPrice` which implements the `types.PriceInfo` interface. This allows seamless integration with the unified price cache system:

- **Unified Storage**: Chainlink prices are stored alongside Pyth prices in the same cache using source-prefixed identifiers (`chainlink:0xfeedaddr`)
- **Type Safety**: The `PriceInfo` interface ensures consistent operations across different price sources
- **Cache Integration**: Prices are automatically stored in `PriceCacheManager` when fetched by the monitor

Example integration:
```go
// Fetch price data
priceData, err := chainlink.FetchPriceData(opts)
if err != nil {
    log.Fatal(err)
}

// priceData is *types.ChainlinkPrice which implements types.PriceInfo
// It can be directly stored in the unified cache:
cacheManager.UpdatePrice(networkID, feedAddress, types.SourceChainlink, priceData)
```

## Dependencies

- `github.com/ethereum/go-ethereum/ethclient` - Ethereum client
- `github.com/morpheum-labs/pricefeeding/aggregatorv3` - Chainlink contract bindings
- `github.com/morpheum-labs/pricefeeding/types` - Shared types

## Thread Safety

The `FetchPriceData` function is thread-safe and can be called concurrently from multiple goroutines. Each call uses its own client instance and options.
