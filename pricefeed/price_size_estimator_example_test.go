package pricefeed_test

import (
	"fmt"
	"math/big"
	"time"

	"github.com/morpheum-labs/pricefeeding/pricefeed"
	"github.com/morpheum-labs/pricefeeding/types"
)

// ExampleCustomPrice demonstrates how to create a custom price type
// and register a size estimator for it to work with the cache manager.
type ExampleCustomPrice struct {
	ID        string
	Symbol    string
	Price     *big.Int
	Exponent  int
	Timestamp time.Time
	NetworkID uint64
	Metadata  map[string]string // Custom field that affects size
	ExtraData []byte            // Another custom field
}

// Implement PriceInfo interface
func (p *ExampleCustomPrice) GetSource() types.PriceSource {
	return types.PriceSource("custom")
}

func (p *ExampleCustomPrice) GetNetworkID() uint64 {
	return p.NetworkID
}

func (p *ExampleCustomPrice) GetTimestamp() time.Time {
	return p.Timestamp
}

func (p *ExampleCustomPrice) GetPrice() (*big.Int, int) {
	return p.Price, p.Exponent
}

func (p *ExampleCustomPrice) GetIdentifier() string {
	return p.ID
}

// ExampleRegisterSizeEstimator demonstrates how to register a custom size estimator
// for a custom price type using generics.
func Example_registerSizeEstimator() {
	// Register a size estimator for ExampleCustomPrice
	// This must be done before using the price type with the cache
	pricefeed.RegisterSizeEstimator[*ExampleCustomPrice](func(p *ExampleCustomPrice) int64 {
		size := int64(0)
		// Base fields
		size += int64(len(p.ID)) + 8     // ID string
		size += int64(len(p.Symbol)) + 8 // Symbol string
		size += 32                       // Price *big.Int
		size += 8                        // Exponent int
		size += 15                       // Timestamp time.Time
		size += 8                        // NetworkID uint64

		// Custom fields
		size += 8 // map overhead
		for k, v := range p.Metadata {
			size += int64(len(k)) + int64(len(v)) + 16 // key + value + overhead
		}

		size += int64(len(p.ExtraData)) + 8 // ExtraData slice

		return size
	})

	fmt.Println("Size estimator registered for ExampleCustomPrice")
}

// ExamplePriceCacheManagerWithCustomSize demonstrates how to use
// PriceCacheManager with custom price types that have registered size estimators.
func Example_priceCacheManagerWithCustomSize() {
	// Step 1: Register the size estimator (typically done at package init)
	pricefeed.RegisterSizeEstimator[*ExampleCustomPrice](func(p *ExampleCustomPrice) int64 {
		size := int64(0)
		size += int64(len(p.ID)) + 8
		size += int64(len(p.Symbol)) + 8
		size += 32 + 8 + 15 + 8 // Price, Exponent, Timestamp, NetworkID
		size += 8               // map overhead
		for k, v := range p.Metadata {
			size += int64(len(k)) + int64(len(v)) + 16
		}
		size += int64(len(p.ExtraData)) + 8
		return size
	})

	// Step 2: Create a cache manager
	cacheManager := pricefeed.NewPriceCacheManager()

	// Step 3: Add feeds for your custom price type
	networkID := uint64(1)
	customSource := types.PriceSource("custom")
	cacheManager.AddFeed(networkID, "custom-feed-1", customSource)
	cacheManager.AddFeed(networkID, "custom-feed-2", customSource)

	// Step 4: Update prices - the cache will automatically use the registered size estimator
	customPrice1 := &ExampleCustomPrice{
		ID:        "custom-feed-1",
		Symbol:    "CUSTOM/USD",
		Price:     big.NewInt(100000000),
		Exponent:  -8,
		Timestamp: time.Now(),
		NetworkID: networkID,
		Metadata: map[string]string{
			"source":  "custom-api",
			"version": "v1",
		},
		ExtraData: []byte("some extra data"),
	}

	customPrice2 := &ExampleCustomPrice{
		ID:        "custom-feed-2",
		Symbol:    "OTHER/USD",
		Price:     big.NewInt(200000000),
		Exponent:  -8,
		Timestamp: time.Now(),
		NetworkID: networkID,
		Metadata:  make(map[string]string),
		ExtraData: []byte("minimal data"),
	}

	cacheManager.UpdatePrice(networkID, "custom-feed-1", customSource, customPrice1)
	cacheManager.UpdatePrice(networkID, "custom-feed-2", customSource, customPrice2)

	// Step 5: Check cache size - this uses the registered size estimators
	cacheSize := cacheManager.GetCacheSize()
	fmt.Printf("Cache size: %d bytes (%.2f KB)\n", cacheSize, float64(cacheSize)/1024)

	// Step 6: Retrieve prices
	price1, err := cacheManager.GetPrice(networkID, "custom-feed-1", customSource)
	if err == nil {
		if cp, ok := price1.(*ExampleCustomPrice); ok {
			fmt.Printf("Retrieved price: %s = %s\n", cp.Symbol, cp.Price.String())
		}
	}

	// Step 7: The cache automatically prunes when size exceeds MaxCacheSizeBytes
	// The pruning uses the registered size estimators to accurately calculate sizes
	cacheManager.PrintStatus()

	// Output:
	// Cache size: <size> bytes (<size> KB)
	// Retrieved price: CUSTOM/USD = 100000000
	// 📊 CACHE STATUS
	//    Last Saved: <timestamp>
	//    Time Since Last Save: <duration>
	//    Cache Size: <size> MB / 10.00 MB (<percentage>%)
	//    Total Monitored Feeds: 2
	//    --------------------------------------------------
}

