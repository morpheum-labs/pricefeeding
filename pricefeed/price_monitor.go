package pricefeed

import (
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	aggregatorv3 "github.com/morpheum/chainlink-price-feed-golang/aggregatorv3"
)

// PriceData represents price information from Chainlink
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
type PriceCache struct {
	mu    sync.RWMutex
	data  map[uint64]map[string]*PriceData // networkID -> feedAddress -> priceData
	feeds map[uint64][]string              // networkID -> list of feed addresses
}

// NewPriceCache creates a new price cache
func NewPriceCache() *PriceCache {
	return &PriceCache{
		data:  make(map[uint64]map[string]*PriceData),
		feeds: make(map[uint64][]string),
	}
}

// AddFeed adds a price feed to monitor for a specific network
func (pc *PriceCache) AddFeed(networkID uint64, feedAddress string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.data[networkID] == nil {
		pc.data[networkID] = make(map[string]*PriceData)
		pc.feeds[networkID] = make([]string, 0)
	}

	// Check if feed already exists
	for _, existing := range pc.feeds[networkID] {
		if existing == feedAddress {
			return // Already exists
		}
	}

	pc.feeds[networkID] = append(pc.feeds[networkID], feedAddress)
	log.Printf("Added price feed %s for network %d", feedAddress, networkID)
}

// GetPrice retrieves the latest price for a specific feed
func (pc *PriceCache) GetPrice(networkID uint64, feedAddress string) (*PriceData, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.data[networkID] == nil {
		return nil, fmt.Errorf("no data for network %d", networkID)
	}

	priceData, exists := pc.data[networkID][feedAddress]
	if !exists {
		return nil, fmt.Errorf("no price data for feed %s on network %d", feedAddress, networkID)
	}

	return priceData, nil
}

// GetAllPrices retrieves all prices for a specific network
func (pc *PriceCache) GetAllPrices(networkID uint64) map[string]*PriceData {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.data[networkID] == nil {
		return make(map[string]*PriceData)
	}

	// Create a copy to avoid race conditions
	result := make(map[string]*PriceData)
	for address, priceData := range pc.data[networkID] {
		result[address] = priceData
	}

	return result
}

// UpdatePrice updates the price data for a specific feed
func (pc *PriceCache) UpdatePrice(networkID uint64, feedAddress string, priceData *PriceData) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.data[networkID] == nil {
		pc.data[networkID] = make(map[string]*PriceData)
	}

	pc.data[networkID][feedAddress] = priceData
}

// PriceMonitor handles monitoring of Chainlink price feeds
type PriceMonitor struct {
	cache         *PriceCache
	clients       map[uint64]*ethclient.Client
	mu            sync.RWMutex
	stopChan      chan struct{}
	interval      time.Duration
	networkConfig interface{} // Will hold reference to NetworkConfiguration for RPC switching
}

