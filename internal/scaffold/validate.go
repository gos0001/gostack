package scaffold

import (
	"fmt"
	"os"
	"regexp"
)

var validName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// ValidateName checks that a project or feature name is well-formed.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if !validName.MatchString(name) {
		return fmt.Errorf("name %q must start with a lowercase letter and contain only [a-z0-9_-]", name)
	}
	return nil
}

// ValidateFeaturePath validates an optionally grouped feature name such as
// "users/get_profile". Every segment must be a valid identifier stem.
func ValidateFeaturePath(p string) error {
	segments := splitPath(p)
	if len(segments) == 0 {
		return fmt.Errorf("name cannot be empty")
	}
	for _, s := range segments {
		if err := ValidateName(s); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePagePath checks that a page path like "home/[user_id]" is valid.
func ValidatePagePath(path string) error {
	if path == "" {
		return fmt.Errorf("page path cannot be empty")
	}
	paramRe := regexp.MustCompile(`^\[([a-z][a-z0-9_]*)\]$`)
	segRe := regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

	segments := splitPath(path)
	for _, seg := range segments {
		if paramRe.MatchString(seg) {
			continue
		}
		if segRe.MatchString(seg) {
			continue
		}
		return fmt.Errorf("invalid path segment %q in %q", seg, path)
	}
	return nil
}

// DirMustNotExist returns an error if path already exists.
func DirMustNotExist(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("directory %q already exists", path)
	}
	return nil
}

func splitPath(path string) []string {
	var parts []string
	for _, p := range regexp.MustCompile(`[/\\]`).Split(path, -1) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}
