package sync

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"

	"pubcode.archuser.org/actions/gh-sync/internal/config"
	"pubcode.archuser.org/actions/gh-sync/internal/files"
	"pubcode.archuser.org/actions/gh-sync/internal/github"
)

type GitHub interface {
	GetReleaseByTag(context.Context, string, string, string) (*github.Release, bool, error)
	CreateRelease(context.Context, string, string, github.CreateReleaseRequest) (*github.Release, error)
	ListAssets(context.Context, string, string, int64) ([]github.Asset, error)
	DeleteAsset(context.Context, string, string, int64) error
	UploadAsset(context.Context, string, string, string, io.Reader) (*github.Asset, error)
}

func Run(ctx context.Context, cfg config.Config, client GitHub, logger *log.Logger) error {
	logger.Printf("target %s/%s", cfg.Owner, cfg.Repository)
	release, exists, err := client.GetReleaseByTag(ctx, cfg.Owner, cfg.Repository, cfg.Tag)
	if err != nil {
		return err
	}
	if exists {
		logger.Printf("release %s already exists", cfg.Tag)
	} else {
		release, err = client.CreateRelease(ctx, cfg.Owner, cfg.Repository, github.CreateReleaseRequest{
			TagName: cfg.Tag, Name: cfg.Name, Body: cfg.Body, Draft: cfg.Draft, Prerelease: cfg.Prerelease,
		})
		if err != nil {
			return err
		}
		logger.Printf("created release %s", cfg.Tag)
	}

	paths, err := files.Expand(cfg.Files)
	if err != nil {
		return err
	}
	logger.Printf("found %d local artifacts", len(paths))
	if len(paths) == 0 {
		logger.Printf("synchronization complete")
		return nil
	}
	localNames := make(map[string]string, len(paths))
	for _, path := range paths {
		name := filepath.Base(path)
		if previous, ok := localNames[name]; ok {
			return fmt.Errorf("local files %q and %q use the same asset filename %q", previous, path, name)
		}
		localNames[name] = path
	}

	assets, err := client.ListAssets(ctx, cfg.Owner, cfg.Repository, release.ID)
	if err != nil {
		return err
	}
	byName := make(map[string]github.Asset, len(assets))
	for _, asset := range assets {
		byName[asset.Name] = asset
	}
	for _, path := range paths {
		name := filepath.Base(path)
		if existing, ok := byName[name]; ok {
			if !cfg.Overwrite {
				return fmt.Errorf("asset %q already exists and overwrite is disabled", name)
			}
			logger.Printf("replacing %s", name)
			if err := client.DeleteAsset(ctx, cfg.Owner, cfg.Repository, existing.ID); err != nil {
				return fmt.Errorf("replace asset %q: %w", name, err)
			}
		}
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open asset %q: %w", path, err)
		}
		contentType := mime.TypeByExtension(filepath.Ext(path))
		_, uploadErr := client.UploadAsset(ctx, release.UploadURL, name, contentType, file)
		closeErr := file.Close()
		if uploadErr != nil {
			return fmt.Errorf("upload asset %q: %w", name, uploadErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close asset %q: %w", path, closeErr)
		}
		logger.Printf("uploaded %s", name)
	}
	logger.Printf("synchronization complete")
	return nil
}
