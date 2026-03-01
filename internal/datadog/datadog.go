package datadog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"monit/internal/config"
)

type Client struct {
	apiKey string
	appKey string
	client *http.Client
}

func New(cfg config.Datadog) *Client {
	return &Client{
		apiKey: cfg.APIKey,
		appKey: cfg.AppKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type LogResponse struct {
	Data []struct {
		Attributes struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"attributes"`
	} `json:"data"`
}

func (c *Client) GetRecentLogs(service string, duration time.Duration) (string, error) {
	if c.apiKey == "" || c.appKey == "" {
		return "", fmt.Errorf("Datadog API key or App key not configured")
	}

	from := time.Now().Add(-duration).Unix()
	query := fmt.Sprintf("service:%s status:error", service)

	url := fmt.Sprintf("https://api.datadoghq.com/api/v2/logs/analytics/aggregate?query[%s]=&from=%d", query, from)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("DD-API-KEY", c.apiKey)
	req.Header.Set("DD-APPLICATION-KEY", c.appKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Datadog API error: %s", string(body))
	}

	var logResp LogResponse
	if err := json.NewDecoder(resp.Body).Decode(&logResp); err != nil {
		return "", err
	}

	if len(logResp.Data) == 0 {
		return "", nil
	}

	var messages string
	for i, log := range logResp.Data {
		if i >= 5 {
			break
		}
		messages += fmt.Sprintf("- %s\n", log.Attributes.Message)
	}

	return messages, nil
}

type LogCountResponse struct {
	Meta struct {
		Page struct {
			TotalCount int `json:"totalCount"`
		} `json:"page"`
	} `json:"meta"`
}

func (c *Client) CountLogs(query string, duration time.Duration) (int, error) {
	if c.apiKey == "" || c.appKey == "" {
		return 0, fmt.Errorf("Datadog API key or App key not configured")
	}

	from := time.Now().Add(-duration).Unix()
	url := fmt.Sprintf("https://api.datadoghq.com/api/v2/logs/events?filter[query]=%s&filter[from]=%d", query, from)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("DD-API-KEY", c.apiKey)
	req.Header.Set("DD-APPLICATION-KEY", c.appKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("Datadog API error: %s", string(body))
	}

	var logResp LogCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&logResp); err != nil {
		return 0, err
	}

	return logResp.Meta.Page.TotalCount, nil
}

func (c *Client) GetMatchingLogs(service, pattern string, duration time.Duration) (string, error) {
	if c.apiKey == "" || c.appKey == "" {
		return "", fmt.Errorf("Datadog API key or App key not configured")
	}

	from := time.Now().Add(-duration).Unix()
	query := fmt.Sprintf("service:%s %s", service, pattern)

	url := fmt.Sprintf("https://api.datadoghq.com/api/v2/logs/events?filter[query]=%s&filter[from]=%d&page[limit]=10", query, from)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("DD-API-KEY", c.apiKey)
	req.Header.Set("DD-APPLICATION-KEY", c.appKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Datadog API error: %s", string(body))
	}

	var logResp struct {
		Data []struct {
			Attributes struct {
				Message string `json:"message"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&logResp); err != nil {
		return "", err
	}

	if len(logResp.Data) == 0 {
		return "", nil
	}

	var messages string
	for _, log := range logResp.Data {
		messages += fmt.Sprintf("- %s\n", log.Attributes.Message)
	}

	return messages, nil
}
