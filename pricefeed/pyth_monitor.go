package pricefeed

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/morpheum/chainlink-price-feed-golang/pyth"
)

// PythPriceData represents price information from Pyth
type PythPriceData struct {
	ID            string    `json:"id"`
	Symbol        string    `json:"symbol,omitempty"`
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

// PythPriceMonitor handles monitoring of Pyth price feeds
type PythPriceMonitor struct {
	cacheManager  *PriceCacheManager
	client        *pyth.HermesClient
	mu            sync.RWMutex
	stopChan      chan struct{}
	interval      time.Duration
	priceFeeds    map[string]string // priceID -> symbol mapping
	immediateMode bool              // If true, prints prices immediately when received
}

// NewPythPriceMonitor creates a new Pyth price monitor
func NewPythPriceMonitor(endpoint string, interval time.Duration, immediateMode bool) *PythPriceMonitor {
	config := &pyth.HermesClientConfig{
		Timeout:     &[]pyth.DurationInMs{5000}[0], // 5 second timeout
		HTTPRetries: &[]int{3}[0],                  // 3 retries
	}

	client := pyth.NewHermesClient(endpoint, config)

	return &PythPriceMonitor{
		cacheManager:  NewPriceCacheManager(),
		client:        client,
		stopChan:      make(chan struct{}),
		interval:      interval,
		priceFeeds:    make(map[string]string),
		immediateMode: immediateMode,
	}
}

// AddPriceFeed adds a Pyth price feed to monitor
func (ppm *PythPriceMonitor) AddPriceFeed(priceID, symbol string) {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()

	ppm.priceFeeds[priceID] = symbol
	log.Printf("Added Pyth price feed: %s (%s)", symbol, priceID)
}

// GetPrice retrieves the latest price for a specific feed
func (ppm *PythPriceMonitor) GetPrice(priceID string) (*PythPriceData, error) {
	// Convert priceID to a networkID (using 0 as default for Pyth)
	// In a real implementation, you might want to map priceIDs to specific networks
	networkID := uint64(0)

	// Try to get from cache first
	if priceData, err := ppm.cacheManager.GetPrice(networkID, priceID); err == nil {
		// Convert PriceData to PythPriceData
		return ppm.convertToPythPriceData(priceData, priceID), nil
	}

	return nil, fmt.Errorf("no price data for feed %s", priceID)
}

// GetAllPrices retrieves all prices for all monitored feeds
func (ppm *PythPriceMonitor) GetAllPrices() map[string]*PythPriceData {
	ppm.mu.RLock()
	defer ppm.mu.RUnlock()

	results := make(map[string]*PythPriceData)
	networkID := uint64(0) // Default network for Pyth

	allPrices := ppm.cacheManager.GetAllPrices(networkID)
	for priceID, priceData := range allPrices {
		results[priceID] = ppm.convertToPythPriceData(priceData, priceID)
	}

	return results
}

// fetchPriceData fetches price data from Pyth for all monitored feeds
func (ppm *PythPriceMonitor) fetchPriceData() error {
	ppm.mu.RLock()
	priceIDs := make([]pyth.HexString, 0, len(ppm.priceFeeds))
	for priceID := range ppm.priceFeeds {
		priceIDs = append(priceIDs, pyth.HexString(priceID))
	}
	ppm.mu.RUnlock()

	if len(priceIDs) == 0 {
		return fmt.Errorf("no price feeds to monitor")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get latest price updates with parsed data
	options := &pyth.GetLatestPriceUpdatesOptions{
		Parsed: &[]bool{true}[0], // Get parsed data
	}

	priceUpdate, err := ppm.client.GetLatestPriceUpdates(ctx, priceIDs, options)
	if err != nil {
		return fmt.Errorf("failed to get latest price updates: %v", err)
	}

	// Process each price feed
	for _, feed := range priceUpdate.Parsed {
		pythPriceData := ppm.convertPythFeedToPriceData(feed)

		// Update cache
		networkID := uint64(0) // Default network for Pyth
		ppm.cacheManager.UpdatePrice(networkID, feed.ID, ppm.convertToChainlinkPriceData(pythPriceData))

		// Update lastSaved timestamp in cache manager
		ppm.cacheManager.UpdateLastSaved()

		// Print immediately if in immediate mode
		if ppm.immediateMode {
			ppm.printPriceUpdate(pythPriceData)
		}
	}

	return nil
}

// convertPythFeedToPriceData converts a Pyth PriceFeed to our PythPriceData structure
func (ppm *PythPriceMonitor) convertPythFeedToPriceData(feed pyth.PriceFeed) *PythPriceData {
	// Convert price string to big.Int
	price, _ := new(big.Int).SetString(feed.Price.Price, 10)
	confidence, _ := new(big.Int).SetString(feed.Price.Conf, 10)

	pythPriceData := &PythPriceData{
		ID:          feed.ID,
		Price:       price,
		Confidence:  confidence,
		Exponent:    feed.Price.Expo,
		PublishTime: feed.Price.PublishTime,
		Slot:        feed.Metadata.Slot,
		Timestamp:   time.Now(),
		NetworkID:   uint64(0), // Default network for Pyth
	}

	// Add symbol if available
	ppm.mu.RLock()
	if symbol, exists := ppm.priceFeeds[feed.ID]; exists {
		pythPriceData.Symbol = symbol
	}
	ppm.mu.RUnlock()

	// Add EMA data if available
	if feed.Ema.Price != "" {
		ema, _ := new(big.Int).SetString(feed.Ema.Price, 10)
		emaConf, _ := new(big.Int).SetString(feed.Ema.Conf, 10)
		pythPriceData.EMA = ema
		pythPriceData.EMAConfidence = emaConf
	}

	return pythPriceData
}

// convertToChainlinkPriceData converts PythPriceData to PriceData for cache compatibility
func (ppm *PythPriceMonitor) convertToChainlinkPriceData(pythData *PythPriceData) *PriceData {
	return &PriceData{
		RoundID:         big.NewInt(pythData.Slot),
		Answer:          pythData.Price,
		StartedAt:       big.NewInt(pythData.PublishTime),
		UpdatedAt:       big.NewInt(time.Now().Unix()),
		AnsweredInRound: big.NewInt(pythData.Slot),
		Timestamp:       pythData.Timestamp,
		NetworkID:       pythData.NetworkID,
	}
}

// convertToPythPriceData converts PriceData back to PythPriceData
func (ppm *PythPriceMonitor) convertToPythPriceData(priceData *PriceData, priceID string) *PythPriceData {
	pythData := &PythPriceData{
		ID:          priceID,
		Price:       priceData.Answer,
		Confidence:  big.NewInt(0), // Default confidence
		Exponent:    0,             // Default exponent
		PublishTime: priceData.StartedAt.Int64(),
		Slot:        priceData.RoundID.Int64(),
		Timestamp:   priceData.Timestamp,
		NetworkID:   priceData.NetworkID,
	}

	// Add symbol if available
	ppm.mu.RLock()
	if symbol, exists := ppm.priceFeeds[priceID]; exists {
		pythData.Symbol = symbol
	}
	ppm.mu.RUnlock()

	return pythData
}

// printPriceUpdate prints price update information
func (ppm *PythPriceMonitor) printPriceUpdate(priceData *PythPriceData) {
	// Calculate actual price from price and exponent
	actualPrice := new(big.Float).SetInt(priceData.Price)

	// Handle negative exponents properly
	var exponent *big.Float
	if priceData.Exponent < 0 {
		// For negative exponents, we need to divide by 10^|exponent|
		exponent = new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-priceData.Exponent)), nil))
		actualPrice.Quo(actualPrice, exponent)
	} else {
		// For positive exponents, we need to multiply by 10^exponent
		exponent = new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(priceData.Exponent)), nil))
		actualPrice.Mul(actualPrice, exponent)
	}

	// Calculate confidence in the same way
	actualConfidence := new(big.Float).SetInt(priceData.Confidence)
	if priceData.Exponent < 0 {
		actualConfidence.Quo(actualConfidence, exponent)
	} else {
		actualConfidence.Mul(actualConfidence, exponent)
	}

	fmt.Printf("🔄 PYTH PRICE UPDATE [%s]\n", time.Now().Format("15:04:05"))
	fmt.Printf("   Symbol: %s\n", priceData.Symbol)
	fmt.Printf("   Price ID: %s\n", priceData.ID)
	fmt.Printf("   Price: %s\n", actualPrice.Text('f', 8))
	fmt.Printf("   Confidence: ±%s\n", actualConfidence.Text('f', 8))
	fmt.Printf("   Publish Time: %s\n", time.Unix(priceData.PublishTime, 0).Format("15:04:05"))
	fmt.Printf("   Slot: %d\n", priceData.Slot)

	if priceData.EMA != nil {
		actualEMA := new(big.Float).SetInt(priceData.EMA)
		actualEMA.Quo(actualEMA, exponent)
		fmt.Printf("   EMA: %s\n", actualEMA.Text('f', 8))
	}

	fmt.Printf("   Last Saved: %s\n", ppm.cacheManager.GetLastSaved().Format("15:04:05"))
	fmt.Println("   " + strings.Repeat("-", 50))
}

