package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/morpheum/chainlink-price-feed-golang/pricefeed"
	"github.com/morpheum/chainlink-price-feed-golang/rpcscan"
)

func main() {
	// Parse command line arguments
	var (
		chainlink = flag.Bool("chainlink", false, "Start Chainlink price feed monitor")
		pyth      = flag.Bool("pyth", false, "Start Pyth price feed client")
	)
	flag.Parse()

	// Check if any mode is specified
	if !*chainlink && !*pyth {
		fmt.Println("Usage:")
		fmt.Println("  --chainlink    Start Chainlink price feed monitor")
		fmt.Println("  --pyth         Start Pyth price feed client")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  go run . --chainlink")
		fmt.Println("  go run . --pyth")
		os.Exit(1)
	}

	// Check if both modes are specified
	if *chainlink && *pyth {
		log.Fatal("Cannot start both Chainlink and Pyth clients simultaneously. Please choose one.")
	}

	// Start the appropriate service
	if *chainlink {
		log.Println("Starting Chainlink price feed monitor...")
		chainlink_start()
	} else if *pyth {
		log.Println("Starting Pyth price feed client...")
		pyth_start()
	}
}

// Helper functions for pointer creation
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func chainlink_start() {
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
	priceCacheManager := pricefeed.NewPriceCacheManager()

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

	// Create price monitor with 30-second intervals and immediate mode enabled
	priceMonitor := pricefeed.NewPriceMonitorWithImmediateMode(30*time.Second, true)

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
				// Use the enhanced method with symbol
				priceMonitor.AddPriceFeedWithSymbol(networkID, feed.Address, feed.Symbol)
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

	// Start status monitoring goroutine
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Print Chainlink monitor status
				priceMonitor.PrintStatus()

				// Display current prices for all networks
				clients := networkConfig.GetAllClients()
				for networkID := range clients {
					prices := priceCacheManager.GetAllPrices(networkID)
					if len(prices) > 0 {
						log.Printf("📊 CURRENT CHAINLINK PRICES - Network %d:", networkID)

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

							// Convert price to human readable format (assuming 8 decimals)
							// Use big.Float for proper precision
							priceFloat := new(big.Float).SetInt(priceData.Answer)
							divisor := new(big.Float).SetInt64(1e8) // 10^8
							priceFloat.Quo(priceFloat, divisor)
							
							// Convert to float64 for display
							priceValue, _ := priceFloat.Float64()

							log.Printf("  %s (%s): $%.2f (Updated: %s, Round: %s)",
								feedName,
								symbol,
								priceValue,
								priceData.Timestamp.Format("15:04:05"),
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
	log.Println("- 30-second price polling intervals with immediate mode")
	log.Println("- Local price cache storage with persistence tracking")
	log.Println("- Optimized computation with concurrent request limiting")
	log.Println("- Enhanced price display with symbol mapping")
	log.Println("- Real-time status monitoring and cache tracking")
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

// Helper function to get asset name from price ID
func getAssetName(priceId string, priceIdToAsset map[string]string) string {
	if assetName, exists := priceIdToAsset[priceId]; exists {
		return assetName
	}
	return "Unknown"
}

func pyth_start() {
	log.Println("Starting Pyth Price Feed Monitor...")

	// Default configuration
	endpoint := "https://hermes.pyth.network"
	interval := 10 * time.Second // Poll every 10 seconds
	immediateMode := true        // Print prices immediately when received

	// Define price feed IDs and their symbols
	priceFeeds := map[string]string{
		"e62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43": "BTC/USD",
		"ff61491a931112ddf1bd8147cd1b641375f79f5825126d665480874634fd0ace": "ETH/USD",
		// Add more price feeds here as needed:
		// "ef0d8b6fda2ceba41da15d4095d1da392a0d2f8ed0c6c7bc0f4cfac8c280b56d": "SOL/USD",
		// "93da3352f9f1d105fdf1104972eccd99cebaecc431460e19d20f67a0f6b59200": "AVAX/USD",
	}

	// Create Pyth price monitor
	monitor := pricefeed.NewPythPriceMonitor(endpoint, interval, immediateMode)

	// Add price feeds to monitor
	for priceID, symbol := range priceFeeds {
		monitor.AddPriceFeed(priceID, symbol)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the monitor in a goroutine
	go monitor.Start()

	// Start a status display goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-sigChan:
				return
			case <-ticker.C:
				// Print cache status every 30 seconds
				monitor.PrintLastSavedStatus()

				// Also print all current prices
				allPrices := monitor.GetAllPrices()
				if len(allPrices) > 0 {
					log.Printf("📊 CURRENT PRICES:")
					for _, priceData := range allPrices {
						log.Printf("  %s: $%s (Updated: %s)",
							priceData.Symbol,
							priceData.Price.String(),
							priceData.Timestamp.Format("15:04:05"))
					}
				}
			}
		}
	}()

	log.Printf("Pyth Price Feed Monitor started successfully!")
	log.Printf("Monitoring %d price feeds:", len(priceFeeds))
	for priceID, symbol := range priceFeeds {
		log.Printf("  - %s (%s)", symbol, priceID)
	}
	log.Println("Features:")
	log.Printf("- Polling every %v", interval)
	log.Println("- Immediate price printing when updates are received")
	log.Println("- Local price cache with persistence tracking")
	log.Println("- Thread-safe operations")
	log.Println("- Graceful shutdown handling")
	log.Println("Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal, stopping Pyth price monitor...")
	monitor.Stop()
	log.Println("Pyth price monitor stopped.")
}
