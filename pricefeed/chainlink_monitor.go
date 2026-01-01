package pricefeed

import (
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/morpheum-labs/pricefeeding/chainlink"
	"github.com/morpheum-labs/pricefeeding/rpcscan"
	"github.com/morpheum-labs/pricefeeding/types"
)

// CLPriceMonitor handles monitoring of Chainlink price feeds
type CLPriceMonitor struct {
	cacheManager  *PriceCacheManager
	clients       map[uint64]*ethclient.Client
	mu            sync.RWMutex
	stopChan      chan struct{}
	interval      time.Duration
	networkConfig *rpcscan.NetworkConfiguration // Network configuration for RPC switching
	feedSymbols   map[uint64]map[string]string  // networkID -> feedAddress -> symbol mapping
	immediateMode bool                          // If true, prints prices immediately when received
}

// NewCLPriceMonitor creates a new Chainlink price monitor
// cacheManager: the price cache manager to use (required)
// interval: how often to update prices
// immediateMode: if true, prints prices immediately when received
func NewCLPriceMonitor(cacheManager *PriceCacheManager, interval time.Duration, immediateMode bool) *CLPriceMonitor {
	return &CLPriceMonitor{
		cacheManager:  cacheManager,
		clients:       make(map[uint64]*ethclient.Client),
		stopChan:      make(chan struct{}),
		interval:      interval,
		feedSymbols:   make(map[uint64]map[string]string),
		immediateMode: immediateMode,
	}
}

// AddClient adds an Ethereum client for a specific network
func (pm *CLPriceMonitor) AddClient(networkID uint64, client *ethclient.Client) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.clients[networkID] = client
	log.Printf("Added client for network %d", networkID)
}

// UpdateClient updates an Ethereum client for a specific network (used after RPC switching)
func (pm *CLPriceMonitor) UpdateClient(networkID uint64, client *ethclient.Client) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.clients[networkID] = client
	log.Printf("Updated client for network %d after RPC switch", networkID)
}

// AddPriceFeed adds a price feed to monitor
func (pm *CLPriceMonitor) AddPriceFeed(networkID uint64, feedAddress string) {
	pm.cacheManager.AddFeed(networkID, feedAddress, types.SourceChainlink)
}

// AddPriceFeedWithSymbol adds a price feed to monitor with a symbol for better display
func (pm *CLPriceMonitor) AddPriceFeedWithSymbol(networkID uint64, feedAddress string, symbol string) {
	pm.cacheManager.AddFeed(networkID, feedAddress, types.SourceChainlink)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.feedSymbols[networkID] == nil {
		pm.feedSymbols[networkID] = make(map[string]string)
	}
	pm.feedSymbols[networkID][feedAddress] = symbol
	log.Printf("Added Chainlink price feed: %s (%s) for network %d", symbol, feedAddress, networkID)
}

// GetPrice retrieves the latest price for a specific feed
func (pm *CLPriceMonitor) GetPrice(networkID uint64, feedAddress string) (*types.ChainlinkPrice, error) {
	priceInfo, err := pm.cacheManager.GetPrice(networkID, feedAddress, types.SourceChainlink)
	if err != nil {
		return nil, err
	}
	if clPrice, ok := priceInfo.(*types.ChainlinkPrice); ok {
		return clPrice, nil
	}
	return nil, fmt.Errorf("price info is not Chainlink data")
}

// GetAllPrices retrieves all prices for a specific network (Chainlink only)
func (pm *CLPriceMonitor) GetAllPrices(networkID uint64) map[string]*types.ChainlinkPrice {
	allPrices := pm.cacheManager.GetAllPricesBySource(networkID, types.SourceChainlink)
	result := make(map[string]*types.ChainlinkPrice)
	for identifier, priceInfo := range allPrices {
		if clPrice, ok := priceInfo.(*types.ChainlinkPrice); ok {
			result[identifier] = clPrice
		}
	}
	return result
}

// fetchPriceData fetches price data from a specific feed
func (pm *CLPriceMonitor) fetchPriceData(networkID uint64, feedAddress string) (*types.ChainlinkPrice, error) {
	pm.mu.RLock()
	client, exists := pm.clients[networkID]
	networkConfig := pm.networkConfig
	pm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no client available for network %d", networkID)
	}

	// Create RPC switcher adapter if network config is available
	var rpcSwitcher chainlink.RPCSwitcher
	if networkConfig != nil {
		rpcSwitcher = &rpcSwitcherAdapter{
			networkConfig: networkConfig,
			priceMonitor:  pm,
			networkID:     networkID,
		}
	}

	// Use the chainlink package to fetch price data
	opts := chainlink.FetchPriceDataOptions{
		NetworkID:   networkID,
		FeedAddress: feedAddress,
		Client:      client,
		RPCSwitcher: rpcSwitcher,
		MaxRetries:  1,
		RetryDelay:  2 * time.Second,
	}

	return chainlink.FetchPriceData(opts)
}

