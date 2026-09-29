package forgejo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type Release struct {
	ID         int64  `json:"id"`
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

type Asset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (c *Client) GetReleaseByTag(ctx context.Context, owner, repository, tag string) (*Release, error) {
	endpoint := c.baseURL + c.repoPath(owner, repository) + "/releases/tags/" + url.PathEscape(tag)
	var release Release
	resp, err := c.do(ctx, http.MethodGet, endpoint, &release)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("Forgejo source release %q was not found", tag)
		}
		return nil, fmt.Errorf("Forgejo release lookup failed: %w", err)
	}
	if release.ID == 0 || release.TagName == "" {
		return nil, fmt.Errorf("Forgejo release lookup returned malformed release metadata")
	}
	return &release, nil
}

func (c *Client) ListAssets(ctx context.Context, owner, repository string, releaseID int64) ([]Asset, error) {
	base := c.baseURL + c.repoPath(owner, repository) + "/releases/" + strconv.FormatInt(releaseID, 10) + "/assets"
	var all []Asset
	for page := 1; ; page++ {
		endpoint := base + "?limit=50&page=" + strconv.Itoa(page)
		var assets []Asset
		if _, err := c.do(ctx, http.MethodGet, endpoint, &assets); err != nil {
			return nil, fmt.Errorf("Forgejo asset listing failed: %w", err)
		}
		for _, asset := range assets {
			if asset.ID == 0 || asset.Name == "" || asset.BrowserDownloadURL == "" || asset.Size < 0 {
				return nil, fmt.Errorf("Forgejo asset listing returned malformed asset metadata")
			}
		}
		all = append(all, assets...)
		if len(assets) < 50 {
			return all, nil
		}
	}
}

// DownloadAsset opens a Forgejo asset stream. Authorization is retained only
// for redirects on the exact Forgejo API origin and stripped for other HTTPS
// origins, such as signed object-storage URLs.
func (c *Client) DownloadAsset(ctx context.Context, asset Asset) (io.ReadCloser, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Forgejo API URL: %w", err)
	}
	downloadURL, err := url.Parse(asset.BrowserDownloadURL)
	if err != nil || !sameOrigin(base, downloadURL) {
		return nil, fmt.Errorf("unsafe Forgejo download URL for asset %q", asset.Name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	clientCopy := *c.httpClient
	previousCheck := clientCopy.CheckRedirect
	clientCopy.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if next.URL.Scheme != base.Scheme {
			return fmt.Errorf("refusing non-%s Forgejo asset redirect", base.Scheme)
		}
		if sameOrigin(base, next.URL) {
			next.Header.Set("Authorization", "token "+c.token)
		} else {
			next.Header.Del("Authorization")
		}
		if previousCheck != nil {
			return previousCheck(next, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}
	resp, err := clientCopy.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download Forgejo asset %q: %w", asset.Name, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("download Forgejo asset %q: %w", asset.Name, c.responseError(resp))
	}
	return resp.Body, nil
}
