// Package lib holds small utilities shared across reportd binaries.
package lib

import (
	"fmt"
	"regexp"
)

var validServiceName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateService rejects empty names, names over 32 characters, and
// names outside [A-Za-z0-9_-].
func ValidateService(name string) error {
	if name == "" {
		return fmt.Errorf("service must not be empty")
	}
	if len(name) > 32 {
		return fmt.Errorf("service must be less than 32 characters")
	}
	if !validServiceName.MatchString(name) {
		return fmt.Errorf("service %q must match %s", name, validServiceName.String())
	}
	return nil
}
