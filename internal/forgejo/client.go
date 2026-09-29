package forgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(token, baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, httpClient: httpClient}
}

type apiError struct {
	Message string `json:"message"`
}

func (c *Client) do(ctx context.Context, method, endpoint string, out any) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, c.responseError(resp)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp, fmt.Errorf("decode response: %w", err)
		}
	}
	return resp, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "actions/gh-sync")
}

func (c *Client) responseError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	message := strings.TrimSpace(string(data))
	var apiErr apiError
	if json.Unmarshal(data, &apiErr) == nil && apiErr.Message != "" {
		message = apiErr.Message
	}
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	if c.token != "" {
		message = strings.ReplaceAll(message, c.token, "***")
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
}

func (c *Client) repoPath(owner, repository string) string {
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repository)
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}
