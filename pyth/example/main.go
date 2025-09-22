package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/morpheum/chainlink-price-feed-golang/pyth"
)

func main() {
	// Parse command line arguments
	var (
		endpoint = flag.String("endpoint", "https://hermes.pyth.network", "Endpoint URL for the price service")
		priceIds = flag.String("price-ids", "", "Space separated price feed ids (in hex without leading 0x) to fetch")
	)
	flag.Parse()

	if *priceIds == "" {
		log.Fatal("price-ids is required")
	}

	// Parse price IDs
	ids := strings.Fields(*priceIds)
	hexIds := make([]pyth.HexString, len(ids))
	for i, id := range ids {
		hexIds[i] = pyth.HexString(id)
	}

	// Extract basic auth from URL if present
	endpointURL, headers := extractBasicAuthFromURL(*endpoint)

	// Create Hermes client
	client := pyth.NewHermesClient(endpointURL, &pyth.HermesClientConfig{
		Headers: headers,
	})

	ctx := context.Background()

	// Get price feeds
	fmt.Println("Price feeds matching 'btc' with asset type 'crypto':")
	assetType := pyth.AssetTypeCrypto
	priceFeeds, err := client.GetPriceFeeds(ctx, &pyth.GetPriceFeedsOptions{
		Query:     stringPtr("btc"),
		AssetType: &assetType,
	})
	if err != nil {
		log.Fatalf("Failed to get price feeds: %v", err)
	}

	for _, feed := range priceFeeds {
		fmt.Printf("ID: %s, Symbol: %s, Asset Type: %s, Description: %s\n",
			feed.ID, feed.Symbol, feed.AssetType, feed.Description)
	}

	// Latest price updates
	fmt.Printf("\nLatest price updates for price IDs %v:\n", ids)
	parsed := true
	encoding := pyth.EncodingTypeHex
	priceUpdates, err := client.GetLatestPriceUpdates(ctx, hexIds, &pyth.GetLatestPriceUpdatesOptions{
		Encoding: &encoding,
		Parsed:   &parsed,
	})
	if err != nil {
		log.Fatalf("Failed to get latest price updates: %v", err)
	}

	if priceUpdates.Parsed != nil {
		for _, feed := range priceUpdates.Parsed.PriceFeeds {
			fmt.Printf("Price Feed ID: %s\n", feed.ID)
			fmt.Printf("  Price: %s (expo: %d)\n", feed.Price.Price, feed.Price.Expo)
			fmt.Printf("  Confidence: %s\n", feed.Price.Conf)
			fmt.Printf("  Publish Time: %d\n", feed.Price.PublishTime)
			fmt.Printf("  EMA Price: %s (expo: %d)\n", feed.Ema.Price, feed.Ema.Expo)
			fmt.Println()
		}
	}

	// Get the latest 5 second TWAPs
	fmt.Printf("Latest 5 second TWAPs for price IDs %v:\n", ids)
	twapUpdates, err := client.GetLatestTwaps(ctx, hexIds, 5, &pyth.GetLatestTwapsOptions{
		Encoding: &encoding,
		Parsed:   &parsed,
	})
	if err != nil {
		log.Fatalf("Failed to get latest TWAPs: %v", err)
	}

	if twapUpdates.Parsed != nil {
		for _, twap := range twapUpdates.Parsed.Twaps {
			fmt.Printf("TWAP ID: %s\n", twap.ID)
			fmt.Printf("  Price: %s (expo: %d)\n", twap.Price.Price, twap.Price.Expo)
			fmt.Printf("  Confidence: %s\n", twap.Price.Conf)
			fmt.Printf("  Publish Time: %d\n", twap.Price.PublishTime)
			fmt.Println()
		}
	}

	// Streaming price updates
	fmt.Printf("Streaming latest prices for price IDs %v...\n", ids)
	eventSource, err := client.GetPriceUpdatesStream(ctx, hexIds, &pyth.GetPriceUpdatesStreamOptions{
		Encoding:       &encoding,
		Parsed:         &parsed,
		AllowUnordered: boolPtr(false),
		BenchmarksOnly: boolPtr(true),
	})
	if err != nil {
		log.Fatalf("Failed to get price updates stream: %v", err)
	}

	// Set up event handlers
	eventSource.OnMessage(func(data string) {
		fmt.Println("Received price update:", data)

		// Parse the price update
		var priceUpdate pyth.PriceUpdate
		if err := json.Unmarshal([]byte(data), &priceUpdate); err != nil {
			fmt.Printf("Failed to parse price update: %v\n", err)
			return
		}

		if priceUpdate.Parsed != nil {
			for _, feed := range priceUpdate.Parsed.PriceFeeds {
				fmt.Printf("Streamed Price Feed ID: %s, Price: %s\n", feed.ID, feed.Price.Price)
			}
		}
	})

	eventSource.OnError(func(err error) {
		fmt.Printf("Error receiving updates: %v\n", err)
		eventSource.Close()
	})

	// Wait for 5 seconds to receive updates
	time.Sleep(5 * time.Second)

	// Close the event source
	fmt.Println("Closing event source.")
	eventSource.Close()
}

// extractBasicAuthFromURL extracts basic authentication from a URL
func extractBasicAuthFromURL(urlStr string) (string, map[string]string) {
	headers := make(map[string]string)

	// Simple check for basic auth in URL
	if strings.Contains(urlStr, "@") {
		// This is a simplified version - in a real implementation,
		// you'd want to properly parse the URL and extract credentials
		// For now, we'll just return the URL as-is
		return urlStr, headers
	}

	return urlStr, headers
}

// Helper functions for pointer creation
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
