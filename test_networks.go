package main

import (
	"fmt"
	"log"
	"github.com/aelmanaa/chainlink-price-feed-golang/rpcscan"
)

func main() {
	// Test loading extraRpcs.json
	extraRPCs, err := rpcscan.LoadExtraRPCs("conf/extraRpcs.json")
	if err != nil {
		log.Fatalf("Failed to load extraRpcs.json: %v", err)
	}

	fmt.Printf("✅ Loaded %d networks from extraRpcs.json
", len(*extraRPCs))

	// Test creating network config
	config := &rpcscan.ExtendedConfig{}
	networkConfig := config.CreateNetworkConfig()
	
	fmt.Printf("✅ Created network configuration with %d networks
", len(networkConfig.Networks))
	
	// Show sample networks
	count := 0
	for _, network := range networkConfig.Networks {
		if count >= 5 {
			break
		}
		fmt.Printf("  - %s (%s): %d endpoints
", network.NameStd, network.NetworkID, len(network.Endpoints))
		count++
	}
	
	if len(networkConfig.Networks) > 5 {
		fmt.Printf("  ... and %d more networks
", len(networkConfig.Networks)-5)
	}
}
