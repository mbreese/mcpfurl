package fetchurl

import (
	"fmt"
	"regexp"
	"strings"
)

// ensureURLAllowed enforces allow/deny lists for outbound HTTP requests.
func ensureURLAllowed(target string, allowList, denyList []string) (bool, error) {
	// fmt.Printf("checking: %s\nallow: %s\ndeny: %s\n", target, allowList, denyList)

	target = strings.TrimSpace(target)
	if target == "" {
		return false, fmt.Errorf("missing URL")
	}

	// check allow first
	matched, err := matchGlobList(target, allowList)
	if err != nil {
		return false, err
	}
	if matched {
		return true, nil
	}

	if matched, err := matchGlobList(target, denyList); err != nil {
		return false, err
	} else if matched {
		return false, fmt.Errorf("URL %q is denied by policy", target)
	}

	// if there is no set allow list, then we assume it is "*"
	if len(allowList) == 0 {
		return true, nil
	}

	return false, fmt.Errorf("URL %q is denied by default", target)
}

func matchGlobList(target string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := globToRegex(pattern)
		if err != nil {
			return false, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if re.MatchString(target) {
			return true, nil
		}
	}
	return false, nil
}

func globToRegex(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	escaped := false
	for _, r := range pattern {
		if escaped {
			b.WriteString(regexp.QuoteMeta(string(r)))
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	if escaped {
		b.WriteString("\\\\")
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
