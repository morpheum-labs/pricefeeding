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

// PriceData represents price information from Chainlink (deprecated, use ChainlinkPrice)
// Kept for backward compatibility during migration
type PriceData struct {
	RoundID         *big.Int
	Answer          *big.Int
	StartedAt       *big.Int
	UpdatedAt       *big.Int
	AnsweredInRound *big.Int
	Timestamp       time.Time
	NetworkID       uint64
}

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
	clPrice := &types.ChainlinkPrice{
		RoundID:         priceData.RoundID,
		Answer:          priceData.Answer,
		StartedAt:       priceData.StartedAt,
		UpdatedAt:       priceData.UpdatedAt,
		AnsweredInRound: priceData.AnsweredInRound,
		Timestamp:       priceData.Timestamp,
		NetworkID:       priceData.NetworkID,
		FeedAddress:     feedAddress,
	}
	pc.UpdatePrice(networkID, feedAddress, types.SourceChainlink, clPrice)
}

// PriceMonitor handles monitoring of Chainlink price feeds
type PriceMonitor struct {
	cache         *PriceCache
	clients       map[uint64]*ethclient.Client
	mu            sync.RWMutex
	stopChan      chan struct{}
	interval      time.Duration
	networkConfig *rpcscan.NetworkConfiguration // Network configuration for RPC switching
	feedSymbols   map[uint64]map[string]string  // networkID -> feedAddress -> symbol mapping
	immediateMode bool                          // If true, prints prices immediately when received
}

// NewPriceMonitor creates a new price monitor
func NewPriceMonitor(interval time.Duration) *PriceMonitor {
	return &PriceMonitor{
		cache:         NewPriceCache(),
		clients:       make(map[uint64]*ethclient.Client),
		stopChan:      make(chan struct{}),
		interval:      interval,
		feedSymbols:   make(map[uint64]map[string]string),
		immediateMode: false, // Default to false, can be enabled later
	}
}

// NewPriceMonitorWithImmediateMode creates a new price monitor with immediate mode setting
func NewPriceMonitorWithImmediateMode(interval time.Duration, immediateMode bool) *PriceMonitor {
	return &PriceMonitor{
		cache:         NewPriceCache(),
		clients:       make(map[uint64]*ethclient.Client),
		stopChan:      make(chan struct{}),
		interval:      interval,
		feedSymbols:   make(map[uint64]map[string]string),
		immediateMode: immediateMode,
	}
}

// AddClient adds an Ethereum client for a specific network
func (pm *PriceMonitor) AddClient(networkID uint64, client *ethclient.Client) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.clients[networkID] = client
	log.Printf("Added client for network %d", networkID)
}

// UpdateClient updates an Ethereum client for a specific network (used after RPC switching)
func (pm *PriceMonitor) UpdateClient(networkID uint64, client *ethclient.Client) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.clients[networkID] = client
	log.Printf("Updated client for network %d after RPC switch", networkID)
}

// AddPriceFeed adds a price feed to monitor
func (pm *PriceMonitor) AddPriceFeed(networkID uint64, feedAddress string) {
	pm.cache.AddFeed(networkID, feedAddress, types.SourceChainlink)
}

// AddPriceFeedWithSymbol adds a price feed to monitor with a symbol for better display
func (pm *PriceMonitor) AddPriceFeedWithSymbol(networkID uint64, feedAddress string, symbol string) {
	pm.cache.AddFeed(networkID, feedAddress, types.SourceChainlink)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.feedSymbols[networkID] == nil {
		pm.feedSymbols[networkID] = make(map[string]string)
	}
	pm.feedSymbols[networkID][feedAddress] = symbol
	log.Printf("Added Chainlink price feed: %s (%s) for network %d", symbol, feedAddress, networkID)
}

