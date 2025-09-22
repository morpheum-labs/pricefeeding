package pyth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHermesClient(t *testing.T) {
	client := NewHermesClient("https://hermes.pyth.network", nil)

	if client.baseURL != "https://hermes.pyth.network" {
		t.Errorf("Expected baseURL to be 'https://hermes.pyth.network', got '%s'", client.baseURL)
	}

	if client.timeout != DefaultTimeout {
		t.Errorf("Expected timeout to be %d, got %d", DefaultTimeout, client.timeout)
	}

	if client.httpRetries != DefaultHTTPRetries {
		t.Errorf("Expected httpRetries to be %d, got %d", DefaultHTTPRetries, client.httpRetries)
	}
}

func TestNewHermesClientWithConfig(t *testing.T) {
	timeout := DurationInMs(10000)
	retries := 5
	headers := map[string]string{"Authorization": "Bearer token"}

	config := &HermesClientConfig{
		Timeout:     &timeout,
		HTTPRetries: &retries,
		Headers:     headers,
	}

	client := NewHermesClient("https://hermes.pyth.network", config)

	if client.timeout != timeout {
		t.Errorf("Expected timeout to be %d, got %d", timeout, client.timeout)
	}

	if client.httpRetries != retries {
		t.Errorf("Expected httpRetries to be %d, got %d", retries, client.httpRetries)
	}

	if len(client.headers) != len(headers) {
		t.Errorf("Expected %d headers, got %d", len(headers), len(client.headers))
	}
}

func TestGetPriceFeeds(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/price_feeds" {
			t.Errorf("Expected path '/v2/price_feeds', got '%s'", r.URL.Path)
		}

		// Check query parameters
		query := r.URL.Query().Get("query")
		if query != "btc" {
			t.Errorf("Expected query 'btc', got '%s'", query)
		}

		assetType := r.URL.Query().Get("asset_type")
		if assetType != "crypto" {
			t.Errorf("Expected asset_type 'crypto', got '%s'", assetType)
		}

		// Return mock response
		response := []PriceFeedMetadata{
			{
				ID:          "test-id",
				Symbol:      "BTC/USD",
				AssetType:   AssetTypeCrypto,
				Description: "Bitcoin to US Dollar",
				Decimals:    8,
				Status:      "active",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	client := NewHermesClient(server.URL, nil)

	// Test GetPriceFeeds
	assetType := AssetTypeCrypto
	options := &GetPriceFeedsOptions{
		Query:     stringPtr("btc"),
		AssetType: &assetType,
	}

	feeds, err := client.GetPriceFeeds(context.Background(), options)
	if err != nil {
		t.Fatalf("GetPriceFeeds failed: %v", err)
	}

	if len(feeds) != 1 {
		t.Errorf("Expected 1 feed, got %d", len(feeds))
	}

	if feeds[0].ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", feeds[0].ID)
	}
}

