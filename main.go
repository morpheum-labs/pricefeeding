package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/morpheum/chainlink-price-feed-golang/pricefeed"
	"github.com/morpheum/chainlink-price-feed-golang/rpcscan"
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

	// Create price feed manager for Arbitrum network (Chain ID: 42161)
	priceFeedManager := rpcscan.NewPriceFeedManager(42161)

	// Load price feed configurations from YAML files
	if err := priceFeedManager.LoadConfig("conf"); err != nil {
		log.Fatalf("Failed to load price feed configurations: %v", err)
	}

	// Log loaded feeds for debugging
	allFeeds := priceFeedManager.GetAllFeeds()
	log.Printf("Loaded %d price feeds from configuration files", len(allFeeds))
	for _, feed := range allFeeds {
		log.Printf("  - %s (%s): %s", feed.Name, feed.Symbol, feed.Address)
	}

	// Create network configuration from price feed configs
	networkConfig := priceFeedManager.CreateNetworkConfig()

	// Create price cache manager
	priceCacheManager := NewPriceCacheManager()

	// Start RPC monitoring with optimized intervals
	stopChan := make(chan struct{})
	log.Printf("Starting RPC monitoring with %d networks", len(networkConfig.Networks))
	for _, network := range networkConfig.Networks {
		log.Printf("Network %s has %d endpoints", network.NetworkID, len(network.Endpoints))
	}
	go rpcscan.MonitorAllRPCEndpoints(&rpcscan.Config{RootDir: "."}, networkConfig, priceFeedManager.GetDefaultRPCCheckInterval(), stopChan)

	// Wait for initial RPC clients to be established
	log.Println("Waiting for RPC clients to be established...")
	time.Sleep(35 * time.Second) // Wait for at least one RPC monitoring cycle

	// Wait for RPC clients to be available
	maxRetries := 10
	retryCount := 0
	for {
		clients := networkConfig.GetAllClients()
		if len(clients) > 0 {
			log.Printf("Found %d clients to add to monitor", len(clients))
			break
		}
		retryCount++
		if retryCount >= maxRetries {
			log.Fatalf("Failed to establish RPC clients after %d retries", maxRetries)
		}
		log.Printf("No clients found, retrying in 2 seconds... (attempt %d/%d)", retryCount, maxRetries)
		time.Sleep(2 * time.Second)
	}

	// Create price monitor with 30-second intervals as requested
	priceMonitor := pricefeed.NewPriceMonitor(30 * time.Second)

	// Set network configuration for RPC switching
	priceMonitor.SetNetworkConfig(networkConfig)

	// Add clients and price feeds to monitor
	clients := networkConfig.GetAllClients()
	for networkID, client := range clients {
		log.Printf("Adding client for network %d", networkID)
		priceMonitor.AddClient(networkID, client.GetClient())

		// Add price feeds for this network from price feed configuration
		feeds := priceFeedManager.GetFeedsForNetwork(networkID)
		log.Printf("Found %d feeds for network %d", len(feeds), networkID)
		for _, feed := range feeds {
			if feed.Address != "" && feed.Address != "0x" {
				priceMonitor.AddPriceFeed(networkID, feed.Address)
				priceCacheManager.AddFeed(networkID, feed.Address)
				log.Printf("Added price feed %s (%s) for network %d - %s", feed.Name, feed.Address, networkID, feed.Symbol)
			} else {
				log.Printf("Skipping invalid feed %s with address: %s", feed.Name, feed.Address)
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
		ticker := time.NewTicker(15 * time.Second)
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

	// Start client refresh goroutine to ensure we have the latest RPC endpoints
	go func() {
		ticker := time.NewTicker(15 * time.Second) // Refresh clients every minute
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Refresh clients from network configuration
				clients := networkConfig.GetAllClients()
				for networkID, client := range clients {
					priceMonitor.UpdateClient(networkID, client.GetClient())
				}
				log.Printf("Refreshed %d clients from network configuration", len(clients))
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

						// Get all feeds to match addresses with names
						allFeeds := priceFeedManager.GetAllFeeds()
						feedMap := make(map[string]rpcscan.PriceFeedInfo)
						for _, feed := range allFeeds {
							feedMap[feed.Address] = feed
						}

						for feedAddress, priceData := range prices {
							// Find feed info
							feedInfo, exists := feedMap[feedAddress]
							var feedName, symbol string
							if exists {
								feedName = feedInfo.Name
								symbol = feedInfo.Symbol
							} else {
								feedName = "Unknown"
								symbol = "Unknown"
							}

							// Convert price to human readable format based on decimals
							decimals := 8 // default
							if exists {
								decimals = feedInfo.Decimals
							}
							priceFloat := float64(priceData.Answer.Int64()) / float64(1e8)
							if decimals != 8 {
								priceFloat = float64(priceData.Answer.Int64()) / float64(1e8)
							}

							log.Printf("Feed %s (%s): $%.2f (Updated: %s, Round: %s)",
								feedName,
								symbol,
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
