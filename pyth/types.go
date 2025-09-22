package pyth

import (
	"encoding/json"
	"time"
)

// AssetType represents the type of asset for price feeds
type AssetType string

const (
	AssetTypeCrypto               AssetType = "crypto"
	AssetTypeEquity               AssetType = "equity"
	AssetTypeFX                   AssetType = "fx"
	AssetTypeMetal                AssetType = "metal"
	AssetTypeRates                AssetType = "rates"
	AssetTypeCryptoRedemptionRate AssetType = "crypto_redemption_rate"
)

// EncodingType represents the encoding format for binary data
type EncodingType string

const (
	EncodingTypeHex    EncodingType = "hex"
	EncodingTypeBase64 EncodingType = "base64"
)

// UnixTimestamp represents a Unix timestamp in seconds
type UnixTimestamp int64

// DurationInSeconds represents a duration in seconds
type DurationInSeconds int64

// DurationInMs represents a duration in milliseconds
type DurationInMs int64

// HexString represents a hex-encoded string
type HexString string

// PriceFeedMetadata represents metadata for a price feed
type PriceFeedMetadata struct {
	ID            string    `json:"id"`
	Symbol        string    `json:"symbol"`
	AssetType     AssetType `json:"asset_type"`
	Description   string    `json:"description"`
	MinPublishers int       `json:"min_publishers"`
	Decimals      int       `json:"decimals"`
	Status        string    `json:"status"`
	LastUpdated   time.Time `json:"last_updated"`
}

