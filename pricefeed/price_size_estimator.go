package pricefeed

import (
	"reflect"
	"sync"

	"github.com/morpheum-labs/pricefeeding/types"
)

// Package pricefeed provides extensible size estimation for price cache management.
//
// The size estimator supports three extension mechanisms:
//
//  1. SizablePriceInfo interface: Types can implement EstimateSize() method
//  2. RegisterSizeEstimator: Register custom estimators using generics (type-safe)
//  3. Built-in support: ChainlinkPrice and PythPrice are handled automatically
//
// The PriceCacheManager automatically uses size estimators for:
//   - Cache size calculation (GetCacheSize())
//   - Automatic pruning when cache exceeds MaxCacheSizeBytes
//
// See price_size_estimator_example_test.go for complete usage examples.

// SizablePriceInfo is an optional interface that PriceInfo types can implement
// to provide their own size estimation. This allows custom types to self-report
// their memory footprint.
type SizablePriceInfo interface {
	types.PriceInfo
	// EstimateSize returns the estimated memory size of this price info in bytes
	EstimateSize() int64
}

var (
	// customSizeEstimators is a registry of custom size estimation functions
	// for specific types that cannot implement SizablePriceInfo
	customSizeEstimators = make(map[reflect.Type]func(types.PriceInfo) int64)
	customEstimatorsMu   sync.RWMutex
)

// RegisterSizeEstimator registers a custom size estimation function for a specific type using generics.
// This allows external packages to provide type-safe size estimation for their custom PriceInfo types
// without modifying this package.
//
// The registered estimator will be used automatically by PriceCacheManager when calculating cache size
// and during automatic pruning operations.
//
// Example usage with PriceCacheManager:
//
//	// 1. Define your custom price type
//	type MyCustomPrice struct {
//	    ID        string
//	    Price     *big.Int
//	    Data      []byte
//	    Metadata  map[string]string
//	}
//	// ... implement types.PriceInfo interface ...
//
//	// 2. Register size estimator (typically in init() or at startup)
//	RegisterSizeEstimator[*MyCustomPrice](func(p *MyCustomPrice) int64 {
//	    size := int64(len(p.ID)) + 8
//	    size += 32 // Price *big.Int
//	    size += int64(len(p.Data)) + 8
//	    size += 8 // map overhead
//	    for k, v := range p.Metadata {
//	        size += int64(len(k)) + int64(len(v)) + 16
//	    }
//	    return size
//	})
//
//	// 3. Use with PriceCacheManager
//	cacheManager := NewPriceCacheManager()
//	cacheManager.AddFeed(networkID, "my-feed", types.PriceSource("custom"))
//	cacheManager.UpdatePrice(networkID, "my-feed", types.PriceSource("custom"), &MyCustomPrice{...})
//	// Cache size calculation and pruning will automatically use the registered estimator
func RegisterSizeEstimator[T types.PriceInfo](estimator func(T) int64) {
	customEstimatorsMu.Lock()
	defer customEstimatorsMu.Unlock()

	var zero T
	priceType := reflect.TypeOf(zero)

	// Wrap the generic function to work with the PriceInfo interface
	customSizeEstimators[priceType] = func(pi types.PriceInfo) int64 {
		if p, ok := pi.(T); ok {
			return estimator(p)
		}
		// Fallback if type assertion fails (shouldn't happen if used correctly)
		return 100
	}
}

// EstimateSize is a generic helper function that provides type-safe size estimation.
// It first checks if the type implements SizablePriceInfo, then falls back to
// registered estimators or built-in calculations.
//
// This is useful when you want type-safe size estimation without type assertions.
// The PriceCacheManager uses EstimatePriceInfoSize internally, which works with
// the PriceInfo interface.
//
// Example:
//
//	var price *MyCustomPrice = ...
//	size := EstimateSize(price)  // Type-safe, no interface conversion needed
func EstimateSize[T types.PriceInfo](price T) int64 {
	return EstimatePriceInfoSize(price)
}

// EstimatePriceInfoSize estimates the memory size of a PriceInfo in bytes.
// It uses the following priority order:
// 1. If the type implements SizablePriceInfo, use its EstimateSize() method
// 2. If a custom estimator is registered for the type, use it
// 3. Fall back to built-in type-specific calculations
// 4. Return a conservative default estimate for unknown types
func EstimatePriceInfoSize(priceInfo types.PriceInfo) int64 {
	if priceInfo == nil {
		return 0
	}

	// First, check if the type implements SizablePriceInfo interface
	if sizable, ok := priceInfo.(SizablePriceInfo); ok {
		return sizable.EstimateSize()
	}

	// Second, check for registered custom estimators
	customEstimatorsMu.RLock()
	priceType := reflect.TypeOf(priceInfo)
	if estimator, exists := customSizeEstimators[priceType]; exists {
		customEstimatorsMu.RUnlock()
		return estimator(priceInfo)
	}
	customEstimatorsMu.RUnlock()

	// Third, fall back to built-in type-specific calculations
	switch p := priceInfo.(type) {
	case *types.ChainlinkPrice:
		// ChainlinkPrice: 5 *big.Int + time.Time + int + uint64 + string
		return 5*32 + // 5 big.Int values (rough estimate)
			15 + // time.Time
			8 + // int (exponent)
			8 + // uint64 (networkID)
			int64(len(p.FeedAddress)) + 8 // string (feedAddress)
	case *types.PythPrice:
		// PythPrice: ID, Symbol, Price, Confidence, EMA, EMAConfidence (all *big.Int or string), Exponent, PublishTime, Slot, Timestamp, NetworkID
		return int64(len(p.ID)) + 8 +
			int64(len(p.Symbol)) + 8 +
			32 + // Price *big.Int
			32 + // Confidence *big.Int
			32 + // EMA *big.Int (if present)
			32 + // EMAConfidence *big.Int (if present)
			8 + // Exponent
			8 + // PublishTime
			8 + // Slot
			15 + // Timestamp
			8 // NetworkID
	default:
		// Unknown type, return a conservative estimate
		return 100
	}
}
