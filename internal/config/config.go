package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Config contains the validated inputs needed for a synchronization.
type Config struct {
	Owner      string
	Repository string
	Token      string
	IsFine     bool
	Mirror     bool
	SourceToken      string
	SourceAPIURL     string
	SourceOwner      string
	SourceRepository string
	Tag        string
	Name       string
	Body       string
	Files      string
	Draft      bool
	Prerelease bool
	Overwrite  bool
}

// Raw contains string values supplied by flags or action input variables.
type Raw struct {
	Repository string
	Token      string
	IsFine     string
	SourceToken      string
	SourceAPIURL     string
	SourceRepository string
	Tag        string
	Name       string
	Body       string
	Files      string
	Draft      string
	Prerelease string
	Overwrite  string
	ForgejoRef string
	GitHubRef  string
}

func Parse(raw Raw) (Config, error) {
	owner, repository, err := ParseRepository(raw.Repository)
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(raw.Token) == "" {
		return Config{}, fmt.Errorf("token is required")
	}
	isFine, err := ParseBool("is_fine", raw.IsFine, false)
	if err != nil {
		return Config{}, err
	}

	tag := strings.TrimSpace(raw.Tag)
	if tag == "" {
		tag = strings.TrimSpace(raw.ForgejoRef)
	}
	if tag == "" {
		tag = strings.TrimSpace(raw.GitHubRef)
	}
	if tag == "" {
		return Config{}, fmt.Errorf("no release tag specified and no ref name is available")
	}
	name := raw.Name
	if strings.TrimSpace(name) == "" {
		name = tag
	}

	draft, err := ParseBool("draft", raw.Draft, false)
	if err != nil {
		return Config{}, err
	}
	prerelease, err := ParseBool("prerelease", raw.Prerelease, false)
	if err != nil {
		return Config{}, err
	}
	overwrite, err := ParseBool("overwrite", raw.Overwrite, true)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{Owner: owner, Repository: repository, Token: raw.Token, IsFine: isFine, Tag: tag,
		Name: name, Body: raw.Body, Files: raw.Files, Draft: draft,
		Prerelease: prerelease, Overwrite: overwrite}

	if strings.TrimSpace(raw.SourceToken) == "" {
		return cfg, nil
	}
	sourceOwner, sourceRepository, err := ParseRepository(raw.SourceRepository)
	if err != nil {
		return Config{}, fmt.Errorf("invalid source repository: %w", err)
	}
	sourceAPIURL := strings.TrimRight(strings.TrimSpace(raw.SourceAPIURL), "/")
	parsedURL, err := url.Parse(sourceAPIURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return Config{}, fmt.Errorf("invalid source API URL %q: expected an HTTPS URL", sourceAPIURL)
	}
	if strings.TrimSpace(raw.Name) != "" || raw.Body != "" || strings.TrimSpace(raw.Files) != "" || draft || prerelease || !overwrite {
		return Config{}, fmt.Errorf("name, body, files, draft, prerelease, and overwrite=false are legacy-mode inputs and cannot be used with source_token")
	}
	cfg.Mirror = true
	cfg.SourceToken = raw.SourceToken
	cfg.SourceAPIURL = sourceAPIURL
	cfg.SourceOwner = sourceOwner
	cfg.SourceRepository = sourceRepository
	return cfg, nil
}

func ParseRepository(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" ||
		strings.TrimSpace(parts[0]) != parts[0] || strings.TrimSpace(parts[1]) != parts[1] ||
		strings.ContainsAny(parts[0], " \\?#\t\r\n") || strings.ContainsAny(parts[1], " \\?#\t\r\n") ||
		parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." {
		return "", "", fmt.Errorf("invalid repository slug %q: expected owner/repository", value)
	}
	return parts[0], parts[1], nil
}

func ParseBool(name, value string, defaultValue bool) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	switch strings.ToLower(value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s value %q: expected true or false", name, value)
	}
}
