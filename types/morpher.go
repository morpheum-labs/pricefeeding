package types

import (
	"fmt"
	"math/big"
	"time"

	"github.com/morpheum-labs/safem"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PriceSource identifies the data provider
type PriceSource string

const (
	SourceChainlink PriceSource = "chainlink"
	SourcePyth      PriceSource = "pyth"
)
const (
	OracleNetworkIDPyth      = 0
	OracleNetworkIDChainlink = 1
	OracleNetworkIDArbitrum  = 42161
	OracleNetworkIDPolygon   = 137
	OracleNetworkIDBSC       = 56
	OracleNetworkIDOptimism  = 10
	OracleNetworkIDFantom    = 250
	OracleNetworkIDAvalanche = 43114
)

// PriceInfo is an interface for price data from any source
type PriceInfo interface {
	GetSource() PriceSource    // Returns the source (e.g., "chainlink" or "pyth")
	GetNetworkID() uint64      // Returns the network ID
	GetTimestamp() time.Time   // Returns the timestamp
	GetPrice() (*big.Int, int) // Returns the raw price and exponent
	GetIdentifier() string     // Returns the identifier (feedAddress for Chainlink, ID for Pyth)
	GetUint64SatoshiPrice() uint64 // Returns the price in satoshi format as uint64 (convenience method)
	GetPriceInSatoshi() (*big.Int, error) // Returns the price in satoshi format (1e8), adjusted by the exponent
}

// ChainlinkPrice implements PriceInfo for Chainlink data
type ChainlinkPrice struct {
	RoundID         *big.Int
	Answer          *big.Int
	StartedAt       *big.Int
	UpdatedAt       *big.Int
	AnsweredInRound *big.Int
	Timestamp       time.Time
	Exponent        int
	NetworkID       uint64
	FeedAddress     string // Store the feed address for identifier
}

func (p *ChainlinkPrice) GetSource() PriceSource {
	return SourceChainlink
}

func (p *ChainlinkPrice) GetNetworkID() uint64 {
	return p.NetworkID
}

func (p *ChainlinkPrice) GetTimestamp() time.Time {
	return p.Timestamp
}

func (p *ChainlinkPrice) GetPrice() (*big.Int, int) {
	return p.Answer, p.Exponent
}

func (p *ChainlinkPrice) GetIdentifier() string {
	return p.FeedAddress
}

// GetPriceInSatoshi returns the price in satoshi format (1e8), adjusted by the exponent
//
// PURPOSE: Convert Chainlink price format (Answer big.Int + exponent) to satoshi-based uint64
// USAGE: Converting oracle prices to internal satoshi format for orderbook/matching
// CRITICAL: Answer is stored as big.Int, exponent adjusts decimal position
// Formula: actual_price = Answer * 10^exponent, then satoshi_price = actual_price * SatoshiScale
// Simplified: satoshi_price = Answer * 10^exponent * SatoshiScale
//
// Example:
//   - Answer: 5000000000, Exponent: -8 → Actual: 50.0 → Satoshi: 5000000000
//   - Answer: 100000000, Exponent: -8 → Actual: 1.0 → Satoshi: 100000000
//   - Answer: 5000000000000, Exponent: -8 → Actual: 50000.0 → Satoshi: 5000000000000
func (p *ChainlinkPrice) GetPriceInSatoshi() (*big.Int, error) {
	if p.Answer == nil {
		return nil, fmt.Errorf("Answer is nil")
	}

	// Calculate adjustment factor: 10^exponent * SatoshiScale
	// Exponent adjusts the decimal position, SatoshiScale converts to satoshi format
	exponentFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(p.Exponent)), nil)
	satoshiScaleBig := big.NewInt(int64(safem.SatoshiScale))
	adjustment := new(big.Int).Mul(exponentFactor, satoshiScaleBig)

	// Multiply Answer by adjustment to get satoshi value
	result := new(big.Int).Mul(p.Answer, adjustment)

	return result, nil
}

// GetUint64SatoshiPrice returns the price in satoshi format as uint64
// This is a convenience method that calls GetPriceInSatoshi() and converts to uint64
// Note: This will panic if the price exceeds uint64 max value
func (p *ChainlinkPrice) GetUint64SatoshiPrice() uint64 {
	priceInSatoshi, _ := p.GetPriceInSatoshi()
	return priceInSatoshi.Uint64()
}