// updateAllPrices updates all monitored price feeds efficiently
func (pm *CLPriceMonitor) updateAllPrices() {
	pm.mu.RLock()
	clients := make(map[uint64]*ethclient.Client)
	for networkID, client := range pm.clients {
		clients[networkID] = client
	}
	pm.mu.RUnlock()

	cache := pm.cacheManager.GetCache()
	cache.mu.RLock()
	feeds := make(map[uint64][]string)
	for networkID, feedList := range cache.feeds {
		feeds[networkID] = make([]string, len(feedList))
		copy(feeds[networkID], feedList)
	}
	cache.mu.RUnlock()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Limit concurrent requests

	for networkID, feedList := range feeds {
		if _, exists := clients[networkID]; !exists {
			continue // Skip if no client available
		}

		for _, prefixedFeed := range feedList {
			wg.Add(1)
			go func(netID uint64, prefixed string) {
				defer wg.Done()

				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				// Extract feed address from prefixed identifier (e.g., "chainlink:0xaddr" -> "0xaddr")
				feedAddress := strings.TrimPrefix(prefixed, string(types.SourceChainlink)+":")

				priceData, err := pm.fetchPriceData(netID, feedAddress)
				if err != nil {
					log.Printf("Failed to fetch price data for feed %s on network %d: %v", feedAddress, netID, err)
					return
				}

				pm.cacheManager.UpdatePrice(netID, feedAddress, types.SourceChainlink, priceData)

				// Print immediately if in immediate mode
				if pm.immediateMode {
					pm.printPriceUpdate(netID, feedAddress, priceData)
				} else {
					log.Printf("Updated price for feed %s on network %d: %s", feedAddress, netID, priceData.Answer.String())
				}
			}(networkID, prefixedFeed)
		}
	}

	wg.Wait()
}

// printPriceUpdate prints price update information in a formatted way
func (pm *CLPriceMonitor) printPriceUpdate(networkID uint64, feedAddress string, priceData *types.ChainlinkPrice) {
	// Get symbol if available
	pm.mu.RLock()
	symbol := "Unknown"
	if networkSymbols, exists := pm.feedSymbols[networkID]; exists {
		if feedSymbol, exists := networkSymbols[feedAddress]; exists {
			symbol = feedSymbol
		}
	}
	pm.mu.RUnlock()

	// Convert price to human readable format using Exponent
	priceFloat := new(big.Float).SetInt(priceData.Answer)
	// Calculate divisor from Exponent: 10^(-Exponent)
	// If Exponent is -8, divisor is 10^8
	exponent := priceData.Exponent
	if exponent == 0 {
		// Default to -8 if not set
		exponent = -8
	}
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-exponent)), nil))
	priceFloat.Quo(priceFloat, divisor)

	fmt.Printf("🔄 CHAINLINK PRICE UPDATE [%s]\n", time.Now().Format("15:04:05"))
	fmt.Printf("   Symbol: %s\n", symbol)
	fmt.Printf("   Network ID: %d\n", networkID)
	fmt.Printf("   Feed Address: %s\n", feedAddress)
	fmt.Printf("   Price: $%s\n", priceFloat.Text('f', 8))
	fmt.Printf("   Round ID: %s\n", priceData.RoundID.String())
	fmt.Printf("   Started At: %s\n", time.Unix(priceData.StartedAt.Int64(), 0).Format("15:04:05"))
	fmt.Printf("   Updated At: %s\n", time.Unix(priceData.UpdatedAt.Int64(), 0).Format("15:04:05"))
	fmt.Printf("   Answered In Round: %s\n", priceData.AnsweredInRound.String())
	fmt.Printf("   Timestamp: %s\n", priceData.Timestamp.Format("15:04:05"))
	fmt.Println("   " + strings.Repeat("-", 50))
}

// Start begins monitoring price feeds
func (pm *CLPriceMonitor) Start() {
	log.Printf("Starting Chainlink price monitor with %v interval (immediate mode: %v)", pm.interval, pm.immediateMode)

	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	// Initial update
	pm.updateAllPrices()

	for {
		select {
		case <-pm.stopChan:
			log.Println("Stopping Chainlink price monitor")
			return
		case <-ticker.C:
			pm.updateAllPrices()
		}
	}
}

