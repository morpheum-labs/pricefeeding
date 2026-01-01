package pricefeed

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/morpheum-labs/pricefeeding/types"
)

const (
	// MaxCacheSizeBytes is the maximum size of the cache in bytes (10MB)
	MaxCacheSizeBytes = 10 * 1024 * 1024
)

// PriceCache stores price data with thread-safe access
// Uses PriceInfo interface to support multiple sources (Chainlink, Pyth, etc.)
type PriceCache struct {
	mu    sync.RWMutex
	data  map[uint64]map[string]types.PriceInfo // networkID -> prefixedIdentifier -> PriceInfo
	feeds map[uint64][]string                   // networkID -> list of prefixed identifiers (e.g., "chainlink:0xaddr", "pyth:id")
}

// NewPriceCache creates a new price cache
func NewPriceCache() *PriceCache {
	return &PriceCache{
		data:  make(map[uint64]map[string]types.PriceInfo),
		feeds: make(map[uint64][]string),
	}
}

// makePrefixedIdentifier creates a prefixed identifier for a price source
func makePrefixedIdentifier(source types.PriceSource, identifier string) string {
	return string(source) + ":" + identifier
}

// AddFeed adds a price feed to monitor for a specific network
func (pc *PriceCache) AddFeed(networkID uint64, identifier string, source types.PriceSource) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	prefixed := makePrefixedIdentifier(source, identifier)

	if pc.data[networkID] == nil {
		pc.data[networkID] = make(map[string]types.PriceInfo)
		pc.feeds[networkID] = make([]string, 0)
	}

	// Check if feed already exists
	for _, existing := range pc.feeds[networkID] {
		if existing == prefixed {
			return // Already exists
		}
	}

	pc.feeds[networkID] = append(pc.feeds[networkID], prefixed)
	log.Printf("Added price feed %s for network %d (source: %s)", identifier, networkID, source)
}

// GetPrice retrieves the latest price for a specific feed
func (pc *PriceCache) GetPrice(networkID uint64, identifier string, source types.PriceSource) (types.PriceInfo, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	prefixed := makePrefixedIdentifier(source, identifier)

	if pc.data[networkID] == nil {
		return nil, fmt.Errorf("no data for network %d", networkID)
	}

	priceInfo, exists := pc.data[networkID][prefixed]
	if !exists {
		return nil, fmt.Errorf("no price data for feed %s on network %d (source: %s)", identifier, networkID, source)
	}

	return priceInfo, nil
}

// GetAllPrices retrieves all prices for a specific network
func (pc *PriceCache) GetAllPrices(networkID uint64) map[string]types.PriceInfo {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.data[networkID] == nil {
		return make(map[string]types.PriceInfo)
	}

	// Create a copy to avoid race conditions
	result := make(map[string]types.PriceInfo)
	for prefixed, priceInfo := range pc.data[networkID] {
		result[prefixed] = priceInfo
	}

	return result
}

// GetAllPricesBySource retrieves all prices for a specific network and source
func (pc *PriceCache) GetAllPricesBySource(networkID uint64, source types.PriceSource) map[string]types.PriceInfo {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	result := make(map[string]types.PriceInfo)
	if pc.data[networkID] == nil {
		return result
	}

	prefix := string(source) + ":"
	for prefixed, priceInfo := range pc.data[networkID] {
		if strings.HasPrefix(prefixed, prefix) {
			// Extract the identifier (remove the prefix)
			identifier := strings.TrimPrefix(prefixed, prefix)
			result[identifier] = priceInfo
		}
	}

	return result
}

// UpdatePrice updates the price data for a specific feed
func (pc *PriceCache) UpdatePrice(networkID uint64, identifier string, source types.PriceSource, priceInfo types.PriceInfo) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	prefixed := makePrefixedIdentifier(source, identifier)

	if pc.data[networkID] == nil {
		pc.data[networkID] = make(map[string]types.PriceInfo)
	}

	pc.data[networkID][prefixed] = priceInfo

	// Ensure feed is in the feeds list
	found := false
	for _, existing := range pc.feeds[networkID] {
		if existing == prefixed {
			found = true
			break
		}
	}
	if !found {
		pc.feeds[networkID] = append(pc.feeds[networkID], prefixed)
	}

	// Check cache size and prune if necessary (unlock first to avoid deadlock)
	size := pc.estimateSizeUnlocked()
	if size > MaxCacheSizeBytes {
		pc.mu.Unlock()
		pc.prune()
		pc.mu.Lock()
	}
}

// estimateSize estimates the current size of the cache in bytes
// This is a rough estimation based on the data structures
func (pc *PriceCache) estimateSize() int64 {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.estimateSizeUnlocked()
}

