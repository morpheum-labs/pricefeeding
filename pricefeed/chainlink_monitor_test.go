package pricefeed

import (
	"testing"
	"time"
)

func TestPriceMonitorCreation(t *testing.T) {
	interval := 30 * time.Second
	immediateMode := true

	monitor := NewPriceMonitorWithImmediateMode(interval, immediateMode)

	if monitor == nil {
		t.Fatal("Expected monitor to be created, got nil")
	}

	if monitor.cache == nil {
		t.Fatal("Expected cache to be initialized, got nil")
	}

	if monitor.interval != interval {
		t.Errorf("Expected interval %v, got %v", interval, monitor.interval)
	}

	if monitor.immediateMode != immediateMode {
		t.Errorf("Expected immediate mode %v, got %v", immediateMode, monitor.immediateMode)
	}
}

func TestPriceMonitorAddFeedWithSymbol(t *testing.T) {
	monitor := NewPriceMonitorWithImmediateMode(30*time.Second, true)

	networkID := uint64(42161)                                  // Arbitrum
	feedAddress := "0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612" // ETH/USD on Arbitrum
	symbol := "ETH/USD"

	monitor.AddPriceFeedWithSymbol(networkID, feedAddress, symbol)

	monitor.mu.RLock()
	defer monitor.mu.RUnlock()

	if monitor.feedSymbols[networkID] == nil {
		t.Fatal("Expected feed symbols map to be initialized for network")
	}

	if monitor.feedSymbols[networkID][feedAddress] != symbol {
		t.Errorf("Expected symbol %s for feed %s, got %s", symbol, feedAddress, monitor.feedSymbols[networkID][feedAddress])
	}
}

func TestGetFeedSymbol(t *testing.T) {
	monitor := NewPriceMonitorWithImmediateMode(30*time.Second, true)

	networkID := uint64(42161)
	feedAddress := "0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612"
	symbol := "ETH/USD"

	monitor.AddPriceFeedWithSymbol(networkID, feedAddress, symbol)

	retrievedSymbol := monitor.GetFeedSymbol(networkID, feedAddress)
	if retrievedSymbol != symbol {
		t.Errorf("Expected symbol %s, got %s", symbol, retrievedSymbol)
	}

	// Test unknown feed
	unknownSymbol := monitor.GetFeedSymbol(networkID, "0x0000000000000000000000000000000000000000")
	if unknownSymbol != "Unknown" {
		t.Errorf("Expected 'Unknown' for unknown feed, got %s", unknownSymbol)
	}
}

func TestChainlinkImmediateModeToggle(t *testing.T) {
	monitor := NewPriceMonitorWithImmediateMode(30*time.Second, false)

	if monitor.immediateMode != false {
		t.Errorf("Expected immediate mode to be false, got %v", monitor.immediateMode)
	}

	monitor.SetImmediateMode(true)

	if monitor.immediateMode != true {
		t.Errorf("Expected immediate mode to be true, got %v", monitor.immediateMode)
	}
}

func TestPrintStatus(t *testing.T) {
	monitor := NewPriceMonitorWithImmediateMode(30*time.Second, true)

	// Add some test feeds
	monitor.AddPriceFeedWithSymbol(42161, "0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612", "ETH/USD")
	monitor.AddPriceFeedWithSymbol(42161, "0x316978519aD4F9c7E99B3Ac5C1Dd2C3E8E8D7B1A", "BTC/USD")

	// This should not panic
	monitor.PrintStatus()
}
