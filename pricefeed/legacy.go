package pricefeed

import (
	"math/big"
	"time"
)

// PriceData represents price information from Chainlink (deprecated, use ChainlinkPrice)
// Kept for backward compatibility during migration
type PriceData struct {
	RoundID         *big.Int
	Answer          *big.Int
	Exponent        int
	StartedAt       *big.Int
	UpdatedAt       *big.Int
	AnsweredInRound *big.Int
	Timestamp       time.Time
	NetworkID       uint64
}

// Legacy methods for backward compatibility (deprecated)
// These methods are kept for backward compatibility and will be removed in a future version.

// UpdatePriceLegacy updates a price using the old format (assumes Chainlink)
func (pcm *PriceCacheManager) UpdatePriceLegacy(networkID uint64, feedAddress string, priceData *PriceData) {
	pcm.cache.UpdatePriceLegacy(networkID, feedAddress, priceData)
}

// GetPriceLegacy retrieves a price using the old format (assumes Chainlink)
func (pcm *PriceCacheManager) GetPriceLegacy(networkID uint64, feedAddress string) (*PriceData, error) {
	return pcm.cache.GetPriceLegacy(networkID, feedAddress)
}

// GetAllPricesLegacy retrieves all prices using the old format (assumes Chainlink)
func (pcm *PriceCacheManager) GetAllPricesLegacy(networkID uint64) map[string]*PriceData {
	return pcm.cache.GetAllPricesLegacy(networkID)
}

// AddFeedLegacy adds a price feed using the old format (assumes Chainlink)
func (pcm *PriceCacheManager) AddFeedLegacy(networkID uint64, feedAddress string) {
	pcm.cache.AddFeedLegacy(networkID, feedAddress)
}
