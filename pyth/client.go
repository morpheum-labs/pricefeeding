package pyth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HermesClient represents a client for interacting with the Pyth Hermes service
type HermesClient struct {
	baseURL     string
	timeout     DurationInMs
	httpRetries int
	headers     map[string]string
	httpClient  *http.Client
}

// NewHermesClient creates a new Hermes client
func NewHermesClient(endpoint string, config *HermesClientConfig) *HermesClient {
	timeout := DefaultTimeout
	if config != nil && config.Timeout != nil {
		timeout = *config.Timeout
	}

	httpRetries := DefaultHTTPRetries
	if config != nil && config.HTTPRetries != nil {
		httpRetries = *config.HTTPRetries
	}

	headers := make(map[string]string)
	if config != nil && config.Headers != nil {
		headers = config.Headers
	}

	httpClient := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
	}

	return &HermesClient{
		baseURL:     endpoint,
		timeout:     timeout,
		httpRetries: httpRetries,
		headers:     headers,
		httpClient:  httpClient,
	}
}

// httpRequest performs an HTTP request with retry logic and exponential backoff
func (c *HermesClient) httpRequest(ctx context.Context, method, url string, body io.Reader, result interface{}) error {
	var lastErr error

	// Adding randomness to the initial backoff to avoid "thundering herd" scenario
	backoff := 100 + rand.Intn(100)

	for attempt := 0; attempt <= c.httpRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Add headers
		for key, value := range c.headers {
			req.Header.Set(key, value)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			if attempt < c.httpRetries {
				// Wait for backoff period before retrying
				time.Sleep(time.Duration(backoff) * time.Millisecond)
				backoff *= 2 // Exponential backoff
				continue
			}
			return lastErr
		}

		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("HTTP error! status: %d, body: %s", resp.StatusCode, string(bodyBytes))
			if attempt < c.httpRetries {
				time.Sleep(time.Duration(backoff) * time.Millisecond)
				backoff *= 2
				continue
			}
			return lastErr
		}

		// Parse response
		if result != nil {
			if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}
		}

		return nil
	}

	return lastErr
}

// buildURL constructs a URL for the given endpoint
func (c *HermesClient) buildURL(endpoint string) *url.URL {
	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	u, _ := url.Parse(baseURL)
	u.Path = strings.TrimSuffix(u.Path, "/") + endpoint
	return u
}

// appendURLSearchParams adds query parameters to a URL
func (c *HermesClient) appendURLSearchParams(u *url.URL, params map[string]interface{}) {
	query := u.Query()
	for key, value := range params {
		if value != nil {
			switch v := value.(type) {
			case string:
				if v != "" {
					query.Add(key, v)
				}
			case bool:
				query.Add(key, strconv.FormatBool(v))
			case int:
				query.Add(key, strconv.Itoa(v))
			case int64:
				query.Add(key, strconv.FormatInt(v, 10))
			case float64:
				query.Add(key, strconv.FormatFloat(v, 'f', -1, 64))
			case []string:
				for _, item := range v {
					query.Add(key+"[]", item)
				}
			}
		}
	}
	u.RawQuery = query.Encode()
}

// GetPriceFeeds fetches the set of available price feeds
func (c *HermesClient) GetPriceFeeds(ctx context.Context, options *GetPriceFeedsOptions) ([]PriceFeedMetadata, error) {
	u := c.buildURL("v2/priceFeeds")

	if options != nil {
		params := make(map[string]interface{})
		if options.Query != nil {
			params["query"] = *options.Query
		}
		if options.AssetType != nil {
			params["assetType"] = string(*options.AssetType)
		}
		c.appendURLSearchParams(u, params)
	}

	var result []PriceFeedMetadata
	err := c.httpRequest(ctx, "GET", u.String(), nil, &result)
	return result, err
}

