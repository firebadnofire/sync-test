package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"pubcode.archuser.org/actions/gh-sync/internal/config"
	"pubcode.archuser.org/actions/gh-sync/internal/github"
	synchronizer "pubcode.archuser.org/actions/gh-sync/internal/sync"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gh-sync: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	repository := flag.String("repository", env("INPUT_REPOSITORY", ""), "GitHub repository (owner/repository)")
	token := flag.String("token", env("INPUT_TOKEN", ""), "GitHub token (prefer INPUT_TOKEN)")
	tag := flag.String("tag", env("INPUT_TAG", ""), "release tag")
	name := flag.String("name", env("INPUT_NAME", ""), "release name")
	body := flag.String("body", env("INPUT_BODY", ""), "release body")
	filePatterns := flag.String("files", env("INPUT_FILES", ""), "newline-separated files or glob patterns")
	draft := flag.String("draft", env("INPUT_DRAFT", "false"), "whether the release is a draft")
	prerelease := flag.String("prerelease", env("INPUT_PRERELEASE", "false"), "whether the release is a prerelease")
	overwrite := flag.String("overwrite", env("INPUT_OVERWRITE", "true"), "whether existing assets are replaced")
	apiURL := flag.String("api-url", env("GH_SYNC_API_URL", "https://api.github.com"), "GitHub API base URL")
	flag.Parse()

	cfg, err := config.Parse(config.Raw{
		Repository: *repository, Token: *token, Tag: *tag, Name: *name, Body: *body,
		Files: *filePatterns, Draft: *draft, Prerelease: *prerelease, Overwrite: *overwrite,
		ForgejoRef: os.Getenv("FORGEJO_REF_NAME"), GitHubRef: os.Getenv("GITHUB_REF_NAME"),
	})
	if err != nil {
		return err
	}
	logger := log.New(os.Stdout, "gh-sync: ", 0)
	client := github.NewClient(cfg.Token, *apiURL, nil)
	return synchronizer.Run(context.Background(), cfg, client, logger)
}

func env(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
