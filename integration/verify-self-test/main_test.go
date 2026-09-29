package main

import (
	"strings"
	"testing"
)

func TestValidateAssets(t *testing.T) {
	assets := []asset{
		{ID: 10, Name: primaryName},
		{ID: 11, Name: secondaryName},
		{ID: 12, Name: "unrelated.txt"},
	}
	id, err := validateAssets(assets)
	if err != nil || id != 10 {
		t.Fatalf("validateAssets() = %d, %v", id, err)
	}
}

func TestValidateAssetsRequiresExactlyOneOfEach(t *testing.T) {
	tests := []struct {
		name   string
		assets []asset
		want   string
	}{
		{"missing", []asset{{ID: 1, Name: primaryName}}, "was not found"},
		{"duplicate", []asset{{ID: 1, Name: primaryName}, {ID: 2, Name: primaryName}, {ID: 3, Name: secondaryName}}, "exactly one"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateAssets(test.assets)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want substring %q", err, test.want)
			}
		})
	}
}