// GetLatestPriceUpdates fetches the latest price updates for a set of price feed IDs
func (c *HermesClient) GetLatestPriceUpdates(ctx context.Context, ids []HexString, options *GetLatestPriceUpdatesOptions) (*PriceUpdate, error) {
	u := c.buildURL("v2/updates/price/latest")

	// Add price IDs as query parameters
	query := u.Query()
	for _, id := range ids {
		query.Add("ids[]", string(id))
	}
	u.RawQuery = query.Encode()

	if options != nil {
		params := make(map[string]interface{})
		if options.Encoding != nil {
			params["encoding"] = string(*options.Encoding)
		}
		if options.Parsed != nil {
			params["parsed"] = *options.Parsed
		}
		if options.IgnoreInvalidPriceIds != nil {
			params["ignore_invalidPriceIds"] = *options.IgnoreInvalidPriceIds
		}
		c.appendURLSearchParams(u, params)
	}

	var result PriceUpdate
	err := c.httpRequest(ctx, "GET", u.String(), nil, &result)
	return &result, err
}

// GetPriceUpdatesAtTimestamp fetches price updates for a set of price feed IDs at a given timestamp
func (c *HermesClient) GetPriceUpdatesAtTimestamp(ctx context.Context, publishTime UnixTimestamp, ids []HexString, options *GetPriceUpdatesAtTimestampOptions) (*PriceUpdate, error) {
	u := c.buildURL(fmt.Sprintf("v2/updates/price/%d", publishTime))

	// Add price IDs as query parameters
	query := u.Query()
	for _, id := range ids {
		query.Add("ids[]", string(id))
	}
	u.RawQuery = query.Encode()

	if options != nil {
		params := make(map[string]interface{})
		if options.Encoding != nil {
			params["encoding"] = string(*options.Encoding)
		}
		if options.Parsed != nil {
			params["parsed"] = *options.Parsed
		}
		if options.IgnoreInvalidPriceIds != nil {
			params["ignore_invalidPriceIds"] = *options.IgnoreInvalidPriceIds
		}
		c.appendURLSearchParams(u, params)
	}

	var result PriceUpdate
	err := c.httpRequest(ctx, "GET", u.String(), nil, &result)
	return &result, err
}

// GetLatestTwaps fetches the latest TWAP (time weighted average price) for a set of price feed IDs
func (c *HermesClient) GetLatestTwaps(ctx context.Context, ids []HexString, windowSeconds int, options *GetLatestTwapsOptions) (*TwapsResponse, error) {
	u := c.buildURL(fmt.Sprintf("v2/updates/twap/%d/latest", windowSeconds))

	// Add price IDs as query parameters
	query := u.Query()
	for _, id := range ids {
		query.Add("ids[]", string(id))
	}
	u.RawQuery = query.Encode()

	if options != nil {
		params := make(map[string]interface{})
		if options.Encoding != nil {
			params["encoding"] = string(*options.Encoding)
		}
		if options.Parsed != nil {
			params["parsed"] = *options.Parsed
		}
		if options.IgnoreInvalidPriceIds != nil {
			params["ignore_invalidPriceIds"] = *options.IgnoreInvalidPriceIds
		}
		c.appendURLSearchParams(u, params)
	}

	var result TwapsResponse
	err := c.httpRequest(ctx, "GET", u.String(), nil, &result)
	return &result, err
}

// GetLatestPublisherCaps fetches the latest publisher stake caps
func (c *HermesClient) GetLatestPublisherCaps(ctx context.Context, options *GetLatestPublisherCapsOptions) (*PublisherCaps, error) {
	u := c.buildURL("v2/updates/publisher_stake_caps/latest")

	if options != nil {
		params := make(map[string]interface{})
		if options.Encoding != nil {
			params["encoding"] = string(*options.Encoding)
		}
		if options.Parsed != nil {
			params["parsed"] = *options.Parsed
		}
		c.appendURLSearchParams(u, params)
	}

	var result PublisherCaps
	err := c.httpRequest(ctx, "GET", u.String(), nil, &result)
	return &result, err
}