// estimateSizeUnlocked estimates size without acquiring locks (caller must hold lock)
func (pc *PriceCache) estimateSizeUnlocked() int64 {
	var size int64

	// Estimate size of maps overhead
	// Each map entry has overhead: ~8 bytes for key + pointer overhead
	for _, networkData := range pc.data {
		// Network ID overhead (8 bytes)
		size += 8

		// For each price entry in the network
		for prefixed, priceInfo := range networkData {
			// String key size (prefixed identifier)
			size += int64(len(prefixed))
			size += 8 // String header overhead

			// Estimate PriceInfo size based on type
			size += EstimatePriceInfoSize(priceInfo)
		}
	}

	// Estimate feeds map overhead
	for _, feedList := range pc.feeds {
		size += 8 // networkID
		for _, feed := range feedList {
			size += int64(len(feed)) + 8 // string + slice overhead
		}
	}

	return size
}

// prune removes old entries from the cache to keep it under the size limit
// It keeps the most recent entry for each feed and removes older entries
func (pc *PriceCache) prune() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Collect all entries with their timestamps
	type cacheEntry struct {
		networkID uint64
		prefixed  string
		priceInfo types.PriceInfo
		timestamp time.Time
		entrySize int64
	}

	var entries []cacheEntry
	var totalSize int64

	// Collect all entries
	for networkID, networkData := range pc.data {
		for prefixed, priceInfo := range networkData {
			timestamp := priceInfo.GetTimestamp()
			// Estimate entry size
			entrySize := int64(len(prefixed)) + 8 // key size
			entrySize += EstimatePriceInfoSize(priceInfo)

			entries = append(entries, cacheEntry{
				networkID: networkID,
				prefixed:  prefixed,
				priceInfo: priceInfo,
				timestamp: timestamp,
				entrySize: entrySize,
			})
			totalSize += entrySize
		}
	}

	// If we're under the limit, no need to prune
	if totalSize <= MaxCacheSizeBytes {
		return
	}

	// Sort by timestamp (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].timestamp.Before(entries[j].timestamp)
	})

	// Keep track of the most recent entry for each feed
	latestEntries := make(map[string]cacheEntry) // key: networkID:prefixed
	for _, entry := range entries {
		key := fmt.Sprintf("%d:%s", entry.networkID, entry.prefixed)
		if existing, exists := latestEntries[key]; !exists || entry.timestamp.After(existing.timestamp) {
			latestEntries[key] = entry
		}
	}

	// Remove entries starting from oldest until we're under the limit
	// But always keep the latest entry for each feed
	removedSize := int64(0)
	for _, entry := range entries {
		if totalSize-removedSize <= MaxCacheSizeBytes {
			break
		}

		key := fmt.Sprintf("%d:%s", entry.networkID, entry.prefixed)
		latest := latestEntries[key]

		// Don't remove if this is the latest entry for this feed
		if entry.timestamp.Equal(latest.timestamp) || entry.timestamp.After(latest.timestamp) {
			continue
		}

		// Remove this entry
		if networkData, exists := pc.data[entry.networkID]; exists {
			delete(networkData, entry.prefixed)
			removedSize += entry.entrySize

			// If network has no more entries, clean up
			if len(networkData) == 0 {
				delete(pc.data, entry.networkID)
			}
		}
	}

	if removedSize > 0 {
		log.Printf("Pruned cache: removed ~%d bytes, current size ~%d bytes", removedSize, totalSize-removedSize)
	}
}

// Legacy methods for backward compatibility (deprecated)
// These will be removed in a future version

// AddFeedLegacy adds a price feed using the old format (assumes Chainlink)
func (pc *PriceCache) AddFeedLegacy(networkID uint64, feedAddress string) {
	pc.AddFeed(networkID, feedAddress, types.SourceChainlink)
}

// GetPriceLegacy retrieves price using the old format (assumes Chainlink)
func (pc *PriceCache) GetPriceLegacy(networkID uint64, feedAddress string) (*PriceData, error) {
	priceInfo, err := pc.GetPrice(networkID, feedAddress, types.SourceChainlink)
	if err != nil {
		return nil, err
	}

	// Convert ChainlinkPrice to PriceData for backward compatibility
	if clPrice, ok := priceInfo.(*types.ChainlinkPrice); ok {
		return &PriceData{
			RoundID:         clPrice.RoundID,
			Answer:          clPrice.Answer,
			Exponent:        clPrice.Exponent,
			StartedAt:       clPrice.StartedAt,
			UpdatedAt:       clPrice.UpdatedAt,
			AnsweredInRound: clPrice.AnsweredInRound,
			Timestamp:       clPrice.Timestamp,
			NetworkID:       clPrice.NetworkID,
		}, nil
	}

	return nil, fmt.Errorf("price info is not Chainlink data")
}

// GetAllPricesLegacy retrieves all prices using the old format (assumes Chainlink)
func (pc *PriceCache) GetAllPricesLegacy(networkID uint64) map[string]*PriceData {
	chainlinkPrices := pc.GetAllPricesBySource(networkID, types.SourceChainlink)
	result := make(map[string]*PriceData)

	for identifier, priceInfo := range chainlinkPrices {
		if clPrice, ok := priceInfo.(*types.ChainlinkPrice); ok {
			result[identifier] = &PriceData{
				RoundID:         clPrice.RoundID,
				Answer:          clPrice.Answer,
				Exponent:        clPrice.Exponent,
				StartedAt:       clPrice.StartedAt,
				UpdatedAt:       clPrice.UpdatedAt,
				AnsweredInRound: clPrice.AnsweredInRound,
				Timestamp:       clPrice.Timestamp,
				NetworkID:       clPrice.NetworkID,
			}
		}
	}

	return result
}

