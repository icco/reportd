// Package lib holds small utilities shared across reportd binaries.
package lib

import (
	"fmt"
	"regexp"
)

// validServiceName is the allowed character set for a service identifier.
var validServiceName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateService returns an error if name is empty, longer than 32
// characters, or contains characters outside [A-Za-z0-9_-].
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
