package chainlink

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	aggregatorv3 "github.com/morpheum-labs/pricefeeding/aggregatorv3"
	"github.com/morpheum-labs/pricefeeding/types"
)

// RPCSwitcher is an interface for handling RPC endpoint switching
type RPCSwitcher interface {
	SwitchRPCEndpointImmediately(networkID uint64) error
	GetBestClient(networkID uint64) (*ethclient.Client, error)
}

// FetchPriceDataOptions contains options for fetching price data
type FetchPriceDataOptions struct {
	NetworkID      uint64
	FeedAddress    string
	Client         *ethclient.Client
	RPCSwitcher    RPCSwitcher // Optional RPC switcher for retry logic
	MaxRetries     int         // Maximum number of retries (default: 1)
	RetryDelay     time.Duration // Delay between retries (default: 2 seconds)
}

// FetchPriceData fetches price data from a Chainlink aggregator contract
// This is the main entry point for fetching Chainlink price feeds
func FetchPriceData(opts FetchPriceDataOptions) (*types.ChainlinkPrice, error) {
	if opts.Client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}
	if opts.FeedAddress == "" {
		return nil, fmt.Errorf("feed address cannot be empty")
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 1 // Default to 1 retry
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = 2 * time.Second // Default 2 second delay
	}

	return fetchPriceDataWithRetry(opts, 1)
}

// fetchPriceDataWithRetry fetches price data with retry logic after RPC switching
func fetchPriceDataWithRetry(opts FetchPriceDataOptions, attempt int) (*types.ChainlinkPrice, error) {
	// Create the aggregator contract instance
	contractAddress := common.HexToAddress(opts.FeedAddress)
	aggregator, err := aggregatorv3.NewAggregatorV3Interface(contractAddress, opts.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregator contract: %v", err)
	}

	// Get the latest round data
	roundData, err := aggregator.LatestRoundData(&bind.CallOpts{})
	if err != nil {
		// Check if this is the specific error code -32097 that requires immediate RPC switching
		if IsErrorCode32097(err) && opts.RPCSwitcher != nil && attempt <= opts.MaxRetries {
			log.Printf("Detected error code -32097 for network %d, triggering immediate RPC switch (attempt %d)", opts.NetworkID, attempt)
			
			// Trigger immediate RPC switching for this network
			if err := opts.RPCSwitcher.SwitchRPCEndpointImmediately(opts.NetworkID); err != nil {
				log.Printf("Failed to switch RPC endpoint for network %d: %v", opts.NetworkID, err)
				return nil, fmt.Errorf("failed to get latest round data: %v", err)
			}

			// Get the new client
			newClient, err := opts.RPCSwitcher.GetBestClient(opts.NetworkID)
			if err != nil {
				log.Printf("Failed to get new client for network %d: %v", opts.NetworkID, err)
				return nil, fmt.Errorf("failed to get latest round data: %v", err)
			}

			// Wait a moment for the RPC switch to complete
			time.Sleep(opts.RetryDelay)

			// Update client in options for retry
			opts.Client = newClient

			// Retry with the new RPC endpoint
			log.Printf("Retrying price fetch for network %d with new RPC endpoint (attempt %d)", opts.NetworkID, attempt+1)
			return fetchPriceDataWithRetry(opts, attempt+1)
		}
		return nil, fmt.Errorf("failed to get latest round data: %v", err)
	}

	// Get decimals from contract
	decimals, err := aggregator.Decimals(&bind.CallOpts{})
	if err != nil {
		// Log warning and use default
		log.Printf("Warning: Failed to get decimals for feed %s, using default -8: %v", opts.FeedAddress, err)
		decimals = 8 // Default for most Chainlink feeds
	}

	// Convert to our ChainlinkPrice structure
	priceData := &types.ChainlinkPrice{
		RoundID:         roundData.RoundId,
		Answer:          roundData.Answer,
		Exponent:        -int(decimals), // Negative sign as requested
		StartedAt:       roundData.StartedAt,
		UpdatedAt:       roundData.UpdatedAt,
		AnsweredInRound: roundData.AnsweredInRound,
		Timestamp:       time.Now(),
		NetworkID:       opts.NetworkID,
		FeedAddress:     opts.FeedAddress,
	}

	return priceData, nil
}

// IsErrorCode32097 checks if the error contains the specific error code -32097
// This error code typically indicates execution reverted, which may require RPC switching
func IsErrorCode32097(err error) bool {
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
