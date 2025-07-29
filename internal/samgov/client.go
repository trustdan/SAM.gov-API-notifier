package samgov

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.sam.gov/opportunities/v2/search"
	DefaultTimeout = 30 * time.Second
	UserAgent      = "SAM.gov-Monitor/1.0"
)

// Client represents a SAM.gov API client
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new SAM.gov API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// NewClientWithOptions creates a client with custom options
func NewClientWithOptions(apiKey, baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Search executes a search query against the SAM.gov API with retry logic
func (c *Client) Search(ctx context.Context, params map[string]string) (*SearchResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Retry configuration
	maxRetries := 3
	baseDelay := 5 * time.Second // Increased base delay for rate limits
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Build URL with parameters
		u, err := url.Parse(c.baseURL)
		if err != nil {
			return nil, fmt.Errorf("parsing base URL: %w", err)
		}

		q := u.Query()
		q.Set("api_key", c.apiKey)

		// Add search parameters
		for key, value := range params {
			if value != "" {
				q.Set(key, value)
			}
		}
		u.RawQuery = q.Encode()

		// Create request
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Accept", "application/json")

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt < maxRetries && IsRetryableError(err) {
				delay := time.Duration(1<<attempt) * baseDelay
				time.Sleep(delay)
				continue
			}
			return nil, fmt.Errorf("executing request: %w", err)
		}

		// Check status code
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			
			apiErr := &APIError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("API returned status %d", resp.StatusCode),
				Details:    resp.Status,
			}
			
			// Retry on rate limit errors with exponential backoff
			if resp.StatusCode == 429 && attempt < maxRetries {
				delay := time.Duration(1<<attempt) * baseDelay * 2 // Longer delay for rate limits
				log.Printf("Received 429 rate limit error, retrying in %v (attempt %d/%d)", delay, attempt+1, maxRetries)
				time.Sleep(delay)
				continue
			}
			
			return nil, apiErr
		}

		// Parse response
		var result SearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		resp.Body.Close()

		return &result, nil
	}
	
	return nil, fmt.Errorf("max retries exceeded")
}

// SearchWithDefaults executes a search with common default parameters
func (c *Client) SearchWithDefaults(ctx context.Context, customParams map[string]string, lookbackDays int) (*SearchResponse, error) {
	params := make(map[string]string)

	// Set default date range
	to := time.Now()
	from := to.AddDate(0, 0, -lookbackDays)
	params["postedFrom"] = from.Format("01/02/2006")
	params["postedTo"] = to.Format("01/02/2006")

	// Set default pagination
	params["limit"] = "100"
	params["offset"] = "0"

	// Merge custom parameters (they override defaults)
	for key, value := range customParams {
		params[key] = value
	}

	return c.Search(ctx, params)
}

// BuildSearchParams converts a query configuration to API parameters
func BuildSearchParams(queryParams map[string]interface{}, lookbackDays int) map[string]string {
	params := make(map[string]string)

	// Set date range
	to := time.Now()
	from := to.AddDate(0, 0, -lookbackDays)
	params["postedFrom"] = from.Format("01/02/2006")
	params["postedTo"] = to.Format("01/02/2006")

	// Convert query parameters
	for key, value := range queryParams {
		switch v := value.(type) {
		case string:
			if v != "" {
				params[key] = v
			}
		case []interface{}:
			// Handle arrays (e.g., multiple ptypes)
			if len(v) > 0 {
				// For now, take the first value
				// TODO: Handle multiple values properly
				if str, ok := v[0].(string); ok {
					params[key] = str
				}
			}
		case []string:
			// Handle string arrays
			if len(v) > 0 {
				params[key] = strings.Join(v, ",")
			}
		case int:
			params[key] = fmt.Sprintf("%d", v)
		case float64:
			params[key] = fmt.Sprintf("%.0f", v)
		}
	}

	// Set pagination defaults if not specified
	if params["limit"] == "" {
		params["limit"] = "100"
	}
	if params["offset"] == "" {
		params["offset"] = "0"
	}

	return params
}

// ValidateAPIKey checks if the API key works by making a test request
func (c *Client) ValidateAPIKey(ctx context.Context) error {
	params := map[string]string{
		"limit":      "1",
		"postedFrom": time.Now().AddDate(0, 0, -1).Format("01/02/2006"),
		"postedTo":   time.Now().Format("01/02/2006"),
	}

	_, err := c.Search(ctx, params)
	return err
}

// IsRetryableError determines if an error should trigger a retry
func IsRetryableError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		// Retry on server errors and rate limits
		return apiErr.StatusCode >= 500 || apiErr.StatusCode == 429
	}
	
	// Retry on network errors
	if strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "connection") ||
		strings.Contains(err.Error(), "network") {
		return true
	}

	return false
}