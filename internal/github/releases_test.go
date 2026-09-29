package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReleaseLifecycleRequests(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("X-GitHub-Api-Version") == "" {
			t.Errorf("required headers missing: %#v", r.Header)
		}
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/releases/tags/v1.0.0":
			json.NewEncoder(w).Encode(Release{ID: 7, TagName: "v1.0.0", UploadURL: serverURL(r) + "/upload{?name,label}"})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/releases":
			var request CreateReleaseRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.TagName != "v2" || !request.Prerelease {
				t.Errorf("create body = %#v", request)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(Release{ID: 8, TagName: "v2", UploadURL: serverURL(r) + "/upload{?name,label}"})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/releases/7/assets":
			json.NewEncoder(w).Encode([]Asset{{ID: 9, Name: "artifact.zip"}})
		case r.Method == http.MethodDelete && r.URL.Path == "/repos/owner/repo/releases/assets/9":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/upload":
			data, _ := io.ReadAll(r.Body)
			if r.URL.Query().Get("name") != "my artifact.zip" || string(data) != "payload" {
				t.Errorf("upload name/body = %q/%q", r.URL.Query().Get("name"), data)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(Asset{ID: 10, Name: "my artifact.zip"})
		default:
			http.Error(w, `{"message":"unexpected request"}`, http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL, server.Client())
	ctx := context.Background()
	release, found, err := client.GetReleaseByTag(ctx, "owner", "repo", "v1.0.0")
	if err != nil || !found || release.ID != 7 {
		t.Fatalf("lookup = %#v, %v, %v", release, found, err)
	}
	created, err := client.CreateRelease(ctx, "owner", "repo", CreateReleaseRequest{TagName: "v2", Name: "Two", Prerelease: true})
	if err != nil || created.ID != 8 {
		t.Fatalf("create = %#v, %v", created, err)
	}
	assets, err := client.ListAssets(ctx, "owner", "repo", 7)
	if err != nil || len(assets) != 1 || assets[0].ID != 9 {
		t.Fatalf("assets = %#v, %v", assets, err)
	}
	if err := client.DeleteAsset(ctx, "owner", "repo", 9); err != nil {
		t.Fatal(err)
	}
	asset, err := client.UploadAsset(ctx, release.UploadURL, "my artifact.zip", "application/zip", strings.NewReader("payload"))
	if err != nil || asset.ID != 10 {
		t.Fatalf("upload = %#v, %v", asset, err)
	}
	if len(requests) != 5 {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestGetReleaseNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"message":"Not Found"}`)
	}))
	defer server.Close()
	client := NewClient("token", server.URL, server.Client())
	release, found, err := client.GetReleaseByTag(context.Background(), "owner", "repo", "missing")
	if err != nil || found || release != nil {
		t.Fatalf("lookup = %#v, %v, %v", release, found, err)
	}
}

func TestAPIErrorIncludesStatusAndMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"message":"Bad credentials: super-secret"}`)
	}))
	defer server.Close()
	client := NewClient("super-secret", server.URL, server.Client())
	_, _, err := client.GetReleaseByTag(context.Background(), "owner", "repo", "v1")
	if err == nil || !strings.Contains(err.Error(), "HTTP 401: Bad credentials: ***") || strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("error = %v", err)
	}
}

func serverURL(r *http.Request) string { return "http://" + r.Host }
