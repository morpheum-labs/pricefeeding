package pricefeed

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

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
func (pcm *PriceCacheManager) UpdatePrice(networkID uint64, feedAddress string, priceData *PriceData) {
	pcm.cache.UpdatePrice(networkID, feedAddress, priceData)
}

// GetPrice retrieves a price from the cache
func (pcm *PriceCacheManager) GetPrice(networkID uint64, feedAddress string) (*PriceData, error) {
	return pcm.cache.GetPrice(networkID, feedAddress)
}

// GetAllPrices retrieves all prices for a network
func (pcm *PriceCacheManager) GetAllPrices(networkID uint64) map[string]*PriceData {
	return pcm.cache.GetAllPrices(networkID)
}

// AddFeed adds a price feed to monitor
func (pcm *PriceCacheManager) AddFeed(networkID uint64, feedAddress string) {
	pcm.cache.AddFeed(networkID, feedAddress)
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

// PrintStatus prints the current cache status
func (pcm *PriceCacheManager) PrintStatus() {
	lastSaved := pcm.GetLastSaved()
	fmt.Printf("📊 CACHE STATUS\n")
	fmt.Printf("   Last Saved: %s\n", lastSaved.Format("2006-01-02 15:04:05"))
	fmt.Printf("   Time Since Last Save: %v\n", time.Since(lastSaved))

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
