package pyth

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// eventSource implements the EventSource interface for Server-Sent Events
type eventSource struct {
	url     string
	client  *http.Client
	headers map[string]string
	ctx     context.Context
	cancel  context.CancelFunc

	messageHandler func(data string)
	errorHandler   func(err error)

	mu     sync.RWMutex
	closed bool
	conn   *http.Response
	reader *bufio.Reader
}

// NewEventSource creates a new EventSource for Server-Sent Events
func NewEventSource(url string, client *http.Client, headers map[string]string) EventSource {
	ctx, cancel := context.WithCancel(context.Background())

	return &eventSource{
		url:     url,
		client:  client,
		headers: headers,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// OnMessage sets the message handler
func (es *eventSource) OnMessage(handler func(data string)) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.messageHandler = handler
}

// OnError sets the error handler
func (es *eventSource) OnError(handler func(err error)) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.errorHandler = handler
}

// Close closes the EventSource connection
func (es *eventSource) Close() error {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.closed {
		return nil
	}

	es.closed = true
	es.cancel()

	if es.conn != nil {
		es.conn.Body.Close()
	}

	return nil
}

// Start begins the EventSource connection and starts reading events
func (es *eventSource) Start() error {
	es.mu.Lock()
	if es.closed {
		es.mu.Unlock()
		return fmt.Errorf("event source is closed")
	}
	es.mu.Unlock()

	req, err := http.NewRequestWithContext(es.ctx, "GET", es.url, nil)
	if err != nil {
		es.handleError(fmt.Errorf("failed to create request: %w", err))
		return err
	}

	// Set headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for key, value := range es.headers {
		req.Header.Set(key, value)
	}

	resp, err := es.client.Do(req)
	if err != nil {
		es.handleError(fmt.Errorf("failed to connect: %w", err))
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		err := fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
		es.handleError(err)
		return err
	}

	es.mu.Lock()
	es.conn = resp
	es.reader = bufio.NewReader(resp.Body)
	es.mu.Unlock()

	// Start reading events in a goroutine
	go es.readEvents()

	return nil
}

// readEvents reads Server-Sent Events from the connection
func (es *eventSource) readEvents() {
	defer func() {
		es.mu.Lock()
		if es.conn != nil {
			es.conn.Body.Close()
		}
		es.mu.Unlock()
	}()

	for {
		select {
		case <-es.ctx.Done():
			return
		default:
		}

		es.mu.RLock()
		reader := es.reader
		es.mu.RUnlock()

		if reader == nil {
			return
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			es.handleError(fmt.Errorf("failed to read line: %w", err))
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse Server-Sent Events format
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			es.handleMessage(data)
		} else if strings.HasPrefix(line, "event: ") {
			// Handle event type if needed
			continue
		} else if strings.HasPrefix(line, "id: ") {
			// Handle event ID if needed
			continue
		} else if strings.HasPrefix(line, "retry: ") {
			// Handle retry interval if needed
			continue
		}
	}
}

// handleMessage calls the message handler if set
func (es *eventSource) handleMessage(data string) {
	es.mu.RLock()
	handler := es.messageHandler
	es.mu.RUnlock()

	if handler != nil {
		handler(data)
	}
}

// handleError calls the error handler if set
func (es *eventSource) handleError(err error) {
	es.mu.RLock()
	handler := es.errorHandler
	es.mu.RUnlock()

	if handler != nil {
		handler(err)
	}
}

// GetPriceUpdatesStream fetches streaming price updates for a set of price feed IDs
func (c *HermesClient) GetPriceUpdatesStream(ctx context.Context, ids []HexString, options *GetPriceUpdatesStreamOptions) (EventSource, error) {
	u := c.buildURL("updates/price/stream")

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
		if options.AllowUnordered != nil {
			params["allow_unordered"] = *options.AllowUnordered
		}
		if options.BenchmarksOnly != nil {
			params["benchmarks_only"] = *options.BenchmarksOnly
		}
		if options.IgnoreInvalidPriceIds != nil {
			params["ignore_invalid_price_ids"] = *options.IgnoreInvalidPriceIds
		}
		c.appendURLSearchParams(u, params)
	}

	// Create a custom HTTP client for streaming with no timeout
	streamClient := &http.Client{
		Timeout: 0, // No timeout for streaming
	}

	es := NewEventSource(u.String(), streamClient, c.headers)

	// Start the connection
	if err := es.(*eventSource).Start(); err != nil {
		return nil, err
	}

	return es, nil
}
