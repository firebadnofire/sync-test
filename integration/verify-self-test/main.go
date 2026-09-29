// Command verify-self-test independently verifies the fixed gh-sync integration
// release. Its target is intentionally not configurable, preventing this test
// helper from publishing to or inspecting an unrelated repository.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiBase         = "https://api.github.com"
	repository      = "firebadnofire/sync-test"
	releaseTag      = "gh-sync-self-test"
	primaryName     = "gh-sync-self-test.txt"
	secondaryName   = "gh-sync-self-test-secondary.txt"
	overwriteMarker = "phase=overwrite"
)

type verifier struct {
	token  string
	isFine bool
	client *http.Client
}

type release struct {
	ID      int64  `json:"id"`
	TagName string `json:"tag_name"`
}

type asset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type apiError struct {
	Message string `json:"message"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "self-test: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	token := strings.TrimSpace(os.Getenv("INPUT_TOKEN"))
	if token == "" {
		return fmt.Errorf("token input is required; configure the GH_KEY Forgejo secret")
	}
	isFine, err := parseTokenType(os.Getenv("INPUT_IS_FINE"))
	if err != nil {
		return err
	}
	v := verifier{token: token, isFine: isFine, client: &http.Client{Timeout: 2 * time.Minute}}
	ctx := context.Background()

	var foundRelease release
	if err := v.getJSON(ctx, apiBase+"/repos/"+repository+"/releases/tags/"+releaseTag, &foundRelease); err != nil {
		return fmt.Errorf("release lookup failed: %w", err)
	}
	if foundRelease.TagName != releaseTag {
		return fmt.Errorf("release lookup returned tag %q; expected %q", foundRelease.TagName, releaseTag)
	}

	var assets []asset
	for page := 1; ; page++ {
		assetsURL := fmt.Sprintf("%s/repos/%s/releases/%d/assets?per_page=100&page=%d", apiBase, repository, foundRelease.ID, page)
		var pageAssets []asset
		if err := v.getJSON(ctx, assetsURL, &pageAssets); err != nil {
			return fmt.Errorf("asset listing failed on page %d: %w", page, err)
		}
		assets = append(assets, pageAssets...)
		if len(pageAssets) < 100 {
			break
		}
	}
	primaryID, err := validateAssets(assets)
	if err != nil {
		return err
	}

	contents, err := v.download(ctx, fmt.Sprintf("%s/repos/%s/releases/assets/%d", apiBase, repository, primaryID))
	if err != nil {
		return fmt.Errorf("download %s failed: %w", primaryName, err)
	}
	if !strings.Contains(string(contents), overwriteMarker) {
		return fmt.Errorf("overwrite verification failed: downloaded %s does not contain %q", primaryName, overwriteMarker)
	}
	if strings.Contains(string(contents), "phase=initial") {
		return fmt.Errorf("overwrite verification failed: downloaded %s still contains phase=initial", primaryName)
	}

	fmt.Printf("self-test: verified release %s in %s\n", releaseTag, repository)
	fmt.Printf("self-test: verified exactly one %s and one %s\n", primaryName, secondaryName)
	fmt.Printf("self-test: verified downloaded %s contains %s\n", primaryName, overwriteMarker)
	return nil
}

func validateAssets(assets []asset) (int64, error) {
	counts := map[string]int{primaryName: 0, secondaryName: 0}
	var primaryID int64
	for _, candidate := range assets {
		if _, expected := counts[candidate.Name]; expected {
			counts[candidate.Name]++
			if candidate.Name == primaryName {
				primaryID = candidate.ID
			}
		}
	}
	for _, name := range []string{primaryName, secondaryName} {
		switch counts[name] {
		case 0:
			return 0, fmt.Errorf("expected asset %s was not found", name)
		case 1:
		default:
			return 0, fmt.Errorf("expected exactly one asset named %s; found %d", name, counts[name])
		}
	}
	return primaryID, nil
}

func parseTokenType(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("invalid is_fine value %q: expected true or false", value)
	}
}

func (v verifier) getJSON(ctx context.Context, endpoint string, destination any) error {
	contents, err := v.request(ctx, endpoint, "application/vnd.github+json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(contents, destination); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

func (v verifier) download(ctx context.Context, endpoint string) ([]byte, error) {
	return v.request(ctx, endpoint, "application/octet-stream")
}

func (v verifier) request(ctx context.Context, endpoint, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	authScheme := "token"
	if v.isFine {
		authScheme = "Bearer"
	}
	req.Header.Set("Authorization", authScheme+" "+v.token)
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "actions/gh-sync-self-test")
	response, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	contents, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if readErr != nil {
		return nil, fmt.Errorf("read response: %w", readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(contents))
		var githubError apiError
		if json.Unmarshal(contents, &githubError) == nil && githubError.Message != "" {
			message = githubError.Message
		}
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		message = strings.ReplaceAll(message, v.token, "***")
		return nil, fmt.Errorf("GitHub API returned HTTP %d: %s", response.StatusCode, message)
	}
	return contents, nil
}
