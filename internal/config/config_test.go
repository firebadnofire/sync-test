package config

import "testing"

func TestParseRepository(t *testing.T) {
	owner, repository, err := ParseRepository("firebadnofire/example")
	if err != nil || owner != "firebadnofire" || repository != "example" {
		t.Fatalf("got %q, %q, %v", owner, repository, err)
	}
	for _, value := range []string{"foo", "/repo", "owner/", "a/b/c", "owner/..", "bad owner/repo", "owner/repo?x"} {
		if _, _, err := ParseRepository(value); err == nil {
			t.Errorf("ParseRepository(%q) unexpectedly succeeded", value)
		}
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		value        string
		defaultValue bool
		want         bool
		wantErr      bool
	}{
		{"", true, true, false},
		{"TRUE", false, true, false},
		{" false ", true, false, false},
		{"yes", false, false, true},
	}
	for _, test := range tests {
		got, err := ParseBool("test", test.value, test.defaultValue)
		if got != test.want || (err != nil) != test.wantErr {
			t.Errorf("ParseBool(%q) = %v, %v; want %v, error=%v", test.value, got, err, test.want, test.wantErr)
		}
	}
}

func TestParseTagAndNameDefaults(t *testing.T) {
	base := Raw{Repository: "owner/repo", Token: "secret"}
	tests := []struct {
		name      string
		raw       Raw
		wantTag   string
		wantName  string
		wantError bool
	}{
		{"explicit", merge(base, Raw{Tag: "v3", Name: "Release 3"}), "v3", "Release 3", false},
		{"forgejo ref", merge(base, Raw{ForgejoRef: "v2", GitHubRef: "v1"}), "v2", "v2", false},
		{"github ref", merge(base, Raw{GitHubRef: "v1"}), "v1", "v1", false},
		{"missing", base, "", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Parse(test.raw)
			if (err != nil) != test.wantError {
				t.Fatalf("Parse() error = %v", err)
			}
			if !test.wantError && (got.Tag != test.wantTag || got.Name != test.wantName) {
				t.Fatalf("tag/name = %q/%q, want %q/%q", got.Tag, got.Name, test.wantTag, test.wantName)
			}
		})
	}
}

func TestParseTokenType(t *testing.T) {
	base := Raw{Repository: "owner/repo", Token: "secret", Tag: "v1"}
	classic, err := Parse(base)
	if err != nil || classic.IsFine {
		t.Fatalf("classic config = %#v, %v", classic, err)
	}
	base.IsFine = "true"
	fine, err := Parse(base)
	if err != nil || !fine.IsFine {
		t.Fatalf("fine config = %#v, %v", fine, err)
	}
	base.IsFine = "classic"
	if _, err := Parse(base); err == nil {
		t.Fatal("invalid is_fine unexpectedly succeeded")
	}
}

func merge(a, b Raw) Raw {
	if b.Tag != "" {
		a.Tag = b.Tag
	}
	if b.Name != "" {
		a.Name = b.Name
	}
	if b.ForgejoRef != "" {
		a.ForgejoRef = b.ForgejoRef
	}
	if b.GitHubRef != "" {
		a.GitHubRef = b.GitHubRef
	}
	return a
}
