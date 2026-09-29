package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Release struct {
	ID        int64  `json:"id"`
	TagName   string `json:"tag_name"`
	UploadURL string `json:"upload_url"`
}

type Asset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CreateReleaseRequest struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func (c *Client) GetReleaseByTag(ctx context.Context, owner, repository, tag string) (*Release, bool, error) {
	endpoint := c.baseURL + repoPath(owner, repository) + "/releases/tags/" + url.PathEscape(tag)
	var release Release
	resp, err := c.do(ctx, http.MethodGet, endpoint, nil, "", &release)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("GitHub release lookup failed: %w", err)
	}
	return &release, true, nil
}

func (c *Client) CreateRelease(ctx context.Context, owner, repository string, request CreateReleaseRequest) (*Release, error) {
	endpoint := c.baseURL + repoPath(owner, repository) + "/releases"
	var release Release
	if _, err := c.jsonRequest(ctx, http.MethodPost, endpoint, request, &release); err != nil {
		return nil, fmt.Errorf("GitHub release creation failed: %w", err)
	}
	return &release, nil
}

func (c *Client) ListAssets(ctx context.Context, owner, repository string, releaseID int64) ([]Asset, error) {
	base := c.baseURL + repoPath(owner, repository) + "/releases/" + strconv.FormatInt(releaseID, 10) + "/assets"
	var all []Asset
	for page := 1; ; page++ {
		endpoint := base + "?per_page=100&page=" + strconv.Itoa(page)
		var assets []Asset
		if _, err := c.do(ctx, http.MethodGet, endpoint, nil, "", &assets); err != nil {
			return nil, fmt.Errorf("GitHub asset listing failed: %w", err)
		}
		all = append(all, assets...)
		if len(assets) < 100 {
			return all, nil
		}
	}
}

func (c *Client) DeleteAsset(ctx context.Context, owner, repository string, assetID int64) error {
	endpoint := c.baseURL + repoPath(owner, repository) + "/releases/assets/" + strconv.FormatInt(assetID, 10)
	if _, err := c.do(ctx, http.MethodDelete, endpoint, nil, "", nil); err != nil {
		return fmt.Errorf("GitHub asset deletion failed: %w", err)
	}
	return nil
}

func (c *Client) UploadAsset(ctx context.Context, uploadURL, name, contentType string, contentLength int64, body io.Reader) (*Asset, error) {
	if before, _, found := strings.Cut(uploadURL, "{"); found {
		uploadURL = before
	}
	parsed, err := url.Parse(uploadURL)
	if err != nil {
		return nil, fmt.Errorf("invalid release upload URL: %w", err)
	}
	query := parsed.Query()
	query.Set("name", name)
	parsed.RawQuery = query.Encode()
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	var asset Asset
	if _, err := c.doWithContentLength(ctx, http.MethodPost, parsed.String(), body, contentType, contentLength, &asset); err != nil {
		return nil, fmt.Errorf("GitHub asset upload failed: %w", err)
	}
	return &asset, nil
}
