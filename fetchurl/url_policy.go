package fetchurl

import (
	"fmt"
	"path"
	"strings"
)

// ensureURLAllowed enforces allow/deny lists for outbound HTTP requests.
func ensureURLAllowed(target string, allowList, denyList []string) (bool, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return false, fmt.Errorf("missing URL")
	}

	if matched, err := matchGlobList(target, denyList); err != nil {
		return false, err
	} else if matched {
		return false, fmt.Errorf("URL %q is disallowed by policy", target)
	}

	if len(allowList) == 0 {
		return true, nil
	}

	matched, err := matchGlobList(target, allowList)
	if err != nil {
		return false, err
	}
	if !matched {
		return false, fmt.Errorf("URL %q is not in the allowed list", target)
	}
	return true, nil
}

func matchGlobList(target string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		match, err := path.Match(pattern, target)
		if err != nil {
			return false, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}
