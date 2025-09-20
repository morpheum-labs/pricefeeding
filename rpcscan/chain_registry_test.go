package rpcscan

import (
	"strings"
	"testing"
)

func TestReadChainRegistryFromJS(t *testing.T) {
	// Create a config pointing to the project root
	conf := &Config{
		RootDir: "/Users/hesdx/Documents/b95/swapbiz/chainlink-price-feed",
	}

	// Test reading chain registry from JavaScript files
	networkConfig, err := ReadChainRegistryFromJS(conf)
	if err != nil {
		t.Fatalf("Failed to read chain registry: %v", err)
	}

	// Verify we got some networks
	if len(networkConfig.Networks) == 0 {
		t.Error("Expected to find some networks, but got none")
	}

	// Check that we have the expected structure
	for _, network := range networkConfig.Networks {
		if network.NetworkID == "" {
			t.Error("Network ID should not be empty")
		}
		if network.NameStd == "" {
			t.Error("Network name should not be empty")
		}
		if network.NameCoinr == "" {
			t.Error("Network symbol should not be empty")
		}
		if len(network.Endpoints) == 0 {
			t.Error("Network should have at least one RPC endpoint")
		}
	}

	t.Logf("Successfully loaded %d networks from chain registry", len(networkConfig.Networks))
}

func TestParseChainRegistryFile(t *testing.T) {
	// Test parsing a specific file
	filePath := "/Users/hesdx/Documents/b95/swapbiz/chainlink-price-feed/constants/additionalChainRegistry/chainid-14.js"

	chainData, err := parseChainRegistryFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse chain registry file: %v", err)
	}

	// Verify the parsed data
	if chainData.ChainID != 14 {
		t.Errorf("Expected ChainID 14, got %d", chainData.ChainID)
	}
	if chainData.Name != "Flare Mainnet" {
		t.Errorf("Expected name 'Flare Mainnet', got '%s'", chainData.Name)
	}
	if len(chainData.RPC) == 0 {
		t.Error("Expected at least one RPC endpoint")
	}

	t.Logf("Successfully parsed chain data: %s (ChainID: %d)", chainData.Name, chainData.ChainID)
}

func TestConvertJSToJSON(t *testing.T) {
	jsContent := `export const data = {
		"name": "Test Chain",
		"chainId": 123,
		"rpc": ["https://rpc.test.com"]
	};`

	jsonContent, err := convertJSToJSON(jsContent)
	if err != nil {
		t.Fatalf("Failed to convert JS to JSON: %v", err)
	}

	// Check that the export statement was removed
	if strings.Contains(jsonContent, "export") {
		t.Error("Export statement should be removed from JSON content")
	}

	// Check that the semicolon was removed
	if strings.Contains(jsonContent, ";") {
		t.Error("Semicolon should be removed from JSON content")
	}

	t.Logf("Converted JS to JSON: %s", jsonContent)
}

func TestConvertToRPCConfig(t *testing.T) {
	chainData := &ChainRegistryData{
		Name:    "Test Chain",
		ChainID: 123,
		NativeCurrency: struct {
			Name     string `json:"name"`
			Symbol   string `json:"symbol"`
			Decimals int    `json:"decimals"`
		}{
			Name:     "Test Token",
			Symbol:   "TEST",
			Decimals: 18,
		},
		RPC: []string{"https://rpc.test.com", "https://rpc2.test.com"},
	}

	rpcConfig := convertToRPCConfig(chainData)

	// Verify the conversion
	if rpcConfig.NetworkID != "123" {
		t.Errorf("Expected NetworkID '123', got '%s'", rpcConfig.NetworkID)
	}
	if rpcConfig.NameStd != "Test Chain" {
		t.Errorf("Expected NameStd 'Test Chain', got '%s'", rpcConfig.NameStd)
	}
	if rpcConfig.NameCoinr != "TEST" {
		t.Errorf("Expected NameCoinr 'TEST', got '%s'", rpcConfig.NameCoinr)
	}
	if len(rpcConfig.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(rpcConfig.Endpoints))
	}

	t.Logf("Successfully converted to RPCConfig: %s", rpcConfig.NameStd)
}