// NewPriceMonitor creates a new price monitor
func NewPriceMonitor(interval time.Duration) *PriceMonitor {
	return &PriceMonitor{
		cache:    NewPriceCache(),
		clients:  make(map[uint64]*ethclient.Client),
		stopChan: make(chan struct{}),
		interval: interval,
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
	pm.cache.AddFeed(networkID, feedAddress)
}

// GetPrice retrieves the latest price for a specific feed
func (pm *PriceMonitor) GetPrice(networkID uint64, feedAddress string) (*PriceData, error) {
	return pm.cache.GetPrice(networkID, feedAddress)
}

// GetAllPrices retrieves all prices for a specific network
func (pm *PriceMonitor) GetAllPrices(networkID uint64) map[string]*PriceData {
	return pm.cache.GetAllPrices(networkID)
}

// fetchPriceData fetches price data from a specific feed
func (pm *PriceMonitor) fetchPriceData(networkID uint64, feedAddress string) (*PriceData, error) {
	return pm.fetchPriceDataWithRetry(networkID, feedAddress, 1)
}

// fetchPriceDataWithRetry fetches price data with retry logic after RPC switching
func (pm *PriceMonitor) fetchPriceDataWithRetry(networkID uint64, feedAddress string, attempt int) (*PriceData, error) {
	pm.mu.RLock()
	client, exists := pm.clients[networkID]
	pm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no client available for network %d", networkID)
	}

	// Create the aggregator contract instance
	contractAddress := common.HexToAddress(feedAddress)
	aggregator, err := aggregatorv3.NewAggregatorV3Interface(contractAddress, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregator contract: %v", err)
	}

	// Get the latest round data
	roundData, err := aggregator.LatestRoundData(&bind.CallOpts{})
	if err != nil {
		// Check if this is the specific error code -32097 that requires immediate RPC switching
		if pm.isErrorCode32097(err) && attempt == 1 {
			log.Printf("Detected error code -32097 for network %d, triggering immediate RPC switch", networkID)
			// Trigger immediate RPC switching for this network
			pm.triggerImmediateRPCSwitch(networkID)

			// Wait a moment for the RPC switch to complete
			time.Sleep(2 * time.Second)

			// Retry with the new RPC endpoint
			log.Printf("Retrying price fetch for network %d with new RPC endpoint (attempt %d)", networkID, attempt+1)
			return pm.fetchPriceDataWithRetry(networkID, feedAddress, attempt+1)
		}
		return nil, fmt.Errorf("failed to get latest round data: %v", err)
	}

	// Convert to our PriceData structure
	priceData := &PriceData{
		RoundID:         roundData.RoundId,
		Answer:          roundData.Answer,
		StartedAt:       roundData.StartedAt,
		UpdatedAt:       roundData.UpdatedAt,
		AnsweredInRound: roundData.AnsweredInRound,
		Timestamp:       time.Now(),
		NetworkID:       networkID,
	}

	return priceData, nil
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

		for _, feedAddress := range feedList {
			wg.Add(1)
			go func(netID uint64, feedAddr string) {
				defer wg.Done()

				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				priceData, err := pm.fetchPriceData(netID, feedAddr)
				if err != nil {
					log.Printf("Failed to fetch price data for feed %s on network %d: %v", feedAddr, netID, err)
					return
				}

				pm.cache.UpdatePrice(netID, feedAddr, priceData)
				log.Printf("Updated price for feed %s on network %d: %s", feedAddr, netID, priceData.Answer.String())
			}(networkID, feedAddress)
		}
	}

	wg.Wait()
}

// Start begins monitoring price feeds
func (pm *PriceMonitor) Start() {
	log.Printf("Starting price monitor with %v interval", pm.interval)

	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	// Initial update
	pm.updateAllPrices()

	for {
		select {
		case <-pm.stopChan:
			log.Println("Stopping price monitor")
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
func (pm *PriceMonitor) SetNetworkConfig(networkConfig interface{}) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.networkConfig = networkConfig
}

// isErrorCode32097 checks if the error contains the specific error code -32097
func (pm *PriceMonitor) isErrorCode32097(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Check for various forms of the error code -32097
	return strings.Contains(errStr, "-32097") ||
		strings.Contains(errStr, "32097") ||
		strings.Contains(errStr, "execution reverted") ||
		strings.Contains(errStr, "revert")
}

// triggerImmediateRPCSwitch triggers immediate RPC switching for a specific network
func (pm *PriceMonitor) triggerImmediateRPCSwitch(networkID uint64) {
	log.Printf("Triggering immediate RPC switch for network %d", networkID)

	if pm.networkConfig != nil {
		// Type assert to get the actual network configuration
		if netconf, ok := pm.networkConfig.(interface {
			SwitchRPCEndpointImmediately(networkID uint64) error
			GetBestClient(networkID uint64) (interface{}, error)
		}); ok {
			// Attempt to switch RPC endpoint immediately
			err := netconf.SwitchRPCEndpointImmediately(networkID)
			if err != nil {
				log.Printf("Failed to switch RPC endpoint for network %d: %v", networkID, err)
				return
			}

			// Get the new client and update our local client map
			newClient, err := netconf.GetBestClient(networkID)
			if err != nil {
				log.Printf("Failed to get new client for network %d: %v", networkID, err)
				return
			}

			// Update our local client map with the new client
			if ethClient, ok := newClient.(interface{ GetClient() *ethclient.Client }); ok {
				pm.UpdateClient(networkID, ethClient.GetClient())
				log.Printf("Successfully updated local client for network %d after RPC switch", networkID)
			}
		} else {
			log.Printf("Network configuration does not support immediate RPC switching")
		}
	} else {
		log.Printf("No network configuration available for immediate RPC switch on network %d", networkID)
	}
}