// Start begins monitoring Pyth price feeds
func (ppm *PythPriceMonitor) Start() {
	log.Printf("Starting Pyth price monitor with %v interval (immediate mode: %v)", ppm.interval, ppm.immediateMode)

	ticker := time.NewTicker(ppm.interval)
	defer ticker.Stop()

	// Initial update
	if err := ppm.fetchPriceData(); err != nil {
		log.Printf("Initial price fetch failed: %v", err)
	}

	for {
		select {
		case <-ppm.stopChan:
			log.Println("Stopping Pyth price monitor")
			return
		case <-ticker.C:
			if err := ppm.fetchPriceData(); err != nil {
				log.Printf("Failed to fetch price data: %v", err)
			}
		}
	}
}

// Stop stops the Pyth price monitor
func (ppm *PythPriceMonitor) Stop() {
	close(ppm.stopChan)
}

// GetCacheManager returns the price cache manager
func (ppm *PythPriceMonitor) GetCacheManager() *PriceCacheManager {
	return ppm.cacheManager
}

// PrintLastSavedStatus prints the current lastSaved status
func (ppm *PythPriceMonitor) PrintLastSavedStatus() {
	ppm.mu.RLock()
	feedCount := len(ppm.priceFeeds)
	ppm.mu.RUnlock()

	fmt.Printf("📊 PYTH CACHE STATUS\n")
	fmt.Printf("   Last Saved: %s\n", ppm.cacheManager.GetLastSaved().Format("2006-01-02 15:04:05"))
	fmt.Printf("   Time Since Last Save: %v\n", time.Since(ppm.cacheManager.GetLastSaved()))
	fmt.Printf("   Monitored Feeds: %d\n", feedCount)
	fmt.Println("   " + strings.Repeat("-", 50))
}

// SetImmediateMode sets whether to print prices immediately
func (ppm *PythPriceMonitor) SetImmediateMode(immediate bool) {
	ppm.mu.Lock()
	defer ppm.mu.Unlock()
	ppm.immediateMode = immediate
	log.Printf("Pyth price monitor immediate mode set to: %v", immediate)
}
