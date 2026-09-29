package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Expand resolves newline-separated paths and glob patterns into unique files.
func Expand(input string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(input, "\n") {
		pattern := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if pattern == "" {
			continue
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("pattern %q matched no files", pattern)
		}
		matchedFile := false
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				return nil, fmt.Errorf("inspect %q: %w", match, err)
			}
			if !info.Mode().IsRegular() {
				continue
			}
			matchedFile = true
			clean := filepath.Clean(match)
			absolute, err := filepath.Abs(clean)
			if err != nil {
				return nil, fmt.Errorf("resolve %q: %w", match, err)
			}
			if !seen[absolute] {
				seen[absolute] = true
				result = append(result, clean)
			}
		}
		if !matchedFile {
			return nil, fmt.Errorf("pattern %q matched no files", pattern)
		}
	}
	return result, nil
}
