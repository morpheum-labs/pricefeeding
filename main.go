package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aelmanaa/chainlink-price-feed-golang/pricefeed"
	"github.com/aelmanaa/chainlink-price-feed-golang/rpcscan"
)

// PriceCacheManager manages the local price cache with persistence
type PriceCacheManager struct {
	cache     *pricefeed.PriceCache
	mu        sync.RWMutex
	lastSaved time.Time
}

// NewPriceCacheManager creates a new price cache manager
func NewPriceCacheManager() *PriceCacheManager {
	return &PriceCacheManager{
		cache:     pricefeed.NewPriceCache(),
		lastSaved: time.Now(),
	}
}

// UpdatePrice updates a price in the cache
func (pcm *PriceCacheManager) UpdatePrice(networkID uint64, feedAddress string, priceData *pricefeed.PriceData) {
	pcm.cache.UpdatePrice(networkID, feedAddress, priceData)
}

// GetPrice retrieves a price from the cache
func (pcm *PriceCacheManager) GetPrice(networkID uint64, feedAddress string) (*pricefeed.PriceData, error) {
	return pcm.cache.GetPrice(networkID, feedAddress)
}

// GetAllPrices retrieves all prices for a network
func (pcm *PriceCacheManager) GetAllPrices(networkID uint64) map[string]*pricefeed.PriceData {
	return pcm.cache.GetAllPrices(networkID)
}

// AddFeed adds a price feed to monitor
func (pcm *PriceCacheManager) AddFeed(networkID uint64, feedAddress string) {
	pcm.cache.AddFeed(networkID, feedAddress)
}

func main() {
	log.Println("Starting Chainlink Price Feed Monitor with Switchable RPC Clients...")

	// Load YAML configuration
	config, err := rpcscan.LoadYamlConfig(".")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create network configuration from YAML config
	networkConfig := config.CreateNetworkConfig()

	// Create price cache manager
	priceCacheManager := NewPriceCacheManager()

	// Start RPC monitoring with optimized intervals
	stopChan := make(chan struct{})
	go rpcscan.MonitorAllRPCEndpoints(&rpcscan.Config{RootDir: "."}, networkConfig, config.GetRPCCheckInterval(), stopChan)

	// Wait for initial RPC clients to be established
	log.Println("Waiting for RPC clients to be established...")
	time.Sleep(10 * time.Second)

	// Create price monitor with 30-second intervals as requested
	priceMonitor := pricefeed.NewPriceMonitor(30 * time.Second)

	// Add clients and price feeds to monitor
	clients := networkConfig.GetAllClients()
	for networkID, client := range clients {
		priceMonitor.AddClient(networkID, client.GetClient())

		// Add price feeds for this network from YAML configuration
		feeds := config.GetPriceFeedsForNetwork(networkID)
		for _, feed := range feeds {
			if feed.Address != "" && feed.Address != "0x" {
				priceMonitor.AddPriceFeed(networkID, feed.Address)
				priceCacheManager.AddFeed(networkID, feed.Address)
				log.Printf("Added price feed %s (%s) for network %d", feed.Name, feed.Address, networkID)
			}
		}
	}

	// Start price monitoring
	go priceMonitor.Start()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start price cache updater goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Update local cache with latest prices
				clients := networkConfig.GetAllClients()
				for networkID := range clients {
					prices := priceMonitor.GetAllPrices(networkID)
					for feedAddress, priceData := range prices {
						priceCacheManager.UpdatePrice(networkID, feedAddress, priceData)
					}
				}
			}
		}
	}()

	// Start price display goroutine
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Display current prices for all networks
				clients := networkConfig.GetAllClients()
				for networkID := range clients {
					prices := priceCacheManager.GetAllPrices(networkID)
					if len(prices) > 0 {
						log.Printf("=== Network %d (Chain ID: %d) Prices ===", networkID, networkID)
						for feedAddress, priceData := range prices {
							// Convert price to human readable format (assuming 8 decimals)
							priceFloat := float64(priceData.Answer.Int64()) / 1e8
							log.Printf("Feed %s: $%.2f (Updated: %s, Round: %s)",
								feedAddress,
								priceFloat,
								priceData.Timestamp.Format(time.RFC3339),
								priceData.RoundID.String())
						}
					}
				}
			}
		}
	}()

	// Start RPC health monitoring
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				clients := networkConfig.GetAllClients()
				log.Printf("Active RPC clients: %d", len(clients))
				for networkID, client := range clients {
					lastUpdated := client.GetLastUpdated()
					log.Printf("Network %d: Last updated %v ago", networkID, time.Since(lastUpdated))
				}
			}
		}
	}()

	log.Println("Chainlink Price Feed Monitor started successfully!")
	log.Println("Features:")
	log.Println("- Switchable RPC clients for consistent connections")
	log.Println("- 30-second price polling intervals")
	log.Println("- Local price cache storage")
	log.Println("- Optimized computation with concurrent request limiting")
	log.Println("- Graceful shutdown handling")
	log.Println("Press Ctrl+C to stop.")

	// Main event loop - wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal, initiating graceful shutdown...")
	cancel() // Cancel all goroutines
	priceMonitor.Stop()
	close(stopChan)

	// Wait a moment for goroutines to finish
	time.Sleep(2 * time.Second)
	log.Println("Shutdown complete")
}