// Stop stops the price monitor
func (pm *CLPriceMonitor) Stop() {
	close(pm.stopChan)
}

// GetCacheManager returns the price cache manager (for external access)
func (pm *CLPriceMonitor) GetCacheManager() *PriceCacheManager {
	return pm.cacheManager
}

// GetCache returns the underlying price cache (for backward compatibility)
func (pm *CLPriceMonitor) GetCache() *PriceCache {
	return pm.cacheManager.GetCache()
}

// SetNetworkConfig sets the network configuration for RPC switching
func (pm *CLPriceMonitor) SetNetworkConfig(networkConfig *rpcscan.NetworkConfiguration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.networkConfig = networkConfig
}

// SetImmediateMode sets whether to print prices immediately
func (pm *CLPriceMonitor) SetImmediateMode(immediate bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.immediateMode = immediate
	log.Printf("Chainlink price monitor immediate mode set to: %v", immediate)
}

// PrintStatus prints the current cache status and monitored feeds
func (pm *CLPriceMonitor) PrintStatus() {
	pm.mu.RLock()
	clientCount := len(pm.clients)
	pm.mu.RUnlock()

	cache := pm.cacheManager.GetCache()
	cache.mu.RLock()
	feedCount := 0
	for _, feeds := range cache.feeds {
		feedCount += len(feeds)
	}
	feedsCopy := make(map[uint64][]string)
	for networkID, feedList := range cache.feeds {
		feedsCopy[networkID] = make([]string, len(feedList))
		copy(feedsCopy[networkID], feedList)
	}
	cache.mu.RUnlock()

	fmt.Printf("📊 CHAINLINK CACHE STATUS\n")
	fmt.Printf("   Active Networks: %d\n", clientCount)
	fmt.Printf("   Monitored Feeds: %d\n", feedCount)
	fmt.Printf("   Immediate Mode: %v\n", pm.immediateMode)
	fmt.Printf("   Update Interval: %v\n", pm.interval)

	// Show feeds by network
	for networkID, feeds := range feedsCopy {
		if len(feeds) > 0 {
			fmt.Printf("   Network %d: %d feeds\n", networkID, len(feeds))
			pm.mu.RLock()
			if networkSymbols, exists := pm.feedSymbols[networkID]; exists {
				for _, prefixedFeed := range feeds {
					// Extract feed address from prefixed identifier
					feedAddress := strings.TrimPrefix(prefixedFeed, string(types.SourceChainlink)+":")
					if symbol, exists := networkSymbols[feedAddress]; exists {
						fmt.Printf("     - %s (%s)\n", symbol, feedAddress)
					} else {
						fmt.Printf("     - Unknown (%s)\n", feedAddress)
					}
				}
			} else {
				for _, prefixedFeed := range feeds {
					// Extract feed address from prefixed identifier
					feedAddress := strings.TrimPrefix(prefixedFeed, string(types.SourceChainlink)+":")
					fmt.Printf("     - Unknown (%s)\n", feedAddress)
				}
			}
			pm.mu.RUnlock()
		}
	}
	fmt.Println("   " + strings.Repeat("-", 50))
}

// GetFeedSymbol returns the symbol for a feed address on a specific network
func (pm *CLPriceMonitor) GetFeedSymbol(networkID uint64, feedAddress string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if networkSymbols, exists := pm.feedSymbols[networkID]; exists {
		if symbol, exists := networkSymbols[feedAddress]; exists {
			return symbol
		}
	}
	return "Unknown"
}

// rpcSwitcherAdapter adapts NetworkConfiguration to chainlink.RPCSwitcher interface
type rpcSwitcherAdapter struct {
	networkConfig *rpcscan.NetworkConfiguration
	priceMonitor  *CLPriceMonitor
	networkID     uint64
}

// SwitchRPCEndpointImmediately switches to a different RPC endpoint
func (r *rpcSwitcherAdapter) SwitchRPCEndpointImmediately(networkID uint64) error {
	return r.networkConfig.SwitchRPCEndpointImmediately(networkID)
}

// GetBestClient returns the best available client for the network
func (r *rpcSwitcherAdapter) GetBestClient(networkID uint64) (*ethclient.Client, error) {
	ethClient, err := r.networkConfig.GetBestClient(networkID)
	if err != nil {
		return nil, err
	}

	// Get the underlying ethclient.Client
	newClient := ethClient.GetClient()

	// Update the price monitor's client map
	r.priceMonitor.UpdateClient(networkID, newClient)

	return newClient, nil
}
