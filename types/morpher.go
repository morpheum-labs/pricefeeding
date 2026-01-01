package types

import (
	"math/big"
	"time"
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
)

// PriceInfo is an interface for price data from any source
type PriceInfo interface {
	GetSource() PriceSource  // Returns the source (e.g., "chainlink" or "pyth")
	GetNetworkID() uint64    // Returns the network ID
	GetTimestamp() time.Time // Returns the timestamp
	GetPrice() *big.Int      // Returns the raw price
	GetIdentifier() string   // Returns the identifier (feedAddress for Chainlink, ID for Pyth)
}

// ChainlinkPrice implements PriceInfo for Chainlink data
type ChainlinkPrice struct {
	RoundID         *big.Int
	Answer          *big.Int
	StartedAt       *big.Int
	UpdatedAt       *big.Int
	AnsweredInRound *big.Int
	Timestamp       time.Time
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

func (p *ChainlinkPrice) GetPrice() *big.Int {
	return p.Answer
}

func (p *ChainlinkPrice) GetIdentifier() string {
	return p.FeedAddress
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

func (p *PythPrice) GetPrice() *big.Int {
	return p.Price
}

func (p *PythPrice) GetIdentifier() string {
	return p.ID
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
