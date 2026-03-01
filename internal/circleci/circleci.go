package circleci

import (
	"fmt"
	"net/http"
	"time"

	"monit/internal/config"
)

type Client struct {
	apiToken string
	client   *http.Client
}

func New(cfg config.CircleCI) *Client {
	return &Client{
		apiToken: cfg.APIToken,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) TriggerPipeline(triggerURL string) error {
	if triggerURL == "" {
		return fmt.Errorf("no trigger URL configured")
	}

	req, err := http.NewRequest("POST", triggerURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Circle-Token", c.apiToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("CircleCI API error: %d", resp.StatusCode)
	}

	return nil
}