// BinaryPriceUpdate represents a binary price update
type BinaryPriceUpdate struct {
	Type     string `json:"type"`
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

// PriceUpdate represents a price update response
type PriceUpdate struct {
	Type     string             `json:"type"`
	Encoding string             `json:"encoding"`
	Data     string             `json:"data"`
	Parsed   *ParsedPriceUpdate `json:"parsed,omitempty"`
}

// ParsedPriceUpdate represents parsed price update data
type ParsedPriceUpdate struct {
	PriceFeeds []PriceFeed `json:"price_feeds"`
}

// PriceFeed represents a single price feed in a price update
type PriceFeed struct {
	ID          string `json:"id"`
	Price       Price  `json:"price"`
	Ema         Ema    `json:"ema"`
	Conf        string `json:"conf"`
	PublishSlot int64  `json:"publish_slot"`
	PrevSlot    int64  `json:"prev_slot"`
	PrevPrice   Price  `json:"prev_price"`
	PrevConf    string `json:"prev_conf"`
	PrevEma     Ema    `json:"prev_ema"`
	PrevEmaConf string `json:"prev_ema_conf"`
}

// Price represents price information
type Price struct {
	Price       string `json:"price"`
	Conf        string `json:"conf"`
	Expo        int    `json:"expo"`
	PublishTime int64  `json:"publish_time"`
}

// Ema represents exponential moving average information
type Ema struct {
	Price       string `json:"price"`
	Conf        string `json:"conf"`
	Expo        int    `json:"expo"`
	PublishTime int64  `json:"publish_time"`
}

// TwapsResponse represents TWAP (Time Weighted Average Price) response
type TwapsResponse struct {
	Type     string             `json:"type"`
	Encoding string             `json:"encoding"`
	Data     string             `json:"data"`
	Parsed   *ParsedTwapsUpdate `json:"parsed,omitempty"`
}

// ParsedTwapsUpdate represents parsed TWAP update data
type ParsedTwapsUpdate struct {
	Twaps []Twap `json:"twaps"`
}

// Twap represents a single TWAP entry
type Twap struct {
	ID          string `json:"id"`
	Price       Price  `json:"price"`
	Ema         Ema    `json:"ema"`
	Conf        string `json:"conf"`
	PublishSlot int64  `json:"publish_slot"`
	PrevSlot    int64  `json:"prev_slot"`
	PrevPrice   Price  `json:"prev_price"`
	PrevConf    string `json:"prev_conf"`
	PrevEma     Ema    `json:"prev_ema"`
	PrevEmaConf string `json:"prev_ema_conf"`
}

// PublisherCaps represents publisher stake caps data
type PublisherCaps struct {
	Type     string                     `json:"type"`
	Encoding string                     `json:"encoding"`
	Data     string                     `json:"data"`
	Parsed   *ParsedPublisherCapsUpdate `json:"parsed,omitempty"`
}

// ParsedPublisherCapsUpdate represents parsed publisher caps data
type ParsedPublisherCapsUpdate struct {
	PublisherStakeCaps []PublisherStakeCap `json:"publisher_stake_caps"`
}

// PublisherStakeCap represents a single publisher stake cap
type PublisherStakeCap struct {
	Publisher string `json:"publisher"`
	Cap       string `json:"cap"`
}

// HermesClientConfig represents configuration for the Hermes client
type HermesClientConfig struct {
	// Timeout of each request (for all of retries). Default: 5000ms
	Timeout *DurationInMs `json:"timeout,omitempty"`
	// Number of times a HTTP request will be retried before the API returns a failure. Default: 3.
	// The connection uses exponential back-off for the delay between retries. However,
	// it will timeout regardless of the retries at the configured timeout time.
	HTTPRetries *int `json:"http_retries,omitempty"`
	// Optional headers to be included in every request.
	Headers map[string]string `json:"headers,omitempty"`
}

// GetPriceFeedsOptions represents options for getting price feeds
type GetPriceFeedsOptions struct {
	Query     *string    `json:"query,omitempty"`
	AssetType *AssetType `json:"asset_type,omitempty"`
}

// GetLatestPriceUpdatesOptions represents options for getting latest price updates
type GetLatestPriceUpdatesOptions struct {
	Encoding              *EncodingType `json:"encoding,omitempty"`
	Parsed                *bool         `json:"parsed,omitempty"`
	IgnoreInvalidPriceIds *bool         `json:"ignore_invalid_price_ids,omitempty"`
}

// GetPriceUpdatesAtTimestampOptions represents options for getting price updates at timestamp
type GetPriceUpdatesAtTimestampOptions struct {
	Encoding              *EncodingType `json:"encoding,omitempty"`
	Parsed                *bool         `json:"parsed,omitempty"`
	IgnoreInvalidPriceIds *bool         `json:"ignore_invalid_price_ids,omitempty"`
}

// GetPriceUpdatesStreamOptions represents options for streaming price updates
type GetPriceUpdatesStreamOptions struct {
	Encoding              *EncodingType `json:"encoding,omitempty"`
	Parsed                *bool         `json:"parsed,omitempty"`
	AllowUnordered        *bool         `json:"allow_unordered,omitempty"`
	BenchmarksOnly        *bool         `json:"benchmarks_only,omitempty"`
	IgnoreInvalidPriceIds *bool         `json:"ignore_invalid_price_ids,omitempty"`
}

// GetLatestTwapsOptions represents options for getting latest TWAPs
type GetLatestTwapsOptions struct {
	Encoding              *EncodingType `json:"encoding,omitempty"`
	Parsed                *bool         `json:"parsed,omitempty"`
	IgnoreInvalidPriceIds *bool         `json:"ignore_invalid_price_ids,omitempty"`
}

// GetLatestPublisherCapsOptions represents options for getting latest publisher caps
type GetLatestPublisherCapsOptions struct {
	Encoding *EncodingType `json:"encoding,omitempty"`
	Parsed   *bool         `json:"parsed,omitempty"`
}

// EventSource represents a Server-Sent Events connection
type EventSource interface {
	OnMessage(handler func(data string))
	OnError(handler func(err error))
	Close() error
}

// Default constants
const (
	DefaultTimeout     DurationInMs = 5000
	DefaultHTTPRetries int          = 3
)

// Helper function to convert camelCase to snake_case
func camelToSnakeCase(str string) string {
	result := ""
	for i, r := range str {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result += "_"
		}
		result += string(r)
	}
	return result
}

// Helper function to convert camelCase object keys to snake_case
func camelToSnakeCaseObject(obj map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range obj {
		snakeKey := camelToSnakeCase(key)
		result[snakeKey] = value
	}
	return result
}

// Helper function to convert struct to map for query parameters
func structToMap(obj interface{}) map[string]interface{} {
	data, _ := json.Marshal(obj)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}