// PythPrice implements PriceInfo for Pyth data
type PythPrice struct {
	ID            string    `json:"id"`
	Symbol        string    `json:"symbol,omitempty"`
	Price         *big.Int  `json:"price"`
	Confidence    *big.Int  `json:"confidence"`
	Exponent      int       `json:"exponent"`
	PublishTime   int64     `json:"publish_time"`
	Slot          int64     `json:"slot"`
	Timestamp     time.Time `json:"timestamp"`
	NetworkID     uint64    `json:"network_id"`
	EMA           *big.Int  `json:"ema,omitempty"`
	EMAConfidence *big.Int  `json:"ema_confidence,omitempty"`
}

func (p *PythPrice) GetSource() PriceSource {
	return SourcePyth
}

func (p *PythPrice) GetNetworkID() uint64 {
	return p.NetworkID
}

func (p *PythPrice) GetTimestamp() time.Time {
	return p.Timestamp
}

func (p *PythPrice) GetPrice() (*big.Int, int) {
	return p.Price, p.Exponent
}

func (p *PythPrice) GetIdentifier() string {
	return p.ID
}

// PythPriceData represents price data from Pyth Network
// This is a morphcore-specific type used in the oracle adapter
type PythPriceData struct {
	PriceID     string                 `json:"price_id"`
	Symbol      string                 `json:"symbol"`
	Price       string                 `json:"price"`
	Confidence  string                 `json:"confidence"`
	Exponent    int                    `json:"exponent"`
	PublishTime *timestamppb.Timestamp `json:"publish_time"`
	Source      string                 `json:"source"`
	Staleness   time.Duration
}

// GetPriceInSatoshi returns the price in satoshi format (1e8), adjusted by the exponent
//
// PURPOSE: Convert Pyth price format (price string + exponent) to satoshi-based uint64
// USAGE: Converting oracle prices to internal satoshi format for orderbook/matching
// CRITICAL: Price is stored as string, exponent adjusts decimal position
// Formula: actual_price = price * 10^exponent, then satoshi_price = actual_price * SatoshiScale
// Simplified: satoshi_price = price * 10^exponent * SatoshiScale
//
// Example:
//   - Price: "5000000000", Exponent: -8 → Actual: 50.0 → Satoshi: 5000000000
//   - Price: "100000000", Exponent: -8 → Actual: 1.0 → Satoshi: 100000000
//   - Price: "5000000000000", Exponent: -8 → Actual: 50000.0 → Satoshi: 5000000000000
func (p *PythPriceData) GetPriceInSatoshi() (*big.Int, error) {
	// Parse price string to big.Int
	priceInt, err := safem.BigIntByString(p.Price)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price string %s: %w", p.Price, err)
	}

	// Calculate adjustment factor: 10^exponent * SatoshiScale
	// Exponent adjusts the decimal position, SatoshiScale converts to satoshi format
	exponentFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(p.Exponent)), nil)
	satoshiScaleBig := big.NewInt(int64(safem.SatoshiScale))
	adjustment := new(big.Int).Mul(exponentFactor, satoshiScaleBig)

	// Multiply price by adjustment to get satoshi value
	result := new(big.Int).Mul(priceInt, adjustment)

	return result, nil
}

// GetUint64SatoshiPrice returns the price in satoshi format as uint64
// This is a convenience method that calls GetPriceInSatoshi() and converts to uint64
// Note: This will panic if the price exceeds uint64 max value
func (p *PythPriceData) GetUint64SatoshiPrice() uint64 {
	priceInSatoshi, _ := p.GetPriceInSatoshi()
	return priceInSatoshi.Uint64()
}

// API responses
// PythHermesResponse represents the response from Pyth Hermes API
type PythHermesResponse struct {
	Binary     PythBinary                `json:"binary"`
	ParsedData []PythPriceFeedDataV2rest `json:"parsed"`
}

// PythPriceFeedData represents individual price feed data from Pyth
type PythBinary struct {
	Encoding string   `json:"encoding"`
	Data     []string `json:"data"`
}

type PythPriceFeedDataV2rest struct {
	FeedId   string      `json:"id"`
	Price    PriceVector `json:"price"`
	EmaPrice PriceVector `json:"ema_price"`
}

type PythPriceFeedDataV2Ws struct {
	FeedId   string      `json:"id"`
	Price    PriceVector `json:"price"`
	EmaPrice PriceVector `json:"ema_price"`
}

type PriceVector struct {
	Price       string `json:"price"`
	Confidence  string `json:"conf"`
	Exponent    int    `json:"expo"`
	PublishTime uint64 `json:"publish_time"`
}

// PythWebSocketMessage represents WebSocket message from Pyth
type PythWebSocketMessage struct {
	Type string                `json:"type"`
	Data PythPriceFeedDataV2Ws `json:"price_feed"`
}
