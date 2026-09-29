package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pubcode.archuser.org/actions/gh-sync/internal/config"
	"pubcode.archuser.org/actions/gh-sync/internal/github"
)

type fakeGitHub struct {
	release       *github.Release
	found         bool
	assets        []github.Asset
	created       *github.CreateReleaseRequest
	deleted       []int64
	uploadedNames []string
	uploadedData  []string
}

func (f *fakeGitHub) GetReleaseByTag(context.Context, string, string, string) (*github.Release, bool, error) {
	return f.release, f.found, nil
}
func (f *fakeGitHub) CreateRelease(_ context.Context, _, _ string, request github.CreateReleaseRequest) (*github.Release, error) {
	f.created = &request
	return &github.Release{ID: 42, UploadURL: "https://uploads.example.test/release{?name,label}"}, nil
}
func (f *fakeGitHub) ListAssets(context.Context, string, string, int64) ([]github.Asset, error) {
	return f.assets, nil
}
func (f *fakeGitHub) DeleteAsset(_ context.Context, _, _ string, id int64) error {
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeGitHub) UploadAsset(_ context.Context, _ string, name, _ string, size int64, body io.Reader) (*github.Asset, error) {
	data, _ := io.ReadAll(body)
	if int64(len(data)) != size {
		return nil, fmt.Errorf("size %d does not match body length %d", size, len(data))
	}
	f.uploadedNames = append(f.uploadedNames, name)
	f.uploadedData = append(f.uploadedData, string(data))
	return &github.Asset{Name: name}, nil
}

func TestRunCreatesReleaseAndUploads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.zip")
	if err := os.WriteFile(path, []byte("contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &fakeGitHub{}
	cfg := config.Config{Owner: "owner", Repository: "repo", Tag: "v1", Name: "One", Body: "notes", Files: path, Prerelease: true, Overwrite: true}
	var output bytes.Buffer
	if err := Run(context.Background(), cfg, fake, log.New(&output, "gh-sync: ", 0)); err != nil {
		t.Fatal(err)
	}
	if fake.created == nil || fake.created.TagName != "v1" || !fake.created.Prerelease {
		t.Fatalf("create = %#v", fake.created)
	}
	if len(fake.uploadedNames) != 1 || fake.uploadedNames[0] != "artifact.zip" || fake.uploadedData[0] != "contents" {
		t.Fatalf("uploads = %#v %#v", fake.uploadedNames, fake.uploadedData)
	}
	if !strings.Contains(output.String(), "synchronization complete") {
		t.Fatalf("log = %q", output.String())
	}
}

func TestRunOverwriteBehavior(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.zip")
	if err := os.WriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := &fakeGitHub{release: &github.Release{ID: 1, UploadURL: "https://upload"}, found: true, assets: []github.Asset{{ID: 9, Name: "artifact.zip"}}}
	cfg := config.Config{Owner: "owner", Repository: "repo", Tag: "v1", Name: "v1", Files: path}
	err := Run(context.Background(), cfg, base, log.New(io.Discard, "", 0))
	if err == nil || !strings.Contains(err.Error(), "overwrite is disabled") || len(base.deleted) != 0 || len(base.uploadedNames) != 0 {
		t.Fatalf("disabled: error=%v fake=%#v", err, base)
	}

	overwrite := &fakeGitHub{release: base.release, found: true, assets: base.assets}
	cfg.Overwrite = true
	if err := Run(context.Background(), cfg, overwrite, log.New(io.Discard, "", 0)); err != nil {
		t.Fatal(err)
	}
	if len(overwrite.deleted) != 1 || overwrite.deleted[0] != 9 || len(overwrite.uploadedNames) != 1 {
		t.Fatalf("overwrite fake = %#v", overwrite)
	}
}

func TestRunRejectsDuplicateAssetNames(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first", "artifact.zip")
	second := filepath.Join(dir, "second", "artifact.zip")
	for _, path := range []string{first, second} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fake := &fakeGitHub{release: &github.Release{ID: 1}, found: true}
	cfg := config.Config{Owner: "owner", Repository: "repo", Tag: "v1", Files: first + "\n" + second, Overwrite: true}
	err := Run(context.Background(), cfg, fake, log.New(io.Discard, "", 0))
	if err == nil || !strings.Contains(err.Error(), "same asset filename") || len(fake.uploadedNames) != 0 {
		t.Fatalf("error = %v, uploads = %#v", err, fake.uploadedNames)
	}
}
