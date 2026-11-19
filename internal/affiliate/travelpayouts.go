package affiliate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LinkItem represents a single URL to convert to affiliate
type LinkItem struct {
	URL   string `json:"url"`
	SubID string `json:"sub_id,omitempty"`
}

// CreateLinksRequest is the payload for /links/v1/create
type CreateLinksRequest struct {
	TRS    int        `json:"trs"`
	Marker int        `json:"marker"`
	Links  []LinkItem `json:"links"`
}

// CreateLinksResponse is the response from /links/v1/create
type CreateLinksResponse struct {
	Result struct {
		Links []struct {
			URL        string `json:"url"`
			PartnerURL string `json:"partner_url"`
			Code       string `json:"code"`
		} `json:"links"`
	} `json:"result"`
	Success bool `json:"success"`
}

// Client handles Travelpayouts affiliate link generation
type Client struct {
	apiToken   string
	trs        int
	marker     int
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Travelpayouts affiliate client
func NewClient(apiToken string, trs, marker int) *Client {
	return &Client{
		apiToken: apiToken,
		trs:      trs,
		marker:   marker,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: "https://travelpayouts.com/api/v1",
	}
}

// GeneratePartnerLinks converts up to 10 URLs to affiliate links in one batch
// Returns a map from original URL to partner URL
func (c *Client) GeneratePartnerLinks(ctx context.Context, items []LinkItem) (map[string]string, error) {
	if len(items) == 0 {
		return nil, nil
	}

	// Batch into chunks of 10 (API limit)
	result := make(map[string]string)
	for i := 0; i < len(items); i += 10 {
		end := i + 10
		if end > len(items) {
			end = len(items)
		}
		chunk := items[i:end]

		err := c.generateBatch(ctx, chunk, result)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func (c *Client) generateBatch(ctx context.Context, items []LinkItem, result map[string]string) error {
	reqBody := CreateLinksRequest{
		TRS:    c.trs,
		Marker: c.marker,
		Links:  items,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/links", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Access-Token", c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var parsed CreateLinksResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	for _, l := range parsed.Result.Links {
		if l.Code == "success" {
			result[l.URL] = l.PartnerURL
		}
	}

	return nil
}