// GetPrice retrieves the latest price for a specific feed
func (pm *PriceMonitor) GetPrice(networkID uint64, feedAddress string) (*types.ChainlinkPrice, error) {
	priceInfo, err := pm.cache.GetPrice(networkID, feedAddress, types.SourceChainlink)
	if err != nil {
		return nil, err
	}
	if clPrice, ok := priceInfo.(*types.ChainlinkPrice); ok {
		return clPrice, nil
	}
	return nil, fmt.Errorf("price info is not Chainlink data")
}

// GetAllPrices retrieves all prices for a specific network (Chainlink only)
func (pm *PriceMonitor) GetAllPrices(networkID uint64) map[string]*types.ChainlinkPrice {
	allPrices := pm.cache.GetAllPricesBySource(networkID, types.SourceChainlink)
	result := make(map[string]*types.ChainlinkPrice)
	for identifier, priceInfo := range allPrices {
		if clPrice, ok := priceInfo.(*types.ChainlinkPrice); ok {
			result[identifier] = clPrice
		}
	}
	return result
}

// fetchPriceData fetches price data from a specific feed
func (pm *PriceMonitor) fetchPriceData(networkID uint64, feedAddress string) (*types.ChainlinkPrice, error) {
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
func (pm *PriceMonitor) updateAllPrices() {
	pm.mu.RLock()
	clients := make(map[uint64]*ethclient.Client)
	for networkID, client := range pm.clients {
		clients[networkID] = client
	}
	pm.mu.RUnlock()

	pm.cache.mu.RLock()
	feeds := make(map[uint64][]string)
	for networkID, feedList := range pm.cache.feeds {
		feeds[networkID] = make([]string, len(feedList))
		copy(feeds[networkID], feedList)
	}
	pm.cache.mu.RUnlock()

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

				pm.cache.UpdatePrice(netID, feedAddress, types.SourceChainlink, priceData)

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
func (pm *PriceMonitor) printPriceUpdate(networkID uint64, feedAddress string, priceData *types.ChainlinkPrice) {
	// Get symbol if available
	pm.mu.RLock()
	symbol := "Unknown"
	if networkSymbols, exists := pm.feedSymbols[networkID]; exists {
		if feedSymbol, exists := networkSymbols[feedAddress]; exists {
			symbol = feedSymbol
		}
	}
	pm.mu.RUnlock()

	// Convert price to human readable format (assuming 8 decimals)
	// Convert to float with proper precision
	priceFloat := new(big.Float).SetInt(priceData.Answer)
	divisor := new(big.Float).SetInt64(1e8) // 10^8
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
func (pm *PriceMonitor) Start() {
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
func (pm *PriceMonitor) Stop() {
	close(pm.stopChan)
}

// GetCache returns the price cache (for external access)
func (pm *PriceMonitor) GetCache() *PriceCache {
	return pm.cache
}

// SetNetworkConfig sets the network configuration for RPC switching
func (pm *PriceMonitor) SetNetworkConfig(networkConfig *rpcscan.NetworkConfiguration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.networkConfig = networkConfig
}

// SetImmediateMode sets whether to print prices immediately
func (pm *PriceMonitor) SetImmediateMode(immediate bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.immediateMode = immediate
	log.Printf("Chainlink price monitor immediate mode set to: %v", immediate)
}

// PrintStatus prints the current cache status and monitored feeds
func (pm *PriceMonitor) PrintStatus() {
	pm.mu.RLock()
	clientCount := len(pm.clients)
	feedCount := 0
	for _, feeds := range pm.cache.feeds {
		feedCount += len(feeds)
	}
	pm.mu.RUnlock()

	fmt.Printf("📊 CHAINLINK CACHE STATUS\n")
	fmt.Printf("   Active Networks: %d\n", clientCount)
	fmt.Printf("   Monitored Feeds: %d\n", feedCount)
	fmt.Printf("   Immediate Mode: %v\n", pm.immediateMode)
	fmt.Printf("   Update Interval: %v\n", pm.interval)

	// Show feeds by network
	for networkID, feeds := range pm.cache.feeds {
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
func (pm *PriceMonitor) GetFeedSymbol(networkID uint64, feedAddress string) string {
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
	priceMonitor  *PriceMonitor
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
