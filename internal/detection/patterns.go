package detection

import (
	"path/filepath"
	"strings"
)

// matchesIgnorePattern checks if a type name matches any ignore pattern.
func matchesIgnorePattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
		if strings.Contains(name, pattern) {
			return true
		}
	}
	return false
}

// ParseIgnorePatterns splits a comma-separated pattern string into a slice.
func ParseIgnorePatterns(patterns string) []string {
	if patterns == "" {
		return nil
	}
	parts := strings.Split(patterns, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
