package config

import (
	"fmt"
	"strings"
)

// Config contains the validated inputs needed for a synchronization.
type Config struct {
	Owner      string
	Repository string
	Token      string
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

	return Config{Owner: owner, Repository: repository, Token: raw.Token, Tag: tag,
		Name: name, Body: raw.Body, Files: raw.Files, Draft: draft,
		Prerelease: prerelease, Overwrite: overwrite}, nil
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