func TestGetLatestPriceUpdates(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/updates/price/latest" {
			t.Errorf("Expected path '/v2/updates/price/latest', got '%s'", r.URL.Path)
		}

		// Check query parameters
		ids := r.URL.Query()["ids[]"]
		if len(ids) != 2 {
			t.Errorf("Expected 2 price IDs, got %d", len(ids))
		}

		encoding := r.URL.Query().Get("encoding")
		if encoding != "hex" {
			t.Errorf("Expected encoding 'hex', got '%s'", encoding)
		}

		// Return mock response
		response := PriceUpdate{
			Type:     "price_update",
			Encoding: "hex",
			Data:     "mock-data",
			Parsed: &ParsedPriceUpdate{
				PriceFeeds: []PriceFeed{
					{
						ID: "test-id",
						Price: Price{
							Price:       "50000",
							Conf:        "100",
							Expo:        -8,
							PublishTime: time.Now().Unix(),
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	client := NewHermesClient(server.URL, nil)

	// Test GetLatestPriceUpdates
	ids := []HexString{"id1", "id2"}
	encoding := EncodingTypeHex
	parsed := true

	options := &GetLatestPriceUpdatesOptions{
		Encoding: &encoding,
		Parsed:   &parsed,
	}

	updates, err := client.GetLatestPriceUpdates(context.Background(), ids, options)
	if err != nil {
		t.Fatalf("GetLatestPriceUpdates failed: %v", err)
	}

	if updates.Type != "price_update" {
		t.Errorf("Expected type 'price_update', got '%s'", updates.Type)
	}

	if updates.Parsed == nil {
		t.Error("Expected parsed data to be present")
	}

	if len(updates.Parsed.PriceFeeds) != 1 {
		t.Errorf("Expected 1 price feed, got %d", len(updates.Parsed.PriceFeeds))
	}
}

func TestGetLatestTwaps(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/updates/twap/5/latest" {
			t.Errorf("Expected path '/v2/updates/twap/5/latest', got '%s'", r.URL.Path)
		}

		// Return mock response
		response := TwapsResponse{
			Type:     "twaps_response",
			Encoding: "hex",
			Data:     "mock-data",
			Parsed: &ParsedTwapsUpdate{
				Twaps: []Twap{
					{
						ID: "test-id",
						Price: Price{
							Price:       "50000",
							Conf:        "100",
							Expo:        -8,
							PublishTime: time.Now().Unix(),
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	client := NewHermesClient(server.URL, nil)

	// Test GetLatestTwaps
	ids := []HexString{"id1"}
	encoding := EncodingTypeHex
	parsed := true

	options := &GetLatestTwapsOptions{
		Encoding: &encoding,
		Parsed:   &parsed,
	}

	twaps, err := client.GetLatestTwaps(context.Background(), ids, 5, options)
	if err != nil {
		t.Fatalf("GetLatestTwaps failed: %v", err)
	}

	if twaps.Type != "twaps_response" {
		t.Errorf("Expected type 'twaps_response', got '%s'", twaps.Type)
	}

	if twaps.Parsed == nil {
		t.Error("Expected parsed data to be present")
	}

	if len(twaps.Parsed.Twaps) != 1 {
		t.Errorf("Expected 1 TWAP, got %d", len(twaps.Parsed.Twaps))
	}
}

func TestGetLatestPublisherCaps(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/updates/publisher_stake_caps/latest" {
			t.Errorf("Expected path '/v2/updates/publisher_stake_caps/latest', got '%s'", r.URL.Path)
		}

		// Return mock response
		response := PublisherCaps{
			Type:     "publisher_caps",
			Encoding: "hex",
			Data:     "mock-data",
			Parsed: &ParsedPublisherCapsUpdate{
				PublisherStakeCaps: []PublisherStakeCap{
					{
						Publisher: "test-publisher",
						Cap:       "1000000",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	client := NewHermesClient(server.URL, nil)

	// Test GetLatestPublisherCaps
	encoding := EncodingTypeHex
	parsed := true

	options := &GetLatestPublisherCapsOptions{
		Encoding: &encoding,
		Parsed:   &parsed,
	}

	caps, err := client.GetLatestPublisherCaps(context.Background(), options)
	if err != nil {
		t.Fatalf("GetLatestPublisherCaps failed: %v", err)
	}

	if caps.Type != "publisher_caps" {
		t.Errorf("Expected type 'publisher_caps', got '%s'", caps.Type)
	}

	if caps.Parsed == nil {
		t.Error("Expected parsed data to be present")
	}

	if len(caps.Parsed.PublisherStakeCaps) != 1 {
		t.Errorf("Expected 1 publisher cap, got %d", len(caps.Parsed.PublisherStakeCaps))
	}
}

func TestBuildURL(t *testing.T) {
	client := NewHermesClient("https://hermes.pyth.network", nil)

	url := client.buildURL("price_feeds")
	expected := "https://hermes.pyth.network/v2/price_feeds"
	if url.String() != expected {
		t.Errorf("Expected URL '%s', got '%s'", expected, url.String())
	}
}

func TestBuildURLWithTrailingSlash(t *testing.T) {
	client := NewHermesClient("https://hermes.pyth.network/", nil)

	url := client.buildURL("price_feeds")
	expected := "https://hermes.pyth.network/v2/price_feeds"
	if url.String() != expected {
		t.Errorf("Expected URL '%s', got '%s'", expected, url.String())
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