// UpdatePriceLegacy updates price using the old format (assumes Chainlink)
func (pc *PriceCache) UpdatePriceLegacy(networkID uint64, feedAddress string, priceData *PriceData) {
	exponent := priceData.Exponent
	if exponent == 0 {
		// Default to -8 if not set (for backward compatibility)
		exponent = -8
	}
	clPrice := &types.ChainlinkPrice{
		RoundID:         priceData.RoundID,
		Answer:          priceData.Answer,
		Exponent:        exponent,
		StartedAt:       priceData.StartedAt,
		UpdatedAt:       priceData.UpdatedAt,
		AnsweredInRound: priceData.AnsweredInRound,
		Timestamp:       priceData.Timestamp,
		NetworkID:       priceData.NetworkID,
		FeedAddress:     feedAddress,
	}
	pc.UpdatePrice(networkID, feedAddress, types.SourceChainlink, clPrice)
}

// PriceCacheManager manages the local price cache with persistence
type PriceCacheManager struct {
	cache     *PriceCache
	mu        sync.RWMutex
	lastSaved time.Time
}

// NewPriceCacheManager creates a new price cache manager
func NewPriceCacheManager() *PriceCacheManager {
	return &PriceCacheManager{
		cache:     NewPriceCache(),
		lastSaved: time.Now(),
	}
}

// UpdatePrice updates a price in the cache
func (pcm *PriceCacheManager) UpdatePrice(networkID uint64, identifier string, source types.PriceSource, priceInfo types.PriceInfo) {
	pcm.cache.UpdatePrice(networkID, identifier, source, priceInfo)
}

// GetPrice retrieves a price from the cache
func (pcm *PriceCacheManager) GetPrice(networkID uint64, identifier string, source types.PriceSource) (types.PriceInfo, error) {
	return pcm.cache.GetPrice(networkID, identifier, source)
}

// GetAllPrices retrieves all prices for a network
func (pcm *PriceCacheManager) GetAllPrices(networkID uint64) map[string]types.PriceInfo {
	return pcm.cache.GetAllPrices(networkID)
}

// GetAllPricesBySource retrieves all prices for a network filtered by source
func (pcm *PriceCacheManager) GetAllPricesBySource(networkID uint64, source types.PriceSource) map[string]types.PriceInfo {
	return pcm.cache.GetAllPricesBySource(networkID, source)
}

// AddFeed adds a price feed to monitor
func (pcm *PriceCacheManager) AddFeed(networkID uint64, identifier string, source types.PriceSource) {
	pcm.cache.AddFeed(networkID, identifier, source)
}

// UpdateLastSaved updates the last saved timestamp
func (pcm *PriceCacheManager) UpdateLastSaved() {
	pcm.mu.Lock()
	defer pcm.mu.Unlock()
	pcm.lastSaved = time.Now()
}

// GetLastSaved returns the last saved timestamp
func (pcm *PriceCacheManager) GetLastSaved() time.Time {
	pcm.mu.RLock()
	defer pcm.mu.RUnlock()
	return pcm.lastSaved
}

// GetCacheSize returns the estimated size of the cache in bytes
func (pcm *PriceCacheManager) GetCacheSize() int64 {
	return pcm.cache.estimateSize()
}

// PruneCache manually triggers cache pruning if size exceeds limit
func (pcm *PriceCacheManager) PruneCache() {
	pcm.cache.prune()
}

// GetCache returns the underlying PriceCache (for advanced use cases)
func (pcm *PriceCacheManager) GetCache() *PriceCache {
	return pcm.cache
}

// PrintStatus prints the current cache status
func (pcm *PriceCacheManager) PrintStatus() {
	lastSaved := pcm.GetLastSaved()
	cacheSize := pcm.GetCacheSize()
	cacheSizeMB := float64(cacheSize) / (1024 * 1024)
	maxSizeMB := float64(10) // MaxCacheSizeBytes / (1024 * 1024)

	fmt.Printf("📊 CACHE STATUS\n")
	fmt.Printf("   Last Saved: %s\n", lastSaved.Format("2006-01-02 15:04:05"))
	fmt.Printf("   Time Since Last Save: %v\n", time.Since(lastSaved))
	fmt.Printf("   Cache Size: %.2f MB / %.2f MB (%.1f%%)\n", cacheSizeMB, maxSizeMB, (cacheSizeMB/maxSizeMB)*100)

	// Count total feeds across all networks
	totalFeeds := 0
	pcm.cache.mu.RLock()
	for _, feeds := range pcm.cache.feeds {
		totalFeeds += len(feeds)
	}
	pcm.cache.mu.RUnlock()

	fmt.Printf("   Total Monitored Feeds: %d\n", totalFeeds)
	fmt.Println("   " + strings.Repeat("-", 50))
}
