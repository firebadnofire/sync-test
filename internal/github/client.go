package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultBaseURL = "https://api.github.com"

type Client struct {
	baseURL    string
	token      string
	isFine     bool
	httpClient *http.Client
}

func NewClient(token string, isFine bool, baseURL string, httpClient *http.Client) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, isFine: isFine, httpClient: httpClient}
}

type apiError struct {
	Message string `json:"message"`
}

func (c *Client) do(ctx context.Context, method, endpoint string, body io.Reader, contentType string, out any) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	authScheme := "token"
	if c.isFine {
		authScheme = "Bearer"
	}
	req.Header.Set("Authorization", authScheme+" "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "actions/gh-sync")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited := io.LimitReader(resp.Body, 1<<20)
		data, _ := io.ReadAll(limited)
		var apiErr apiError
		message := strings.TrimSpace(string(data))
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Message != "" {
			message = apiErr.Message
		}
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		if c.token != "" {
			message = strings.ReplaceAll(message, c.token, "***")
		}
		return resp, fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp, fmt.Errorf("decode response: %w", err)
		}
	}
	return resp, nil
}

func (c *Client) jsonRequest(ctx context.Context, method, endpoint string, value, out any) (*http.Response, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, method, endpoint, bytes.NewReader(data), "application/json", out)
}

func repoPath(owner, repository string) string {
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repository)
}