// ExampleSizablePriceInfo demonstrates how to implement the SizablePriceInfo
// interface for automatic size estimation without registration.
type ExampleSizablePrice struct {
	ID        string
	Price     *big.Int
	Exponent  int
	Timestamp time.Time
	NetworkID uint64
	Data      []byte
}

// Implement PriceInfo interface
func (p *ExampleSizablePrice) GetSource() types.PriceSource {
	return types.PriceSource("sizable")
}

func (p *ExampleSizablePrice) GetNetworkID() uint64 {
	return p.NetworkID
}

func (p *ExampleSizablePrice) GetTimestamp() time.Time {
	return p.Timestamp
}

func (p *ExampleSizablePrice) GetPrice() (*big.Int, int) {
	return p.Price, p.Exponent
}

func (p *ExampleSizablePrice) GetIdentifier() string {
	return p.ID
}

// Implement SizablePriceInfo interface - no registration needed!
func (p *ExampleSizablePrice) EstimateSize() int64 {
	size := int64(0)
	size += int64(len(p.ID)) + 8
	size += 32 + 8 + 15 + 8 // Price, Exponent, Timestamp, NetworkID
	size += int64(len(p.Data)) + 8
	return size
}

// ExampleUsingSizablePriceInfo demonstrates using a type that implements
// SizablePriceInfo - no registration required!
func Example_usingSizablePriceInfo() {
	cacheManager := pricefeed.NewPriceCacheManager()

	// No registration needed - the type implements SizablePriceInfo
	sizablePrice := &ExampleSizablePrice{
		ID:        "sizable-feed-1",
		Price:     big.NewInt(50000000),
		Exponent:  -8,
		Timestamp: time.Now(),
		NetworkID: 1,
		Data:      []byte("some data"),
	}

	cacheManager.AddFeed(1, "sizable-feed-1", types.PriceSource("sizable"))
	cacheManager.UpdatePrice(1, "sizable-feed-1", types.PriceSource("sizable"), sizablePrice)

	// Cache size calculation automatically uses EstimateSize() method
	size := cacheManager.GetCacheSize()
	fmt.Printf("Cache size with SizablePriceInfo: %d bytes\n", size)

	// Output:
	// Cache size with SizablePriceInfo: <size> bytes
}

// ExampleMixedPriceTypes demonstrates using multiple price types
// (built-in and custom) with the cache manager.
func Example_mixedPriceTypes() {
	// Register custom price type
	pricefeed.RegisterSizeEstimator[*ExampleCustomPrice](func(p *ExampleCustomPrice) int64 {
		return int64(len(p.ID)) + int64(len(p.Symbol)) + 100
	})

	cacheManager := pricefeed.NewPriceCacheManager()
	networkID := uint64(42161)

	// Add Chainlink feed (built-in, no registration needed)
	cacheManager.AddFeed(networkID, "0x1234...", types.SourceChainlink)
	chainlinkPrice := &types.ChainlinkPrice{
		Answer:      big.NewInt(5000000000),
		Exponent:    -8,
		Timestamp:   time.Now(),
		NetworkID:   networkID,
		FeedAddress: "0x1234...",
	}
	cacheManager.UpdatePrice(networkID, "0x1234...", types.SourceChainlink, chainlinkPrice)

	// Add Pyth feed (built-in, no registration needed)
	cacheManager.AddFeed(networkID, "pyth-id-123", types.SourcePyth)
	pythPrice := &types.PythPrice{
		ID:        "pyth-id-123",
		Symbol:    "BTC/USD",
		Price:     big.NewInt(6000000000),
		Exponent:  -8,
		Timestamp: time.Now(),
		NetworkID: networkID,
	}
	cacheManager.UpdatePrice(networkID, "pyth-id-123", types.SourcePyth, pythPrice)

	// Add custom feed (requires registration)
	cacheManager.AddFeed(networkID, "custom-1", types.PriceSource("custom"))
	customPrice := &ExampleCustomPrice{
		ID:        "custom-1",
		Symbol:    "CUSTOM/USD",
		Price:     big.NewInt(7000000000),
		Exponent:  -8,
		Timestamp: time.Now(),
		NetworkID: networkID,
		Metadata:  make(map[string]string),
		ExtraData: []byte("data"),
	}
	cacheManager.UpdatePrice(networkID, "custom-1", types.PriceSource("custom"), customPrice)

	// All price types use their respective size estimators automatically
	totalSize := cacheManager.GetCacheSize()
	fmt.Printf("Total cache size with mixed types: %d bytes\n", totalSize)

	// Output:
	// Total cache size with mixed types: <size> bytes
}
