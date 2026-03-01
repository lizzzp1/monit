package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"monit/internal/config"
)

type Client struct {
	apiToken string
	client   *http.Client
}

func New(cfg config.GitHub) *Client {
	return &Client{
		apiToken: cfg.APIToken,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

type CreatePRRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
}

type CreatePRResponse struct {
	URL string `json:"html_url"`
}

func (c *Client) CreateRevertPR(owner, repo, baseBranch, originalCommitMsg string) (string, error) {
	if c.apiToken == "" {
		return "", fmt.Errorf("GitHub API token not configured")
	}

	branchName := "revert-" + time.Now().Format("20060102-150405")
	prTitle := "Revert: " + truncate(originalCommitMsg, 50)
	prBody := fmt.Sprintf("This PR reverts changes due to production alert.\n\nOriginal commit: %s", originalCommitMsg)

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)

	body := CreatePRRequest{
		Title: prTitle,
		Body:  prBody,
		Head:  branchName,
		Base:  baseBranch,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var pr CreatePRResponse
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return "", err
	}

	return pr.URL, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
