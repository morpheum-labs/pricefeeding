package pricefeed

import (
	"math/big"
	"testing"
	"time"

	"github.com/morpheum-labs/pricefeeding/types"
)

func TestPythPriceMonitorCreation(t *testing.T) {
	endpoint := "https://hermes.pyth.network"
	interval := 5 * time.Second
	immediateMode := true

	monitor := NewPythPriceMonitor(endpoint, interval, immediateMode)

	if monitor == nil {
		t.Fatal("Expected monitor to be created, got nil")
	}

	if monitor.client == nil {
		t.Fatal("Expected client to be initialized, got nil")
	}

	if monitor.cacheManager == nil {
		t.Fatal("Expected cache manager to be initialized, got nil")
	}

	if monitor.interval != interval {
		t.Errorf("Expected interval %v, got %v", interval, monitor.interval)
	}

	if monitor.immediateMode != immediateMode {
		t.Errorf("Expected immediate mode %v, got %v", immediateMode, monitor.immediateMode)
	}
}

func TestPythPriceMonitorAddFeed(t *testing.T) {
	monitor := NewPythPriceMonitor("https://hermes.pyth.network", 5*time.Second, true)

	priceID := "0xe62df6c8b4a85fe1a67db44dc12de5db330f7ac66b72dc658afedf0f4a415b43"
	symbol := "BTC/USD"

	monitor.AddPriceFeed(priceID, symbol)

	monitor.mu.RLock()
	defer monitor.mu.RUnlock()

	if len(monitor.priceFeeds) != 1 {
		t.Errorf("Expected 1 price feed, got %d", len(monitor.priceFeeds))
	}

	if monitor.priceFeeds[priceID] != symbol {
		t.Errorf("Expected symbol %s for price ID %s, got %s", symbol, priceID, monitor.priceFeeds[priceID])
	}
}

func TestPythPriceDataStructure(t *testing.T) {
	// Create a test PythPrice
	pythData := &types.PythPrice{
		ID:          "test-id",
		Symbol:      "BTC/USD",
		Price:       big.NewInt(50000000000), // 50000 * 10^6
		Confidence:  big.NewInt(1000000),     // 1 * 10^6
		Exponent:    6,
		PublishTime: 1640995200, // Unix timestamp
		Slot:        12345,
		Timestamp:   time.Now(),
		NetworkID:   uint64(types.OracleNetworkIDPyth),
	}

	// Test PriceInfo interface methods
	if pythData.GetSource() != types.SourcePyth {
		t.Errorf("Expected source %s, got %s", types.SourcePyth, pythData.GetSource())
	}

	if pythData.GetNetworkID() != uint64(types.OracleNetworkIDPyth) {
		t.Errorf("Expected network ID %d, got %d", types.OracleNetworkIDPyth, pythData.GetNetworkID())
	}

	if pythData.GetIdentifier() != pythData.ID {
		t.Errorf("Expected identifier %s, got %s", pythData.ID, pythData.GetIdentifier())
	}

	if pythData.GetPrice().Cmp(pythData.Price) != 0 {
		t.Errorf("Expected price %s, got %s", pythData.Price.String(), pythData.GetPrice().String())
	}
}

func TestCacheManagerIntegration(t *testing.T) {
	monitor := NewPythPriceMonitor("https://hermes.pyth.network", 5*time.Second, true)

	// Test that cache manager is properly initialized
	cacheManager := monitor.GetCacheManager()
	if cacheManager == nil {
		t.Fatal("Expected cache manager to be non-nil")
	}

	// Test lastSaved tracking
	initialTime := cacheManager.GetLastSaved()
	time.Sleep(10 * time.Millisecond) // Small delay

	cacheManager.UpdateLastSaved()
	updatedTime := cacheManager.GetLastSaved()

	if !updatedTime.After(initialTime) {
		t.Error("Expected lastSaved to be updated")
	}
}

func TestImmediateModeToggle(t *testing.T) {
	monitor := NewPythPriceMonitor("https://hermes.pyth.network", 5*time.Second, false)

	if monitor.immediateMode != false {
		t.Errorf("Expected immediate mode to be false, got %v", monitor.immediateMode)
	}

	monitor.SetImmediateMode(true)

	if monitor.immediateMode != true {
		t.Errorf("Expected immediate mode to be true, got %v", monitor.immediateMode)
	}
}
